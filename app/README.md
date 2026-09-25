# FreeReps iOS App

> **Version 2.1 is coming to the App Store soon.** Version 1.0, in the App Store
> today, stopped syncing on iOS 27. Version 2.1 syncs on iOS 27, sends only what
> HealthKit added since the last sync, and adds a Home Screen widget and a
> Shortcuts action. It needs iOS 27 and a FreeReps server 2.1.

[![Download on the App Store](https://developer.apple.com/assets/elements/badges/download-on-the-app-store.svg)](https://apps.apple.com/us/app/freereps/id6760661354)

FreeReps for iOS sends Apple Health data to your own FreeReps server. Your health data flows from HealthKit on your phone to your self-hosted server — no cloud services, no third parties.

## Screenshots

Each image follows the reader's colour scheme, light or dark.

| | | | |
|:-:|:-:|:-:|:-:|
| <picture><source media="(prefers-color-scheme: dark)" srcset="../docs/screenshots/ios/framed-dashboard-dark.png"><img alt="Sync dashboard" src="../docs/screenshots/ios/framed-dashboard.png"></picture> | <picture><source media="(prefers-color-scheme: dark)" srcset="../docs/screenshots/ios/framed-syncing-dark.png"><img alt="Sync in progress" src="../docs/screenshots/ios/framed-syncing.png"></picture> | <picture><source media="(prefers-color-scheme: dark)" srcset="../docs/screenshots/ios/framed-widget-dark.png"><img alt="Home Screen widget" src="../docs/screenshots/ios/framed-widget.png"></picture> | <picture><source media="(prefers-color-scheme: dark)" srcset="../docs/screenshots/ios/framed-settings-dark.png"><img alt="Settings" src="../docs/screenshots/ios/framed-settings.png"></picture> |

## Acknowledgements

This app is based on [HealthBeat](https://github.com/kempu/HealthBeat) by kempu, an open-source iOS app for syncing Apple Health data. HealthBeat was adapted into the FreeReps companion app for the self-hosted FreeReps server. Licensed under the MIT License.

## What it syncs

- **36 quantity types** — activity and energy, distances, body weight and composition, heart rate and HRV, resting and walking heart rate, heart rate recovery, respiratory rate, SpO2, VO2 max, body and wrist temperature, time in daylight, effort scores, cycling metrics, caffeine and water
- **2 category types** — sleep analysis and mindful sessions
- **Workouts** — activity type, duration, energy burned, distance, per-minute heart rate
- **Blood pressure** — systolic/diastolic correlation pairs
- **Workout routes** — GPS coordinates recorded during workouts
- **Activity summaries** — daily ring data (active energy, exercise minutes, stand hours)
- **State of mind** (iOS 18+)

The list is fixed in `Sources/FreeReps/Models/HealthDataType.swift`; diagnoses, clinical records, prescriptions, symptoms and most nutrition are not read at all. Within it, the server decides per user which metrics it accepts (**Settings › Ingest** in the web UI), and the app skips the rest before reading HealthKit. The reasoning is in [`DECISIONS.md`](../DECISIONS.md), 2026-09-25.

## Features

- **Resumable backfill, then only what changed** — the first run backfills the configured range (one week to all data); later runs send what HealthKit added since the previous one
- **Shortcuts action** — "Sync Health Data" for Shortcuts automations, Siri and the Action button
- **Widget** — shows when the last sync finished and whether it succeeded; tapping it opens the app and starts a sync
- **Live Activity** — sync progress on the lock screen and Dynamic Island
- **Per-category re-sync** — long-press a category tile to sync or reset it
- **Alpha Progression import** — **Settings → Data → Import File** uploads a training CSV export
- **No dependencies** — pure Swift using only Apple frameworks (HealthKit, AppIntents, WidgetKit, ActivityKit)

## Architecture

```
HealthKit (iPhone) → HTTP JSON → FreeReps Server → PostgreSQL + TimescaleDB
```

The app uses `FreeRepsService` (a lightweight `URLSession` HTTP wrapper) to POST health data as JSON to the FreeReps server's REST API (`/api/v1/ingest/` and `/api/v1/import`). There is no direct database connection — the server handles all storage.

## Requirements

- iOS 27 or later ([`DECISIONS.md`](../DECISIONS.md), 2026-09-25)
- Physical device (HealthKit is not available in the Simulator)
- A running FreeReps server (see the [main README](../README.md))
- Apple Developer account (for HealthKit entitlement and code signing)

## Developer setup

1. Clone the repository:
   ```
   git clone https://github.com/your-username/FreeReps.git
   ```

2. Open the Xcode project:
   ```
   open app/FreeReps.xcodeproj
   ```

3. In **Signing & Capabilities**, select your development team and set a unique bundle identifier.

4. Verify the following capabilities are present (they should already be configured):
   - HealthKit, on the app target
   - App Groups with `group.com.meltforce.freereps`, on the app and the widget target; the widget reads the last sync outcome from it

5. Verify build settings point to the right files:
   - `INFOPLIST_FILE` = `Sources/FreeReps/Resources/Info.plist`
   - `CODE_SIGN_ENTITLEMENTS` = `Sources/FreeReps/Resources/FreeReps.entitlements`

6. Build and run on a physical device.

### Project structure

```
Sources/FreeReps/
  FreeRepsApp.swift              App entry point; handles freereps://sync and file imports
  ContentView.swift              Root TabView (Sync, Settings)
  Models/
    FreeRepsConfig.swift         Connection config (host, port, HTTPS toggle)
    SyncState.swift              Observable sync state
    HealthDataType.swift         All HealthKit type descriptors
    HealthRecord.swift           Record models
    ...
  Services/
    FreeRepsService.swift        HTTP client for the FreeReps API (URLSession)
    HealthKitService.swift       HealthKit queries and permissions
    SyncService.swift            Sync orchestration: resumable backfill, then anchored syncs
    SyncAnchors.swift            HealthKit anchors per type, stored after the server accepted
  Intents/
    SyncHealthDataIntent.swift   Shortcuts action "Sync Health Data"
  ViewModels/                    View models for each tab
  Views/
    Sync/                        Sync dashboard and category tiles
    Settings/                    All settings and configuration views
    Import/                      Result sheet of a file import
  Resources/
    Info.plist                   HealthKit usage description, freereps URL scheme
    AppShortcuts.xcstrings       German Siri phrases of the Shortcuts action
    FreeReps.entitlements        HealthKit and App Group entitlements
Sources/FreeRepsWidgets/         Live Activity for sync progress, last-sync widget
Sources/Shared/                  Compiled into both targets: deep link and last-sync record
```

## Connection modes

### Tailscale (production)

The recommended setup. Both your iPhone and FreeReps server join the same Tailnet. Authentication is handled by Tailscale — no credentials or API keys needed. The server uses `tsnet` for zero-config TLS.

In the app, set **Host** to your server's Tailscale hostname (e.g., `freereps.your-tailnet.ts.net`), **Port** to `443`, and enable **HTTPS**.

### Plain HTTP (local development)

For local development, point the app at your dev machine's IP address with HTTPS disabled.

Set **Host** to your machine's local IP (e.g., `192.168.1.100`), **Port** to `8080`, and disable **HTTPS**.

## User guide

### Initial setup

1. Install the app on an iPhone in your tailnet.
2. In **Settings**, tap the connection row at the top, set the server's host and port, and tap **Test Connection**.
3. Open **Settings → Apple Health Permissions** and grant access. On iOS 27 the permission sheet has a second step: choose **All Recorded Data and Future Data**. With **Past 30 Days**, HealthKit returns nothing older and reports no error.
4. Under **Settings → Advanced**, choose how far back the first sync reaches (one week to all data).
5. On the **Sync** tab, tap **Full Sync**. The backfill keeps the screen on while it runs, because HealthKit is unreadable once the iPhone locks; **Keep Screen On** in Settings switches that off.

### Ongoing sync

The app does not sync in the background on its own. A sync starts from one of:

- **Full Sync** on the Sync tab
- **The widget** — tapping it opens the app and starts a sync
- **The Shortcuts action "Sync Health Data"** — runs without opening the app, from Siri, the Action button or a personal automation

HealthKit data is readable only while the iPhone is unlocked, so an automation works when its trigger fires while the phone is in use: *Alarm is stopped*, *Apple Watch workout ends*, or *App is closed* for a training or ring app. A time-of-day trigger usually finds the phone locked; the action then reports "iPhone is locked" and skips the sync.

Siri understands "Sync FreeReps", "Sync health data with FreeReps" and "Start a FreeReps sync"; their German translations are in `Sources/FreeReps/Resources/AppShortcuts.xcstrings`.

After the first completed backfill, a sync sends only what HealthKit added since the previous one: the app keeps one `HKAnchoredObjectQuery` anchor per type and re-sends the hours those samples fall into. A sample HealthKit receives more than 7 days after its own time is not sent. Once a day the last 24 hours are re-sent in full. "Reset Sync State" clears the anchors, and the next sync is a backfill again.

## App Store review: temporary test server

Since FreeReps requires a server to function, App Store reviewers need a reachable server during review. To set one up temporarily:

1. Deploy a FreeReps server on a VPS (any cloud provider works).
2. Set `tailscale.enabled: false` in `config.yaml` so the server listens on plain HTTP.
3. Set up HTTPS via a reverse proxy (e.g., Caddy, nginx + Let's Encrypt) or a cloud load balancer.
4. In App Store Connect review notes, provide the server URL and any necessary instructions.
5. Tear down the server after review approval.

No code changes are needed — the app already supports arbitrary host/port/HTTPS configuration.

## Known quirks and limitations

### HealthKit access while locked

HealthKit rejects every read while the device is locked (`errorDatabaseInaccessible`). The Shortcuts action checks for this first and skips the sync with a message. A long backfill needs continuous access; the app keeps the screen on while it runs.

Since iOS 27, the read permission has a second stage: "Past 30 Days and Future Data" or "All Recorded Data and Future Data". With the 30-day grant, HealthKit returns nothing older than 30 days and reports no error, so a longer backfill range has no effect.

### VPN and Tailscale

If your server is only reachable via Tailscale, be aware that iOS aggressively manages VPN connections:

- iOS may disconnect VPN tunnels in the background to save battery.
- Tailscale's iOS app uses NEPacketTunnelProvider, which is subject to the same iOS restrictions.
- When the tunnel is down, connections time out and the sync fails; it has to be started again once the tunnel is up. On 2026-09-25 this surfaced as "A server with the specified hostname could not be found." while Tailscale was off on the phone.

## License

This project is released under the MIT License. See [LICENSE](../LICENSE) for details.
