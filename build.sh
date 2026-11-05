#!/bin/sh
# Release builds into dist/:
#   ./build.sh windows  → dist/ToxicDotaSave-Windows.exe   (any OS, no cgo)
#   ./build.sh mac      → dist/ToxicDotaSave-macOS.dmg     (macOS only, universal arm64+x86_64)
#   ./build.sh          → both
set -e
cd "$(dirname "$0")"

NAME="Toxic Dota Save"
BIN="ToxicDotaSave"
BUNDLE_ID="com.toxicdotasave.app"
VERSION=$(sed -n 's/.*appVersion *= *"\(.*\)".*/\1/p' api.go)

build_windows() {
	GOWINRES="$(go env GOPATH)/bin/go-winres"
	[ -x "$GOWINRES" ] || go install github.com/tc-hib/go-winres@latest
	"$GOWINRES" make --in winres/winres.json --arch amd64 --product-version "$VERSION" --file-version "$VERSION"
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o "dist/$BIN-Windows.exe" .
	echo "→ dist/$BIN-Windows.exe"
}

build_mac() {
	[ "$(uname)" = Darwin ] || { echo "mac build needs macOS"; exit 1; }
	tmp=$(mktemp -d)
	export CGO_ENABLED=1 CGO_CXXFLAGS="-Wno-deprecated-literal-operator" MACOSX_DEPLOYMENT_TARGET=11.0
	GOARCH=arm64 CC="clang -arch arm64" CXX="clang++ -arch arm64" go build -trimpath -ldflags "-s -w" -o "$tmp/arm64" .
	GOARCH=amd64 CC="clang -arch x86_64" CXX="clang++ -arch x86_64" go build -trimpath -ldflags "-s -w" -o "$tmp/amd64" .

	app="$tmp/$NAME.app"
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	lipo -create -output "$app/Contents/MacOS/$BIN" "$tmp/arm64" "$tmp/amd64"

	iconset="$tmp/icon.iconset"
	mkdir "$iconset"
	for s in 16 32 128 256 512; do
		sips -z $s $s winres/icon-mac.png --out "$iconset/icon_${s}x${s}.png" >/dev/null
		sips -z $((s * 2)) $((s * 2)) winres/icon-mac.png --out "$iconset/icon_${s}x${s}@2x.png" >/dev/null
	done
	iconutil -c icns "$iconset" -o "$app/Contents/Resources/icon.icns"

	cat >"$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleName</key><string>$NAME</string>
	<key>CFBundleDisplayName</key><string>$NAME</string>
	<key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
	<key>CFBundleExecutable</key><string>$BIN</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$VERSION</string>
	<key>CFBundleVersion</key><string>$VERSION</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.utilities</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSHumanReadableCopyright</key><string>Not affiliated with Valve Corporation.</string>
</dict></plist>
EOF
	codesign --force --deep --sign - "$app"
	rm -rf "dist/$NAME.app" "dist/$BIN-macOS.dmg"
	mkdir -p dist
	cp -R "$app" "dist/"

	# Drag-to-Applications disk image.
	stage="$tmp/dmg"
	mkdir "$stage"
	cp -R "$app" "$stage/"
	ln -s /Applications "$stage/Applications"
	hdiutil create -quiet -volname "$NAME" -srcfolder "$stage" -fs HFS+ -format UDZO -ov "dist/$BIN-macOS.dmg"
	rm -rf "$tmp"
	echo "→ dist/$NAME.app"
	echo "→ dist/$BIN-macOS.dmg"
}

go test .
case "${1:-all}" in
	windows) build_windows ;;
	mac) build_mac ;;
	all) build_windows; [ "$(uname)" = Darwin ] && build_mac ;;
	*) echo "usage: $0 [windows|mac|all]"; exit 1 ;;
esac
