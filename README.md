# Toxic Dota Save

Back up your Dota 2 settings and load them onto any Steam account — your own smurfs, or a shared
account someone else just reconfigured. Windows and macOS, ~4 MB, no installer.

**[⬇ Download the latest release](../../releases/latest)** — `ToxicDotaSave-Windows.exe` (just run it) or `ToxicDotaSave-macOS.dmg`

[Türkçe ↓](#türkçe)

## How to use

1. Pick your account on the left (the logged-in one is preselected).
2. **Back up** saves your settings.
3. **Load** on a backup → choose the target account (online or offline).
4. Close Dota before loading.
5. The target's previous settings are backed up automatically first.
6. To share, send the `.zip` from the backup folder; the other person adds it with **Import** (or drag & drop).

What gets saved: everything Dota keeps per account in `Steam/userdata/<account>/570` — hotkeys,
game options, chat wheel, etc. Identical settings are never saved twice.

If Steam shows a *Cloud Sync Conflict* when Dota starts after a load, choose the **local** files.

### First launch

The app is new and builds its reputation over time, so you may see a warning once:

- **Windows** — "Windows protected your PC": click **More info → Run anyway**.
  Requires Microsoft Edge WebView2 (built into Windows 11 and current Windows 10).
- **macOS** — open the `.dmg`, drag the app into **Applications**, then **right-click it → Open → Open**.
  If that's not offered: System Settings → Privacy & Security → **Open Anyway**.

Every release lists SHA-256 checksums (`SHA256SUMS.txt`) and is built from this repository by
GitHub Actions — you can check exactly which commit produced it.

## Privacy

The app makes **no network connections**. No accounts, telemetry, analytics or updates. It reads
Steam's local files and writes backups to:

- Windows: `%APPDATA%\ToxicDotaSave\backups`
- macOS: `~/Library/Application Support/ToxicDotaSave/backups`

The only time anything leaves your machine is when you click the credit link, which opens your browser.

## Build from source

Requires Go (see `go.mod`). The macOS build needs Xcode Command Line Tools.

```sh
./build.sh windows   # dist/ToxicDotaSave-Windows.exe (any OS)
./build.sh mac       # dist/ToxicDotaSave-macOS.dmg  (macOS, universal arm64 + x86_64)
go test .
```

UI development in a browser against a fake Steam folder (never touches real backups):

```sh
go run ./dev/fixture dev/steam
STEAM_DIR=dev/steam ACTIVE_USER=11111111 go run -tags dev .   # http://127.0.0.1:5173
```

Layout: `steam.go` (accounts), `backup.go` (zip format, restore, duplicate detection),
`api.go` (the calls the UI makes), `ui/index.html` (the whole interface), `platform_*.go` /
`main_*.go` (Windows WebView2 and macOS WKWebView shells).

## Türkçe

Dota 2 ayarlarını yedekle, istediğin Steam hesabına tek tıkla yükle. Kendi smurf hesapların ya da
arkadaşınla paylaştığın hesap için. Windows ve macOS, ~4 MB, kurulum yok.

**[⬇ Son sürümü indir](../../releases/latest)**

1. Soldan hesabını seç.
2. **Yedekle** ile ayarlarını kaydet.
3. Yedekte **Yükle**'ye bas, hedef hesabı seç (açık ya da kapalı hesap).
4. Yüklemeden önce Dota'yı kapat.
5. Hedef hesabın eski ayarları önce otomatik yedeklenir.
6. Paylaşmak için yedek klasöründeki `.zip`'i gönder; karşı taraf **İçe aktar** ile ekler.

**İlk açılış:** Windows'ta "Bilgisayarınız korundu" çıkarsa **Ek bilgi → Yine de çalıştır**.
macOS'ta `.dmg`'yi aç, uygulamayı **Applications**'a sürükle, sonra **sağ tık → Aç → Aç**.

Uygulama internete bağlanmaz, veri toplamaz.

---

Made by [kick.com/thetoxicparty](https://kick.com/thetoxicparty) — cowhite · [MIT License](LICENSE)

Not affiliated with Valve Corporation. Dota 2 and Steam are trademarks of Valve Corporation.
