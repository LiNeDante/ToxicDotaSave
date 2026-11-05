//go:build dev

// Development harness: serves the UI and API over localhost so the UI can be
// iterated on in a browser against a fake Steam folder (see dev/fixture).
//
//	go run ./dev/fixture dev/steam
//	STEAM_DIR=dev/steam ACTIVE_USER=11111111 go run -tags dev .
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	dataDirOverride, _ = filepath.Abs(filepath.Join("dev", "data"))
	app, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("ui", "index.html"))
	})
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
			Args string `json:"args"`
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		res, err := app.call(req.Name, req.Args)
		if err != nil {
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"result": res})
	})
	addr := "127.0.0.1:" + envOr("PORT", "5173")
	fmt.Println("dev UI on http://"+addr+"  data:", app.dataDir)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
