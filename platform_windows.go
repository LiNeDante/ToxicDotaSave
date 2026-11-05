//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func platformSteamDir() (string, error) {
	var candidates []string
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		if v, _, err := k.GetStringValue("SteamPath"); err == nil {
			candidates = append(candidates, filepath.FromSlash(v))
		}
		k.Close()
	}
	for _, key := range []string{`SOFTWARE\WOW6432Node\Valve\Steam`, `SOFTWARE\Valve\Steam`} {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE); err == nil {
			if v, _, err := k.GetStringValue("InstallPath"); err == nil {
				candidates = append(candidates, v)
			}
			k.Close()
		}
	}
	candidates = append(candidates, filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"), `C:\Program Files (x86)\Steam`)
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "userdata")); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("steam not found")
}

// platformActiveAccount returns the account ID Steam is logged in with
// right now, or "" when Steam is closed / logged out.
func platformActiveAccount() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam\ActiveProcess`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("ActiveUser")
	if err != nil || v == 0 {
		return ""
	}
	return strconv.FormatUint(v, 10)
}

func platformDotaRunning() bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if strings.EqualFold(windows.UTF16ToString(pe.ExeFile[:]), "dota2.exe") {
			return true
		}
	}
	return false
}

func platformReveal(p string, selectFile bool) error {
	if !selectFile {
		return shellOpen(p)
	}
	// explorer needs the raw form /select,"path"; Go's own argument quoting
	// would wrap the whole switch in quotes.
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + p + `"`}
	_ = cmd.Start() // explorer's exit code is meaningless
	return nil
}

func platformOpenURL(u string) error { return shellOpen(u) }

func shellOpen(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, t, nil, nil, windows.SW_SHOWNORMAL)
}
