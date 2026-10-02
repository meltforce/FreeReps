# iOS App

- Open `app/FreeReps.xcodeproj` in Xcode
- Requires physical device (HealthKit unavailable in Simulator)
- Bundle ID: `com.meltforce.freereps`

## Uploading a build

```bash
tools/ios-upload.sh                 # archive, check, upload to App Store Connect
tools/ios-upload.sh --export-only   # archive, check, write the .ipa locally
```

Raise `CURRENT_PROJECT_VERSION` first; App Store Connect refuses a build number
it already has. Use the script rather than `xcodebuild -exportArchive`
directly: with Homebrew's rsync on PATH the export fails with "Copy failed",
and the script checks the App Intent metadata for the words App Store Connect
rejects (ITMS-90626). Its header carries both reasons.

## App Store review server

A submission needs a server the reviewer can reach, because the production
instance is tailnet-only (`DECISIONS.md`, 2026-09-20). It is deployed and
removed with the homelab tool `tools/review-server` — its `README.md` there
governs; nothing about it is restated here:

```bash
~/projects/homelab/tools/review-server/review-server deploy  freereps REVIEW_IMAGE_TAG=<release>
~/projects/homelab/tools/review-server/review-server destroy freereps   # after approval
```

Review notes: host `freereps-test.meltforce.net`, HTTPS on, port 443; the
instance holds 90 days of generated demo data. Add a ROADMAP row to destroy it
when the submission goes in, and remove the row when `destroy` has run.
