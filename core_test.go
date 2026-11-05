package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const loginUsers = `"users"
{
	"76561198000000001"
	{
		"AccountName"		"main_acc"
		"PersonaName"		"Can \"The\" Carry"
		"MostRecent"		"1"
		"Timestamp"		"1700000000"
	}
	"76561198000000002"
	{
		"AccountName"		"smurf"
		"PersonaName"		"pos5"
		"MostRecent"		"0"
		"Timestamp"		"1600000000"
	}
}
`

func TestParseVDF(t *testing.T) {
	root, err := parseVDF("\xEF\xBB\xBF// c\n" + loginUsers)
	if err != nil {
		t.Fatal(err)
	}
	u := root.Child("USERS").Child("76561198000000001")
	if got := u.Get("personaname"); got != `Can "The" Carry` {
		t.Fatalf("persona = %q", got)
	}
	if keys := root.Child("users").ChildKeys(); len(keys) != 2 || keys[0] != "76561198000000001" {
		t.Fatalf("keys = %v", keys)
	}
	if _, err := parseVDF(`"a" { "b" "c"`); err == nil {
		t.Fatal("expected error for unterminated object")
	}
}

func TestSteamIDConversion(t *testing.T) {
	if got := steamID64ToAccountID("76561198000000001"); got != "39734273" {
		t.Fatal(got)
	}
	if got := accountIDToSteamID64("39734273"); got != "76561198000000001" {
		t.Fatal(got)
	}
	if steamID64ToAccountID("123") != "" {
		t.Fatal("small id must be rejected")
	}
}

func fakeSteam(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(d, "config"), 0o755))
	must(t, os.WriteFile(filepath.Join(d, "config", "loginusers.vdf"), []byte(loginUsers), 0o644))
	cfg := filepath.Join(d, "userdata", "39734273", "570", "remote", "cfg")
	must(t, os.MkdirAll(cfg, 0o755))
	must(t, os.WriteFile(filepath.Join(cfg, "dotakeys_personal.lst"), []byte("main keys"), 0o644))
	must(t, os.WriteFile(filepath.Join(d, "userdata", "39734273", "570", "remotecache.vdf"), []byte("main cache"), 0o644))
	// Account only on disk, with persona in localconfig.
	must(t, os.MkdirAll(filepath.Join(d, "userdata", "555", "config"), 0o755))
	must(t, os.WriteFile(filepath.Join(d, "userdata", "555", "config", "localconfig.vdf"),
		[]byte(`"UserLocalConfigStore" { "friends" { "PersonaName" "disk only" } }`), 0o644))
	must(t, os.MkdirAll(filepath.Join(d, "userdata", "0"), 0o755))
	return d
}

