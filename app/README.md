# FreeReps iOS App

FreeReps is an iOS companion app that syncs Apple HealthKit data to a FreeReps server via HTTP. Your health data flows from HealthKit on your phone to your self-hosted server — no cloud services, no third parties.

## What it syncs

- **85 quantity types** — steps, heart rate, blood pressure, blood glucose, body temperature, VO2 max, nutrition (all macros and micronutrients), audio exposure, and more
- **22 category types** — sleep analysis, menstrual cycles, symptoms, mindfulness, heart events, stand hours
- **Workouts** — activity type, duration, energy burned, distance, swim strokes, flights climbed
- **Blood pressure** — systolic/diastolic correlation pairs
- **ECG recordings** — classification, heart rate, voltage measurements
- **Audiograms** — hearing sensitivity by frequency
- **Workout routes** — GPS coordinates recorded during workouts
- **Activity summaries** — daily ring data (active energy, exercise minutes, stand hours)
- **Medications** — name, dosage, start/end dates (iOS 26+)

## Features

- **Full and incremental sync** — initial backfill of all historical data, then ongoing incremental syncs for new records
- **Real-time background sync** — registers HealthKit observer queries for immediate delivery when new data is recorded
- **Background processing** — periodic sync via BGProcessingTask when the app isn't active
- **Live Activity** — sync progress on the lock screen and Dynamic Island
- **Data browser** — browse all synced data by category with search and filtering
- **Location tracking** — continuous GPS logging and geofence-based check-ins with customizable place categories
- **Re-sync and repair** — per-category re-sync to repair or backfill data that may have been missed
- **No dependencies** — pure Swift using only Apple frameworks (HealthKit, BackgroundTasks, ActivityKit)

## Architecture

```
HealthKit (iPhone) → HTTP JSON → FreeReps Server → PostgreSQL + TimescaleDB
```

The app uses `FreeRepsService` (a lightweight `URLSession` HTTP wrapper) to POST health data as JSON to the FreeReps server's REST API (`/api/v1/ingest/` and `/api/v1/import`). There is no direct database connection — the server handles all storage.

## Requirements

- iOS 16.2+
- Physical device (HealthKit is not available in the Simulator)
- A running FreeReps server (see the [server README](../server/README.md))
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
   - HealthKit (with Background Delivery)
   - Background Modes: Background processing, Background fetch

5. Verify build settings point to the right files:
   - `INFOPLIST_FILE` = `Sources/FreeReps/Resources/Info.plist`
   - `CODE_SIGN_ENTITLEMENTS` = `Sources/FreeReps/Resources/FreeReps.entitlements`

6. Build and run on a physical device.

### Project structure

```
Sources/FreeReps/
  FreeRepsApp.swift              App entry point and background task registration
  ContentView.swift              Root TabView (Sync, Browse, Settings)
  Models/
    FreeRepsConfig.swift         Connection config (host, port, HTTPS toggle)
    SyncState.swift              Observable sync state
    HealthDataType.swift         All HealthKit type descriptors
    HealthRecord.swift           Record models for the data browser
    ...
  Services/
    FreeRepsService.swift        HTTP client for the FreeReps API (URLSession)
    HealthKitService.swift       HealthKit queries and permissions
    SyncService.swift            Full/incremental sync orchestration
    BackgroundSyncManager.swift  HKObserverQuery registration and background delivery
  ViewModels/                    View models for each tab
  Views/
    Sync/                        Sync dashboard and category status cards
    DataBrowser/                 Data browsing views per type
    Settings/                    All settings and configuration views
  Resources/
    Info.plist                   HealthKit usage description, BG task identifiers
    FreeReps.entitlements        HealthKit entitlements
Sources/FreeRepsWidgets/         Live Activity widget for sync progress
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

1. Install the app on your iPhone.
2. Go to **Settings > FreeReps Connection** and configure your server's host and port.
3. Tap **Test Connection** to verify connectivity.
4. Go to **Settings > Apple Health Permissions** and grant access to the health data types you want to sync.
5. Return to the **Sync** tab and tap **Full Sync** to backfill your historical data. **Keep the screen on until the full sync completes** — HealthKit is not accessible when the device is locked. The app enables "Keep Screen On" by default during full sync (configurable in Settings).

### Ongoing sync

After the initial full sync, the app automatically syncs new data in the background:

- **Observer-based**: HealthKit notifies the app immediately when new data is recorded (requires the app to have run recently)
- **Scheduled**: A background processing task runs periodically (approximately every 15 minutes, subject to iOS scheduling)
- **Manual**: Pull down on the Sync tab to trigger an incremental sync

### Location tracking

Enable location tracking in **Settings > Location & Places** to log GPS coordinates. You can also set up geofences around places (home, office, gym, etc.) to log check-in and check-out events.

## App Store review: temporary test server

Since FreeReps requires a server to function, App Store reviewers need a reachable server during review. To set one up temporarily:

1. Deploy a FreeReps server on a VPS (any cloud provider works).
2. Set `tailscale.enabled: false` in `config.yaml` so the server listens on plain HTTP.
3. Set up HTTPS via a reverse proxy (e.g., Caddy, nginx + Let's Encrypt) or a cloud load balancer.
4. In App Store Connect review notes, provide the server URL and any necessary instructions.
5. Tear down the server after review approval.

No code changes are needed — the app already supports arbitrary host/port/HTTPS configuration.

## Known quirks and limitations

### HealthKit background access

HealthKit restricts data access when the device is locked:

- **Screen on / app in foreground**: Full HealthKit access. All syncs work normally.
- **Screen off but recently unlocked**: HealthKit data remains accessible briefly. Observer-based background syncs triggered by new health data typically succeed.
- **Device locked for a long time**: HealthKit queries return authorization errors. Background syncs skip and retry next time.
- **Full sync requires screen on**: A full historical backfill needs continuous HealthKit access. Use the "Keep Screen On" toggle.

In practice, incremental syncs work well because they're triggered right when new data is recorded and the data volume per sync is small.

### VPN and Tailscale

If your server is only reachable via Tailscale, be aware that iOS aggressively manages VPN connections:

- iOS may disconnect VPN tunnels in the background to save battery.
- Tailscale's iOS app uses NEPacketTunnelProvider, which is subject to the same iOS restrictions.
- When the tunnel is down, connections will time out. The app retries on the next sync cycle.

### iOS background execution limits

- `BGProcessingTask` runs at iOS's discretion — typically when charging and on Wi-Fi.
- iOS may defer background tasks if the device is low on battery or the app hasn't been used recently.
- After a force-quit, background tasks stop until the app is launched again.

## License

This project is released under the MIT License. See [LICENSE](LICENSE) for details.
