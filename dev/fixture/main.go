// Generates a fake Steam folder for the dev harness: go run ./dev/fixture dev/steam
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
)

type user struct {
	id, login, persona string
	recent             bool
	dota               bool
	c                  color.RGBA
}

func main() {
	root := "dev/steam"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	users := []user{
		{"11111111", "can_main", "ToxicCan", true, true, color.RGBA{180, 50, 40, 255}},
		{"22222222", "can_smurf1", "pos5 enjoyer", false, true, color.RGBA{60, 120, 180, 255}},
		{"33333333", "can_smurf2", "Ahmet", false, true, color.RGBA{70, 150, 80, 255}},
		{"44444444", "fresh_acc", "YeniHesap", false, false, color.RGBA{150, 120, 50, 255}},
	}
	vdf := "\"users\"\n{\n"
	for i, u := range users {
		n, _ := strconv.ParseUint(u.id, 10, 64)
		sid := strconv.FormatUint(n+76561197960265728, 10)
		recent := "0"
		if u.recent {
			recent = "1"
		}
		vdf += fmt.Sprintf("\t\"%s\"\n\t{\n\t\t\"AccountName\"\t\t\"%s\"\n\t\t\"PersonaName\"\t\t\"%s\"\n\t\t\"MostRecent\"\t\t\"%s\"\n\t\t\"Timestamp\"\t\t\"%d\"\n\t}\n",
			sid, u.login, u.persona, recent, 1730000000-i*86400)
		os.MkdirAll(filepath.Join(root, "userdata", u.id, "config"), 0o755)
		if u.dota {
			cfg := filepath.Join(root, "userdata", u.id, "570", "remote", "cfg")
			os.MkdirAll(cfg, 0o755)
			os.WriteFile(filepath.Join(cfg, "dotakeys_personal.lst"), []byte("\"KeyBindings\"\n{\n\t\"owner\" \""+u.persona+"\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(cfg, "config.cfg"), []byte("dota_camera_distance 1200 // "+u.persona+"\n"), 0o644)
			os.WriteFile(filepath.Join(root, "userdata", u.id, "570", "remotecache.vdf"), []byte("\"570\"\n{\n}\n"), 0o644)
		}
		avatar(filepath.Join(root, "config", "avatarcache", sid+".png"), u.c)
	}
	vdf += "}\n"
	os.MkdirAll(filepath.Join(root, "config"), 0o755)
	os.WriteFile(filepath.Join(root, "config", "loginusers.vdf"), []byte(vdf), 0o644)
	fmt.Println("fixture at", root)
}

func avatar(p string, c color.RGBA) {
	os.MkdirAll(filepath.Dir(p), 0o755)
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			k := uint8((x + y) / 2)
			img.Set(x, y, color.RGBA{c.R - c.R/4 + k/3, c.G - c.G/4 + k/3, c.B - c.B/4 + k/3, 255})
		}
	}
	f, _ := os.Create(p)
	png.Encode(f, img)
	f.Close()
}
