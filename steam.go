package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const (
	dotaAppID      = "570"
	steamID64Base  = 76561197960265728
	maxAvatarBytes = 512 << 10
)

type Account struct {
	AccountID   string `json:"accountId"` // userdata folder name (SteamID3 account number)
	SteamID64   string `json:"steamId64"`
	PersonaName string `json:"personaName"`
	AccountName string `json:"accountName"`
	Active      bool   `json:"active"`
	MostRecent  bool   `json:"mostRecent"`
	HasDota     bool   `json:"hasDota"`
	LastLogin   int64  `json:"lastLogin"`
	Avatar      string `json:"avatar,omitempty"` // data: URI
}

func accountIDToSteamID64(id string) string {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(n+steamID64Base, 10)
}

func steamID64ToAccountID(id string) string {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n < steamID64Base {
		return ""
	}
	return strconv.FormatUint(n-steamID64Base, 10)
}

func dotaDir(steamDir, accountID string) string {
	return filepath.Join(steamDir, "userdata", accountID, dotaAppID)
}

// listAccounts merges config/loginusers.vdf with the userdata folders, so
// accounts that were removed from the login list but still hold Dota
// settings on disk are listed too.
func listAccounts(steamDir, activeAccountID string) []Account {
	byID := map[string]*Account{}

	if raw, err := os.ReadFile(filepath.Join(steamDir, "config", "loginusers.vdf")); err == nil {
		if root, err := parseVDF(string(raw)); err == nil {
			users := root.Child("users")
			for _, sid := range users.ChildKeys() {
				u := users.Child(sid)
				aid := steamID64ToAccountID(sid)
				if aid == "" {
					continue
				}
				ts, _ := strconv.ParseInt(u.Get("Timestamp"), 10, 64)
				byID[aid] = &Account{
					AccountID:   aid,
					SteamID64:   sid,
					PersonaName: u.Get("PersonaName"),
					AccountName: u.Get("AccountName"),
					MostRecent:  u.Get("MostRecent") == "1",
					LastLogin:   ts,
				}
			}
		}
	}

	if entries, err := os.ReadDir(filepath.Join(steamDir, "userdata")); err == nil {
		for _, e := range entries {
			aid := e.Name()
			if !e.IsDir() || aid == "0" || accountIDToSteamID64(aid) == "" {
				continue
			}
			if _, ok := byID[aid]; !ok {
				byID[aid] = &Account{AccountID: aid, SteamID64: accountIDToSteamID64(aid)}
			}
		}
	}

	out := make([]Account, 0, len(byID))
	for aid, a := range byID {
		if a.PersonaName == "" {
			a.PersonaName = personaFromLocalConfig(steamDir, aid)
		}
		if a.PersonaName == "" {
			a.PersonaName = a.AccountName
		}
		if a.PersonaName == "" {
			a.PersonaName = "#" + aid
		}
		if st, err := os.Stat(dotaDir(steamDir, aid)); err == nil && st.IsDir() {
			a.HasDota = true
		}
		a.Active = aid == activeAccountID
		a.Avatar = loadAvatar(steamDir, a.SteamID64)
		out = append(out, *a)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		if out[i].HasDota != out[j].HasDota {
			return out[i].HasDota
		}
		return out[i].LastLogin > out[j].LastLogin
	})
	return out
}

func personaFromLocalConfig(steamDir, accountID string) string {
	raw, err := os.ReadFile(filepath.Join(steamDir, "userdata", accountID, "config", "localconfig.vdf"))
	if err != nil {
		return ""
	}
	root, err := parseVDF(string(raw))
	if err != nil {
		return ""
	}
	return root.Child("UserLocalConfigStore").Child("friends").Get("PersonaName")
}

func loadAvatar(steamDir, steamID64 string) string {
	if steamID64 == "" {
		return ""
	}
	p := filepath.Join(steamDir, "config", "avatarcache", steamID64+".png")
	st, err := os.Stat(p)
	if err != nil || st.Size() > maxAvatarBytes {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

// mostRecentAccountID is the fallback "current" account when Steam is not
// running and the registry has no active user.
func mostRecentAccountID(accounts []Account) string {
	for _, a := range accounts {
		if a.MostRecent {
			return a.AccountID
		}
	}
	return ""
}
