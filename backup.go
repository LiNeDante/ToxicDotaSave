package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	backupFormat    = 1
	manifestName    = "manifest.json"
	filesPrefix     = "570/"
	maxBackupFiles  = 4000
	maxBackupBytes  = 64 << 20
	maxImportBytes  = 32 << 20
	keepAutoBackups = 10
)

// Steam's per-app cloud bookkeeping. It must stay with the account it
// belongs to: carrying it across would make Steam misjudge which side of the
// cloud sync is newer.
var excludedFiles = map[string]bool{"remotecache.vdf": true}

var ErrDotaRunning = errors.New("dota_running")

type Manifest struct {
	Format      int       `json:"format"`
	App         string    `json:"app"`
	AppVersion  string    `json:"appVersion"`
	CreatedAt   time.Time `json:"createdAt"`
	AccountID   string    `json:"accountId"`
	SteamID64   string    `json:"steamId64"`
	PersonaName string    `json:"personaName"`
	Label       string    `json:"label"`
	Auto        bool      `json:"auto"`
	FileCount   int       `json:"fileCount"`
	TotalBytes  int64     `json:"totalBytes"`
}

type BackupInfo struct {
	ID string `json:"id"` // zip file name inside the backups folder
	Manifest
	SizeBytes int64  `json:"sizeBytes"`
	Hash      string `json:"hash"` // identity of the settings content, see contentHash
}

// DuplicateError is returned when the exact same settings are already backed
// up, so the same profile can't pile up hundreds of identical copies.
type DuplicateError struct{ Existing BackupInfo }

func (e *DuplicateError) Error() string { return "duplicate" }

type hashItem struct {
	rel  string
	crc  uint32
	size uint64
}

