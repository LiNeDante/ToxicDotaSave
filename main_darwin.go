//go:build darwin && !dev

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// webview gives us a bare NSWindow without a menu bar, which means no Cmd+Q
// and no copy/paste in text fields. Add the standard App + Edit menus and a
// dark, seamless title bar that matches the UI.
static void styleWindow(void *win, const char *appName) {
	NSWindow *w = (__bridge NSWindow *)win;
	w.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
	w.titlebarAppearsTransparent = YES;
	w.backgroundColor = [NSColor colorWithSRGBRed:0.043 green:0.051 blue:0.063 alpha:1];

	NSString *name = [NSString stringWithUTF8String:appName];
	NSMenu *bar = [NSMenu new];

	NSMenuItem *appItem = [NSMenuItem new];
	NSMenu *appMenu = [[NSMenu alloc] initWithTitle:name];
	[appMenu addItemWithTitle:[@"Hide " stringByAppendingString:name] action:@selector(hide:) keyEquivalent:@"h"];
	[appMenu addItem:[NSMenuItem separatorItem]];
	[appMenu addItemWithTitle:[@"Quit " stringByAppendingString:name] action:@selector(terminate:) keyEquivalent:@"q"];
	appItem.submenu = appMenu;
	[bar addItem:appItem];

	NSMenuItem *editItem = [NSMenuItem new];
	NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Edit"];
	[edit addItemWithTitle:@"Undo" action:@selector(undo:) keyEquivalent:@"z"];
	[edit addItemWithTitle:@"Redo" action:@selector(redo:) keyEquivalent:@"Z"];
	[edit addItem:[NSMenuItem separatorItem]];
	[edit addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
	[edit addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
	[edit addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
	[edit addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
	editItem.submenu = edit;
	[bar addItem:editItem];

	NSMenuItem *winItem = [NSMenuItem new];
	NSMenu *winMenu = [[NSMenu alloc] initWithTitle:@"Window"];
	[winMenu addItemWithTitle:@"Minimize" action:@selector(performMiniaturize:) keyEquivalent:@"m"];
	[winMenu addItemWithTitle:@"Close" action:@selector(performClose:) keyEquivalent:@"w"];
	winItem.submenu = winMenu;
	[bar addItem:winItem];

	NSApp.mainMenu = bar;
	[w center];
}
*/
import "C"

import (
	_ "embed"
	"log"
	"unsafe"

	webview "github.com/webview/webview_go"
)

//go:embed ui/index.html
var indexHTML string

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatal(err)
	}
	w := webview.New(false)
	if w == nil {
		log.Fatal("could not create window")
	}
	defer w.Destroy()

	w.SetTitle(appName)
	w.SetSize(1040, 680, webview.HintNone)
	w.SetSize(900, 600, webview.HintMin)
	name := C.CString(appName)
	C.styleWindow(w.Window(), name)
	C.free(unsafe.Pointer(name))

	_ = w.Bind("__call", func(name, args string) (any, error) { return app.call(name, args) })
	w.SetHtml(indexHTML)
	w.Run()
}