func TestListAccounts(t *testing.T) {
	d := fakeSteam(t)
	accs := listAccounts(d, "39734274")
	if len(accs) != 3 {
		t.Fatalf("got %d accounts: %+v", len(accs), accs)
	}
	if accs[0].AccountID != "39734274" || !accs[0].Active {
		t.Fatalf("active account must sort first: %+v", accs[0])
	}
	if accs[1].AccountID != "39734273" || !accs[1].HasDota || !accs[1].MostRecent {
		t.Fatalf("main: %+v", accs[1])
	}
	if accs[2].PersonaName != "disk only" {
		t.Fatalf("disk-only persona: %+v", accs[2])
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	steam := fakeSteam(t)
	dir := t.TempDir()
	accs := listAccounts(steam, "")
	main, _ := findAccount(accs, "39734273")
	smurf, _ := findAccount(accs, "39734274")

	b, err := createBackup(steam, dir, main, "  my keys ", false)
	if err != nil {
		t.Fatal(err)
	}
	if b.Label != "my keys" || b.FileCount != 1 {
		t.Fatalf("manifest: %+v", b.Manifest)
	}

	// Restore onto an account that has never run Dota on this PC.
	must(t, restoreBackup(steam, dir, b.ID, smurf.AccountID))
	got, err := os.ReadFile(filepath.Join(dotaDir(steam, smurf.AccountID), "remote", "cfg", "dotakeys_personal.lst"))
	if err != nil || string(got) != "main keys" {
		t.Fatalf("restored = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dotaDir(steam, smurf.AccountID), "remotecache.vdf")); err == nil {
		t.Fatal("remotecache.vdf must not be carried across accounts")
	}

	list := listBackups(dir)
	if len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("list = %+v", list)
	}

	// Import the same archive into a fresh folder, then reject duplicates.
	raw, _ := os.ReadFile(filepath.Join(dir, b.ID))
	other := t.TempDir()
	if _, err := importBackupData(other, raw); err != nil {
		t.Fatal(err)
	}
	var dup *DuplicateError
	if _, err := importBackupData(other, raw); !errors.As(err, &dup) {
		t.Fatalf("duplicate import: %v", err)
	}

	must(t, deleteBackup(dir, b.ID))
	if len(listBackups(dir)) != 0 {
		t.Fatal("delete failed")
	}
}

func TestDuplicateBackups(t *testing.T) {
	steam := fakeSteam(t)
	dir := t.TempDir()
	accs := listAccounts(steam, "")
	main, _ := findAccount(accs, "39734273")
	smurf, _ := findAccount(accs, "39734274")

	first, err := createBackup(steam, dir, main, "one", false)
	must(t, err)

	// Same content, different note: still a duplicate.
	var dup *DuplicateError
	if _, err := createBackup(steam, dir, main, "two", false); !errors.As(err, &dup) || dup.Existing.ID != first.ID {
		t.Fatalf("expected duplicate of %s, got %v", first.ID, err)
	}

	// Same content on another account (after a restore) is a duplicate too.
	must(t, restoreBackup(steam, dir, first.ID, smurf.AccountID))
	smurf, _ = findAccount(listAccounts(steam, ""), smurf.AccountID)
	if _, err := createBackup(steam, dir, smurf, "", false); !errors.As(err, &dup) {
		t.Fatalf("cross-account duplicate not detected: %v", err)
	}

	// Automatic backups never block manual ones…
	_, err = createBackup(steam, dir, main, "", true)
	if !errors.As(err, &dup) {
		t.Fatalf("auto backup of manually saved content should be a duplicate: %v", err)
	}

	// …and changed settings are a new backup.
	cfg := filepath.Join(dotaDir(steam, main.AccountID), "remote", "cfg", "dotakeys_personal.lst")
	must(t, os.WriteFile(cfg, []byte("new keys"), 0o644))
	second, err := createBackup(steam, dir, main, "", false)
	must(t, err)
	if second.Hash == first.Hash {
		t.Fatal("hash must change with content")
	}

	// Hash read back from disk matches the one computed from the folder.
	info, err := readBackupInfo(dir, second.ID)
	must(t, err)
	if info.Hash != second.Hash {
		t.Fatalf("zip hash %s != folder hash %s", info.Hash, second.Hash)
	}
}

func TestAutoBackupPruning(t *testing.T) {
	steam := fakeSteam(t)
	dir := t.TempDir()
	main, _ := findAccount(listAccounts(steam, ""), "39734273")
	for i := 0; i < keepAutoBackups+3; i++ {
		if _, err := createBackup(steam, dir, main, "", true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := createBackup(steam, dir, main, "manual", false); err != nil {
		t.Fatal(err)
	}
	var autos, manual int
	for _, b := range listBackups(dir) {
		if b.Auto {
			autos++
		} else {
			manual++
		}
	}
	if autos != keepAutoBackups || manual != 1 {
		t.Fatalf("autos=%d manual=%d", autos, manual)
	}
}

func evilZip(t *testing.T, name string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(manifestName)
	w.Write([]byte(`{"format":1,"personaName":"x"}`))
	w, _ = zw.Create(name)
	w.Write([]byte("pwned"))
	must(t, zw.Close())
	return buf.Bytes()
}

func TestRejectsMaliciousArchives(t *testing.T) {
	for _, name := range []string{
		"570/../../evil.txt", "570/../x", "../evil", "570/a/../../b", `570/..\..\evil`,
		"570/C:/evil", "/etc/passwd", "other/file", "570//abs",
	} {
		if _, err := importBackupData(t.TempDir(), evilZip(t, name)); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if _, err := importBackupData(t.TempDir(), []byte("not a zip")); err == nil {
		t.Error("accepted garbage")
	}
	if _, err := importBackupData(t.TempDir(), evilZip(t, "570/remote/cfg/ok.lst")); err != nil {
		t.Errorf("rejected a valid archive: %v", err)
	}
}

func TestBackupIDValidation(t *testing.T) {
	for _, id := range []string{"", "../x.zip", `..\x.zip`, "a/b.zip", "x.txt"} {
		if _, err := backupPath("/tmp", id); err == nil {
			t.Errorf("accepted id %q", id)
		}
	}
	if !strings.HasSuffix(uniqueBackupName(t.TempDir(), "Can <3 / ?", time.Now(), false), ".zip") {
		t.Error("bad name")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
