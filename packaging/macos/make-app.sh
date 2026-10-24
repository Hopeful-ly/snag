#!/bin/sh
# Makes Snag.app and a disk image from release builds; runs on macOS.
#
#   packaging/macos/make-app.sh VERSION DIST
#
# DIST holds darwin-arm64/ and darwin-amd64/ (snag, snag-host) and tools/yt-dlp.
# Without $MACOS_SIGN_IDENTITY the app gets an ad-hoc signature, and Gatekeeper asks on first run.
set -e
version=${1:?usage: make-app.sh VERSION DIST}
dist=${2:?usage: make-app.sh VERSION DIST}
here=$(cd "$(dirname "$0")" && pwd)
app=$dist/Snag.app

rm -rf "$app" "$dist/dmg" "$dist/snag.iconset"
mkdir -p "$app/Contents/MacOS/tools" "$app/Contents/Resources"

for program in snag snag-host; do
    lipo -create -output "$app/Contents/MacOS/$program" \
        "$dist/darwin-arm64/$program" "$dist/darwin-amd64/$program"
done
cp "$dist/tools/yt-dlp" "$app/Contents/MacOS/tools/yt-dlp"
sed "s/__VERSION__/$version/g" "$here/Info.plist" > "$app/Contents/Info.plist"

iconset=$dist/snag.iconset
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
    cp "$here/../icons/snag-$size.png" "$iconset/icon_${size}x${size}.png"
    cp "$here/../icons/snag-$((size * 2)).png" "$iconset/icon_${size}x${size}@2x.png"
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/snag.icns"
rm -rf "$iconset"

if [ -n "$MACOS_SIGN_IDENTITY" ]; then
    codesign --force --options runtime --timestamp --sign "$MACOS_SIGN_IDENTITY" \
        "$app/Contents/MacOS/tools/yt-dlp" "$app/Contents/MacOS/snag-host" "$app"
else
    codesign --force --deep --sign - "$app"
fi

mkdir -p "$dist/dmg"
cp -R "$app" "$dist/dmg/"
ln -s /Applications "$dist/dmg/Applications"
hdiutil create -volname Snag -srcfolder "$dist/dmg" -ov -format UDZO "$dist/Snag-$version-macos.dmg"
rm -rf "$dist/dmg"
if [ -n "$MACOS_SIGN_IDENTITY" ]; then
    codesign --force --timestamp --sign "$MACOS_SIGN_IDENTITY" "$dist/Snag-$version-macos.dmg"
fi
