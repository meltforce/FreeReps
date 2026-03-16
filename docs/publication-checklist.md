# Publication Checklist

Manual tasks remaining for App Store + open-source release.

## Xcode Project

- [x] Add `PrivacyInfo.xcprivacy` to the Xcode target's "Copy Bundle Resources" build phase
- [x] Build iOS app in Xcode — verify settings UI, test mode toggle, version links
- [x] Add `ITSAppUsesNonExemptEncryption = NO` to Info.plist

## App Store Submission

- [x] Take screenshots on device (iPhone 6.5" framed, resized to 1284×2778)
- [x] Write app description and keywords in App Store Connect
- [x] Set category to Health & Fitness
- [x] Set subtitle to "HealthKit Sync to Your Server"
- [x] Set Support URL to `https://github.com/meltforce/freereps/issues`
- [x] Set Marketing URL to `https://freereps.meltforce.org`
- [x] Set Privacy Policy URL to `https://freereps.meltforce.org/privacy/`
- [x] Set age rating (4+, all None)
- [x] Set App Privacy (Data Not Collected)
- [x] Archive + upload via Xcode

## App Store Review

- [x] Spin up a temporary public-facing test server (no Tailscale) with demo data — `https://freereps-test.meltforce.net/`
- [x] Document the test server host + "enable Test Mode" instructions in review notes
- [x] Disable "Sign-in required" in App Review Information
- [x] Select build in App Store Connect
- [x] Click "Add for Review"
- [x] Submit for review

## Homepage

- [x] Copy app icon to `docs/assets/app-icon.png`
- [x] Add iOS screenshots to `docs/screenshots/` (optional, for homepage)
- [x] Add lightbox to homepage for zoomable screenshots
- [x] Enable GitHub Pages from `docs/` on `main` branch in repo settings
- [x] Configure DNS CNAME for `freereps.meltforce.org`

## Releases

- [x] Tag server release: `git tag v1.0.0 && git push origin v1.0.0`
- [x] Tag app release: `git tag app/v1.0.0 && git push origin app/v1.0.0`
- [x] Verify `release.yml` builds Docker image `meltforce/freereps:1.0.0` + `latest`

## Post-Review

- [ ] Tear down the test server after App Store approval
