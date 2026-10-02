#!/usr/bin/env bash
# Archives the iOS app in the version and build number the project carries and
# uploads it to App Store Connect.
#
#   tools/ios-upload.sh                 archive, check, upload
#   tools/ios-upload.sh --export-only   archive, check, write the .ipa locally
#
# Why this exists, rather than running xcodebuild directly:
#
#   * `xcodebuild -exportArchive` copies the app with /usr/bin/rsync, which is
#     openrsync. openrsync starts the receiving side as `rsync` from PATH, and
#     where Homebrew's rsync 3.x comes first there, that side rejects the -E
#     option openrsync passes: "rsync: on remote machine: --extended-attributes:
#     unknown option". The export then fails with nothing but "Copy failed"
#     (2026-10-02, build 2.2.0 (4)). PATH is therefore limited to the system
#     directories below.
#   * App Store Connect rejects an App Intent description that contains "apple"
#     or "iphone" (ITMS-90626), and reports it by mail after the upload; build
#     2.2.0 (3) was rejected that way. The check below reads the archived intent
#     metadata and stops before the upload instead.
#
# The upload needs the Apple account of team R43S29F4G5 signed in under Xcode →
# Settings → Accounts; without it the export fails with "Failed to Use Accounts".
set -euo pipefail

export PATH=/usr/bin:/bin:/usr/sbin:/sbin

cd "$(dirname "$0")/.."

export_only=false
case "${1:-}" in
    "") ;;
    --export-only) export_only=true ;;
    *) echo "usage: $0 [--export-only]" >&2; exit 2 ;;
esac

out="$(mktemp -d -t freereps-ios)"
archive="$out/FreeReps.xcarchive"
options="app/ExportOptions.plist"

xcodebuild -project app/FreeReps.xcodeproj -scheme FreeReps \
    -destination 'generic/platform=iOS' -configuration Release \
    -archivePath "$archive" -allowProvisioningUpdates -quiet archive

info="$archive/Info.plist"
version="$(/usr/libexec/PlistBuddy -c 'Print :ApplicationProperties:CFBundleShortVersionString' "$info")"
build="$(/usr/libexec/PlistBuddy -c 'Print :ApplicationProperties:CFBundleVersion' "$info")"
echo "archived FreeReps ${version} (${build}) at ${archive}"

actions="$archive/Products/Applications/FreeReps.app/Metadata.appintents/extract.actionsdata"
if grep -q -i -E 'apple|iphone' "$actions"; then
    echo "App Intent metadata contains 'apple' or 'iphone', which App Store Connect rejects (ITMS-90626):" >&2
    strings "$actions" | grep -i -E 'apple|iphone' >&2 || true
    exit 1
fi

if $export_only; then
    cp "$options" "$out/ExportOptions.plist"
    /usr/libexec/PlistBuddy -c 'Set :destination export' "$out/ExportOptions.plist"
    options="$out/ExportOptions.plist"
fi

xcodebuild -exportArchive -archivePath "$archive" -exportOptionsPlist "$options" \
    -exportPath "$out/export" -allowProvisioningUpdates

if $export_only; then
    echo "exported ${out}/export/FreeReps.ipa"
else
    echo "uploaded FreeReps ${version} (${build}); App Store Connect processes it before it can be selected"
fi
