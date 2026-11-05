package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Branding lives here only; the UI reads it from getState.
const (
	appName    = "Toxic Dota Save" // working title — final name TBD
	appVersion = "1.0.0"
	creditText = "Made by kick.com/thetoxicparty - cowhite"
	creditURL  = "https://kick.com/thetoxicparty"
)

type Settings struct {
	Lang string `json:"lang"`
}

type App struct {
	mu      sync.Mutex
	dataDir string
}

type State struct {
	AppName       string    `json:"appName"`
	AppVersion    string    `json:"appVersion"`
	Credit        string    `json:"credit"`
	CreditURL     string    `json:"creditUrl"`
	SteamFound    bool      `json:"steamFound"`
	SteamDir      string    `json:"steamDir"`
	SteamRunning  bool      `json:"steamRunning"` // a user is logged in right now
	ActiveAccount string    `json:"activeAccount"`
	DotaRunning   bool      `json:"dotaRunning"`
	Accounts      []Account `json:"accounts"`
	BackupsDir    string    `json:"backupsDir"`
	Settings      Settings  `json:"settings"`
}

// dataDirOverride, when set (dev harness), replaces the per-user data folder
// so development never touches real backups.
var dataDirOverride string

func newApp() (*App, error) {
	dir := dataDirOverride
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(base, "ToxicDotaSave")
	}
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0o755); err != nil {
		return nil, err
	}
	return &App{dataDir: dir}, nil
}

func (a *App) backupsDir() string { return filepath.Join(a.dataDir, "backups") }

func (a *App) settings() Settings {
	var s Settings
	if b, err := os.ReadFile(filepath.Join(a.dataDir, "settings.json")); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func (a *App) steam() (dir string, accounts []Account, active string, err error) {
	dir, err = platformSteamDir()
	if err != nil {
		return "", nil, "", err
	}
	active = platformActiveAccount()
	accounts = listAccounts(dir, active)
	return dir, accounts, active, nil
}

// backupResult reports a duplicate as a normal answer the UI can act on
// ({"duplicateOf": ...}) rather than as an error string.
func backupResult(b BackupInfo, err error) (any, error) {
	var dup *DuplicateError
	if errors.As(err, &dup) {
		return map[string]any{"duplicateOf": dup.Existing}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"backup": b}, nil
}

func findAccount(accounts []Account, id string) (Account, bool) {
	for _, acc := range accounts {
		if acc.AccountID == id {
			return acc, true
		}
	}
	return Account{}, false
}

func (a *App) getState() State {
	s := State{
		AppName: appName, AppVersion: appVersion, Credit: creditText, CreditURL: creditURL,
		BackupsDir: a.backupsDir(), Settings: a.settings(), DotaRunning: platformDotaRunning(),
		Accounts: []Account{},
	}
	dir, accounts, active, err := a.steam()
	if err != nil {
		return s
	}
	s.SteamFound, s.SteamDir, s.Accounts = true, dir, accounts
	s.SteamRunning = active != ""
	if active == "" {
		active = mostRecentAccountID(accounts)
	}
	s.ActiveAccount = active
	return s
}

// call is the single entry point used by both the WebView binding and the
// dev HTTP server. args is a JSON array.
func (a *App) call(name string, rawArgs string) (any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var args []string
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return nil, errors.New("bad arguments")
		}
	}
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}

	switch name {
	case "getState":
		return a.getState(), nil

	case "getStatus": // cheap poll, no avatars
		return map[string]any{"activeAccount": platformActiveAccount(), "dotaRunning": platformDotaRunning()}, nil

	case "listBackups":
		return listBackups(a.backupsDir()), nil

	case "createBackup": // accountId, label
		dir, accounts, _, err := a.steam()
		if err != nil {
			return nil, errors.New("steam_not_found")
		}
		acc, ok := findAccount(accounts, arg(0))
		if !ok {
			return nil, errors.New("account_not_found")
		}
		return backupResult(createBackup(dir, a.backupsDir(), acc, arg(1), false))

	case "restoreBackup": // backupId, targetAccountId
		if platformDotaRunning() {
			return nil, ErrDotaRunning
		}
		dir, accounts, _, err := a.steam()
		if err != nil {
			return nil, errors.New("steam_not_found")
		}
		target, ok := findAccount(accounts, arg(1))
		if !ok {
			return nil, errors.New("account_not_found")
		}
		if _, err := readBackupInfo(a.backupsDir(), arg(0)); err != nil {
			return nil, err
		}
		// Safety net: snapshot whatever the target has before overwriting.
		var safety *BackupInfo
		if target.HasDota {
			b, err := createBackup(dir, a.backupsDir(), target, "", true)
			var dup *DuplicateError
			switch {
			case err == nil:
				safety = &b
			case errors.As(err, &dup):
				safety = &dup.Existing // already safe in a manual backup
			case err.Error() != "no_dota_settings":
				return nil, fmt.Errorf("safety backup failed: %w", err)
			}
		}
		if err := restoreBackup(dir, a.backupsDir(), arg(0), target.AccountID); err != nil {
			return nil, err
		}
		return map[string]any{"safetyBackup": safety}, nil

	case "importBackup": // base64 zip
		data, err := base64.StdEncoding.DecodeString(arg(0))
		if err != nil {
			return nil, errors.New("invalid_backup")
		}
		return backupResult(importBackupData(a.backupsDir(), data))

	case "deleteBackup":
		return nil, deleteBackup(a.backupsDir(), arg(0))

	case "revealBackup":
		p, err := backupPath(a.backupsDir(), arg(0))
		if err != nil {
			return nil, err
		}
		return nil, platformReveal(p, true)

	case "openBackupsDir":
		return nil, platformReveal(a.backupsDir(), false)

	case "openCredit":
		if creditURL == "" {
			return nil, nil
		}
		return nil, platformOpenURL(creditURL)

	case "setLang":
		if l := arg(0); l != "tr" && l != "en" {
			return nil, errors.New("bad lang")
		}
		s := a.settings()
		s.Lang = arg(0)
		b, _ := json.Marshal(s)
		return nil, writeFileAtomic(filepath.Join(a.dataDir, "settings.json"), b)
	}
	return nil, errors.New("unknown method: " + name)
}
