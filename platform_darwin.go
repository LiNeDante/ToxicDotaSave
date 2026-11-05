//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// STEAM_DIR / ACTIVE_USER / $TMPDIR/fake-dota-running exist for the dev
// harness and fixtures; a normal launch never sets them.

func steamRoot() string {
	if d := os.Getenv("STEAM_DIR"); d != "" {
		abs, _ := filepath.Abs(d)
		return abs
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Steam")
}

func platformSteamDir() (string, error) {
	d := steamRoot()
	if st, err := os.Stat(filepath.Join(d, "userdata")); err != nil || !st.IsDir() {
		return "", errors.New("steam not found")
	}
	return d, nil
}

// platformActiveAccount reads Steam's registry.vdf (the macOS stand-in for the
// Windows registry). ActiveProcess/ActiveUser is only meaningful while the
// client runs; if it's missing, the AutoLoginUser login name is mapped back
// through loginusers.vdf.
func platformActiveAccount() string {
	if v, ok := os.LookupEnv("ACTIVE_USER"); ok {
		return v
	}
	if !processRunning("steam_osx") {
		return ""
	}
	dir := steamRoot()
	raw, err := os.ReadFile(filepath.Join(dir, "registry.vdf"))
	if err != nil {
		return ""
	}
	root, err := parseVDF(string(raw))
	if err != nil {
		return ""
	}
	steam := root.Child("Registry").Child("HKCU").Child("Software").Child("Valve").Child("Steam")
	if id := steam.Child("ActiveProcess").Get("ActiveUser"); id != "" && id != "0" {
		if _, err := strconv.ParseUint(id, 10, 32); err == nil {
			return id
		}
	}
	login := steam.Get("AutoLoginUser")
	if login == "" {
		return ""
	}
	for _, a := range listAccounts(dir, "") {
		if strings.EqualFold(a.AccountName, login) {
			return a.AccountID
		}
	}
	return ""
}

func platformDotaRunning() bool {
	if _, err := os.Stat(filepath.Join(os.TempDir(), "fake-dota-running")); err == nil {
		return true
	}
	return processRunning("dota2")
}

func processRunning(name string) bool {
	return exec.Command("/usr/bin/pgrep", "-x", name).Run() == nil
}

func platformReveal(p string, selectFile bool) error {
	if selectFile {
		return exec.Command("/usr/bin/open", "-R", p).Start()
	}
	return exec.Command("/usr/bin/open", p).Start()
}

func platformOpenURL(u string) error { return exec.Command("/usr/bin/open", u).Start() }
