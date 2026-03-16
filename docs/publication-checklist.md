# Publication Checklist

Manual tasks remaining for App Store + open-source release.

## Xcode Project

- [ ] Add `PrivacyInfo.xcprivacy` to the Xcode target's "Copy Bundle Resources" build phase
- [ ] Build iOS app in Xcode — verify settings UI, test mode toggle, version links

## App Store Submission

- [ ] Take screenshots on device (iPhone 6.7" and 6.1" required)
- [ ] Write app description and keywords in App Store Connect
- [ ] Set category to Health & Fitness
- [ ] Set Support URL to `https://github.com/meltforce/freereps/issues`
- [ ] Set Privacy Policy URL to `https://freereps.meltforce.org/privacy/`
- [ ] Archive + upload via Xcode

## App Store Review

- [ ] Spin up a temporary public-facing test server (no Tailscale) with demo data
- [ ] Document the test server host + "enable Test Mode" instructions in review notes

## Homepage

- [x] Copy app icon to `docs/assets/app-icon.png`
- [ ] Add iOS screenshots to `docs/screenshots/` (optional, for homepage)
- [x] Enable GitHub Pages from `docs/` on `main` branch in repo settings
- [x] Configure DNS CNAME for `freereps.meltforce.org`

## Releases

- [ ] Tag server release: `git tag v1.0.0 && git push origin v1.0.0`
- [ ] Tag app release: `git tag app/v1.0.0 && git push origin app/v1.0.0`
- [ ] Verify `release.yml` builds Docker image `meltforce/freereps:1.0.0` + `latest`

## Post-Review

- [ ] Tear down the test server after App Store approval