// contentHash identifies a set of settings files by path, CRC-32 and size.
// Zip archives store exactly these per entry, so backups can be compared
// without decompressing them; labels, dates and owners don't affect it.
func contentHash(items []hashItem) string {
	sort.Slice(items, func(i, j int) bool { return items[i].rel < items[j].rel })
	h := sha256.New()
	for _, it := range items {
		fmt.Fprintf(h, "%s\x00%08x\x00%d\n", it.rel, it.crc, it.size)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func zipContentHash(files []*zip.File) string {
	items := make([]hashItem, len(files))
	for i, f := range files {
		items[i] = hashItem{strings.TrimPrefix(f.Name, filesPrefix), f.CRC32, f.UncompressedSize64}
	}
	return contentHash(items)
}

// findDuplicate returns an existing backup with the given content. Automatic
// backups only count when includeAuto is set: they get pruned, so they can't
// stand in for a backup the user asked for.
func findDuplicate(dir, hash string, includeAuto bool) *BackupInfo {
	for _, b := range listBackups(dir) {
		if b.Hash == hash && (includeAuto || !b.Auto) {
			return &b
		}
	}
	return nil
}

var unsafeNameChars = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

func safeFilePart(s string) string {
	s = strings.Trim(unsafeNameChars.ReplaceAllString(s, "_"), "_")
	if r := []rune(s); len(r) > 32 {
		s = string(r[:32])
	}
	if s == "" {
		s = "backup"
	}
	return s
}

func backupPath(dir, id string) (string, error) {
	if id == "" || id != filepath.Base(id) || strings.ContainsAny(id, `/\`) || !strings.HasSuffix(strings.ToLower(id), ".zip") {
		return "", errors.New("invalid backup id")
	}
	return filepath.Join(dir, id), nil
}

func uniqueBackupName(dir, persona string, t time.Time, auto bool) string {
	base := safeFilePart(persona) + "_" + t.Format("2006-01-02_15-04-05")
	if auto {
		base = "auto_" + base
	}
	name := base + ".zip"
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, fs.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("%s_%d.zip", base, i)
	}
}

// createBackup zips userdata/<account>/570 into dir. If a manual backup with
// identical content already exists it returns a *DuplicateError instead.
func createBackup(steamDir, dir string, acct Account, label string, auto bool) (BackupInfo, error) {
	src := dotaDir(steamDir, acct.AccountID)
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return BackupInfo{}, errors.New("no_dota_settings")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupInfo{}, err
	}

	type entry struct {
		rel  string
		mod  time.Time
		data []byte
	}
	var files []entry
	var total int64
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil // dirs, symlinks, devices
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if excludedFiles[strings.ToLower(rel)] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if len(files) >= maxBackupFiles || total > maxBackupBytes {
			return errors.New("settings folder is unexpectedly large")
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, entry{rel, info.ModTime(), data})
		return nil
	})
	if err != nil {
		return BackupInfo{}, err
	}
	if len(files) == 0 {
		return BackupInfo{}, errors.New("no_dota_settings")
	}

	items := make([]hashItem, len(files))
	for i, f := range files {
		items[i] = hashItem{f.rel, crc32.ChecksumIEEE(f.data), uint64(len(f.data))}
	}
	hash := contentHash(items)
	if dup := findDuplicate(dir, hash, false); dup != nil {
		return BackupInfo{}, &DuplicateError{*dup}
	}

	now := time.Now()
	m := Manifest{
		Format: backupFormat, App: appName, AppVersion: appVersion, CreatedAt: now,
		AccountID: acct.AccountID, SteamID64: acct.SteamID64, PersonaName: acct.PersonaName,
		Label: strings.TrimSpace(label), Auto: auto, FileCount: len(files), TotalBytes: total,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mj, _ := json.MarshalIndent(m, "", "  ")
	if w, err := zw.Create(manifestName); err != nil {
		return BackupInfo{}, err
	} else if _, err := w.Write(mj); err != nil {
		return BackupInfo{}, err
	}
	for _, f := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: filesPrefix + f.rel, Method: zip.Deflate, Modified: f.mod})
		if err != nil {
			return BackupInfo{}, err
		}
		if _, err := w.Write(f.data); err != nil {
			return BackupInfo{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return BackupInfo{}, err
	}

	name := uniqueBackupName(dir, acct.PersonaName, now, auto)
	if err := writeFileAtomic(filepath.Join(dir, name), buf.Bytes()); err != nil {
		return BackupInfo{}, err
	}
	if auto {
		pruneAutoBackups(dir)
	}
	return BackupInfo{ID: name, Manifest: m, SizeBytes: int64(buf.Len()), Hash: hash}, nil
}

// openBackup validates the archive and returns its manifest and the file
// entries under 570/. Every path is checked so an archive from the internet
// cannot write outside the target folder.
func openBackup(r *zip.Reader) (Manifest, []*zip.File, error) {
	var m Manifest
	var files []*zip.File
	var total uint64
	found := false
	for _, f := range r.File {
		if f.Name == manifestName {
			if f.UncompressedSize64 > 1<<20 {
				return m, nil, errors.New("invalid_backup")
			}
			rc, err := f.Open()
			if err != nil {
				return m, nil, errors.New("invalid_backup")
			}
			err = json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&m)
			rc.Close()
			if err != nil {
				return m, nil, errors.New("invalid_backup")
			}
			found = true
			continue
		}
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if !strings.HasPrefix(f.Name, filesPrefix) {
			return m, nil, errors.New("invalid_backup")
		}
		rel := strings.TrimPrefix(f.Name, filesPrefix)
		if rel == "" || strings.Contains(rel, `\`) || strings.Contains(rel, ":") || path.IsAbs(rel) ||
			path.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") || !f.Mode().IsRegular() {
			return m, nil, errors.New("invalid_backup")
		}
		if excludedFiles[strings.ToLower(rel)] {
			continue
		}
		total += f.UncompressedSize64
		files = append(files, f)
	}
	if !found || m.Format < 1 || m.Format > backupFormat {
		return m, nil, errors.New("invalid_backup")
	}
	if len(files) == 0 || len(files) > maxBackupFiles || total > maxBackupBytes {
		return m, nil, errors.New("invalid_backup")
	}
	return m, files, nil
}

func readBackupInfo(dir, id string) (BackupInfo, error) {
	p, err := backupPath(dir, id)
	if err != nil {
		return BackupInfo{}, err
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		return BackupInfo{}, errors.New("invalid_backup")
	}
	defer zr.Close()
	m, files, err := openBackup(&zr.Reader)
	if err != nil {
		return BackupInfo{}, err
	}
	st, _ := os.Stat(p)
	return BackupInfo{ID: id, Manifest: m, SizeBytes: st.Size(), Hash: zipContentHash(files)}, nil
}

func listBackups(dir string) []BackupInfo {
	entries, _ := os.ReadDir(dir)
	out := []BackupInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		if b, err := readBackupInfo(dir, e.Name()); err == nil {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// restoreBackup overwrites the files contained in the backup inside the
// target account's 570 folder. Files not in the backup are left alone so
// Steam Cloud does not treat them as deleted.
func restoreBackup(steamDir, dir, id, targetAccountID string) error {
	p, err := backupPath(dir, id)
	if err != nil {
		return err
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		return errors.New("invalid_backup")
	}
	defer zr.Close()
	_, files, err := openBackup(&zr.Reader)
	if err != nil {
		return err
	}

	dst := dotaDir(steamDir, targetAccountID)
	now := time.Now()
	for _, f := range files {
		rel := strings.TrimPrefix(f.Name, filesPrefix)
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if !strings.HasPrefix(target, dst+string(os.PathSeparator)) {
			return errors.New("invalid_backup")
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxBackupBytes+1))
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeFileAtomic(target, data); err != nil {
			return err
		}
		// Fresh mtime so Steam sees the local copy as the newest one.
		_ = os.Chtimes(target, now, now)
	}
	return nil
}

// importBackupData validates an archive picked by the user and copies it
// into the backups folder.
func importBackupData(dir string, data []byte) (BackupInfo, error) {
	if len(data) == 0 || len(data) > maxImportBytes {
		return BackupInfo{}, errors.New("invalid_backup")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return BackupInfo{}, errors.New("invalid_backup")
	}
	m, files, err := openBackup(zr)
	if err != nil {
		return BackupInfo{}, err
	}
	hash := zipContentHash(files)
	if dup := findDuplicate(dir, hash, true); dup != nil {
		return BackupInfo{}, &DuplicateError{*dup}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupInfo{}, err
	}
	created := m.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	name := uniqueBackupName(dir, m.PersonaName, created, false)
	if err := writeFileAtomic(filepath.Join(dir, name), data); err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{ID: name, Manifest: m, SizeBytes: int64(len(data)), Hash: hash}, nil
}

func deleteBackup(dir, id string) error {
	p, err := backupPath(dir, id)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func pruneAutoBackups(dir string) {
	var autos []BackupInfo
	for _, b := range listBackups(dir) {
		if b.Auto {
			autos = append(autos, b)
		}
	}
	for i := keepAutoBackups; i < len(autos); i++ {
		_ = deleteBackup(dir, autos[i].ID)
	}
}

func writeFileAtomic(p string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
