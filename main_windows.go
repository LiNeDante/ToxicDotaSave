//go:build windows && !dev

package main

import (
	_ "embed"
	"path/filepath"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed ui/index.html
var indexHTML string

func main() {
	app, err := newApp()
	if err != nil {
		fatalBox("Could not create the app data folder:\n" + err.Error())
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  filepath.Join(app.dataDir, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  appName,
			Width:  1040,
			Height: 680,
			IconId: 1, // from rsrc (go-winres), falls back to default if missing
			Center: true,
		},
	})
	if w == nil {
		fatalBox("Microsoft Edge WebView2 Runtime is required.\n\n" +
			"Download it from:\nhttps://developer.microsoft.com/microsoft-edge/webview2/")
		return
	}
	defer w.Destroy()

	useDarkTitleBar(uintptr(w.Window()))
	w.SetSize(900, 600, webview2.HintMin)
	_ = w.Bind("__call", func(name, args string) (any, error) { return app.call(name, args) })
	w.SetHtml(indexHTML)
	w.Run()
}

// useDarkTitleBar asks DWM for the dark caption (Windows 10 20H1+ / 11).
func useDarkTitleBar(hwnd uintptr) {
	dwm := windows.NewLazySystemDLL("dwmapi.dll")
	proc := dwm.NewProc("DwmSetWindowAttribute")
	if proc.Find() != nil {
		return
	}
	on := int32(1)
	const dwmwaUseImmersiveDarkMode = 20
	proc.Call(hwnd, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on))
	// Caption colour (Windows 11 only; ignored elsewhere): matches the UI.
	color := uint32(0x00100D0B) // COLORREF 0x00BBGGRR → #0b0d10
	const dwmwaCaptionColor = 35
	proc.Call(hwnd, dwmwaCaptionColor, uintptr(unsafe.Pointer(&color)), unsafe.Sizeof(color))
}

func fatalBox(msg string) {
	t, _ := windows.UTF16PtrFromString(appName)
	m, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, m, t, windows.MB_OK|windows.MB_ICONERROR)
}
