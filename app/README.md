# FreeReps

FreeReps is an iOS app that continuously syncs Apple HealthKit data to a MySQL database. It connects directly to MySQL over TCP using a built-in wire protocol implementation — no backend server, no cloud service, no dependencies. Your health data goes straight from your phone to your database.

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
- **Data validation** — quick scan (count comparison) or deep scan (record-level comparison) with auto-repair
- **Backup and restore** — manual or automatic backups to iCloud Drive, with selective per-category restore
- **Multi-device iCloud sync** — settings, geofences, and place categories sync across devices; one device claims auto-sync rights to prevent conflicts
- **No dependencies** — pure Swift using only Apple frameworks (HealthKit, Network, BackgroundTasks, CoreLocation, ActivityKit, CloudKit)

## Requirements

- iOS 16.2+
- Physical device (HealthKit is not available in the Simulator)
- MySQL 5.7+ or 8.0+ server accessible from the device's network
- Apple Developer account (for HealthKit entitlement and code signing)

## Developer setup

1. Clone the repository:
   ```
   git clone https://github.com/your-username/FreeReps.git
   ```

2. Open the Xcode project:
   ```
   open "FreeReps.xcodeproj"
   ```

3. In **Signing & Capabilities**, select your development team and set a unique bundle identifier.

4. Verify the following capabilities are present (they should already be configured):
   - HealthKit (with Background Delivery)
   - Background Modes: Background processing, Background fetch
   - iCloud (Key-value storage)

5. Verify build settings point to the right files:
   - `INFOPLIST_FILE` = `Sources/FreeReps/Resources/Info.plist`
   - `CODE_SIGN_ENTITLEMENTS` = `Sources/FreeReps/Resources/FreeReps.entitlements`

6. Build and run on a physical device.

### Project structure

```
Sources/FreeReps/
  FreeRepsApp.swift              App entry point and background task registration
  ContentView.swift                Root TabView (Sync, Browse, Settings)
  Models/
    MySQLConfig.swift              Connection config (saved to UserDefaults)
    SyncState.swift                Observable sync state
    HealthDataType.swift           All HealthKit type descriptors
    HealthRecord.swift             Record models for the data browser
    ...
  Services/
    MySQLService.swift             TCP MySQL wire protocol (async/await actor)
    SchemaService.swift            CREATE TABLE DDL and schema initialization
    HealthKitService.swift         HealthKit queries and permissions
    SyncService.swift              Full/incremental sync orchestration
    BackgroundSyncManager.swift    HKObserverQuery registration and background delivery
    LocationService.swift          GPS tracking and geofence monitoring
    BackupManager.swift            Backup creation, listing, and restore
    iCloudSyncService.swift        iCloud KV store sync and device management
  ViewModels/                      View models for each tab
  Views/
    Sync/                          Sync dashboard and category status cards
    DataBrowser/                   Data browsing views per type
    Settings/                      All settings and configuration views
  Resources/
    Info.plist                     HealthKit usage description, BG task identifiers
    FreeReps.entitlements        HealthKit, iCloud, and location entitlements
Sources/FreeRepsWidgets/         Live Activity widget for sync progress
```

### MySQL wire protocol

The app implements the MySQL client wire protocol directly using `Network.framework`. This means no MySQL client library or SPM dependency is needed. It supports:

- Protocol v4.1 handshake
- `mysql_native_password` (SHA1) and `caching_sha2_password` (SHA256 + RSA full auth)
- Batch `INSERT IGNORE` in chunks of 500 for UUID-based deduplication
- Async/await with continuation-based receive buffering

## User guide

### Initial setup

1. Install the app on your iPhone.
2. Go to **Settings > MySQL Connection** and enter your MySQL server's host, port, database name, username, and password.
3. Tap **Test Connection** to verify connectivity.
4. Go to **Settings > HealthKit Permissions** and grant access to the health data types you want to sync.
5. Return to the **Sync** tab and tap **Full Sync** to backfill your historical data. **Keep the screen on until the full sync completes** — Apple HealthKit is not accessible when the device is locked, and the sync will stall. The app enables "Keep Screen On" by default during full sync (configurable in Settings).

### Database setup

Create a database on your MySQL server. The app will automatically create all required tables on first sync.

```sql
CREATE DATABASE freereps CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'freereps'@'%' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON freereps.* TO 'freereps'@'%';
FLUSH PRIVILEGES;
```

If you're using MySQL 8.0+ and get authentication errors, switch to the legacy auth plugin:

```sql
ALTER USER 'freereps'@'%' IDENTIFIED WITH mysql_native_password BY 'your_password';
```

### Ongoing sync

After the initial full sync, the app automatically syncs new data in the background:

- **Observer-based**: HealthKit notifies the app immediately when new data is recorded (requires the app to have run recently)
- **Scheduled**: A background processing task runs periodically (approximately every 15 minutes, subject to iOS scheduling)
- **Manual**: Pull down on the Sync tab to trigger an incremental sync

### Location tracking

Enable location tracking in **Settings > Location & Places** to log GPS coordinates to the `location_tracks` table. You can also set up geofences around places (home, office, gym, etc.) to log check-in and check-out events.

### Backups

Go to **Settings > Backup & Recovery** to create manual backups or enable automatic backups. Backups are stored in iCloud Drive and include all app configuration (MySQL settings, geofences, place categories, sync state). They do not include health data itself — that lives in HealthKit and your database.

### Multi-device

If you use the app on multiple devices, only one device performs automatic background syncs to avoid duplicate writes. Go to **Settings > iCloud Sync** to see registered devices and change which device is active.

## Known quirks and limitations

### HealthKit background access

HealthKit restricts data access when the device is locked. This affects background sync in important ways:

- **Screen on / app in foreground**: Full HealthKit access. All syncs work normally.
- **Screen off but recently unlocked**: HealthKit data remains accessible for a short window. Observer-based background syncs triggered by new health data (e.g., a workout ending) typically succeed because iOS delivers the notification while the device is still in this window.
- **Screen off for a long time / device locked**: HealthKit queries return authorization errors. The `BGProcessingTask` scheduled sync may wake the app, but if the device has been locked too long, HealthKit will deny access. The app handles this gracefully — it skips the sync and tries again next time.
- **Full sync requires screen on**: A full historical backfill takes a long time and needs continuous HealthKit access. The app has a "Keep Screen On" toggle in settings for this reason. If the screen locks mid-sync, the sync pauses and resumes when access is restored.

In practice, incremental syncs work well because they're triggered by HealthKit observer queries right when new data is recorded (device typically just used), and the data volume per sync is small.

### VPN and Tailscale

If your MySQL server is only reachable via VPN or Tailscale, be aware that iOS aggressively manages VPN connections:

- iOS may disconnect VPN tunnels in the background to save battery, especially on cellular.
- The "Connect On Demand" VPN setting helps but is not guaranteed — iOS can still tear down the tunnel when it decides the app doesn't need network access.
- Tailscale's iOS app uses the NEPacketTunnelProvider API, which is subject to the same iOS restrictions. The tunnel may go down during background sync windows.
- When the VPN is down, MySQL connections will fail with a timeout. The app will retry on the next sync cycle.

**Workaround**: If reliable background sync is important, expose your MySQL server on a network the phone can always reach (e.g., port-forwarded with TLS, or a cloud-hosted database). Alternatively, accept that some background syncs will be missed and rely on the next foreground sync to catch up.

### iOS background execution limits

- `BGProcessingTask` runs at iOS's discretion — typically when the device is charging and on Wi-Fi. The 15-minute interval is a request, not a guarantee.
- iOS may defer background tasks indefinitely if the device is low on battery or the app hasn't been used recently.
- After a force-quit by the user (swipe up in app switcher), background tasks and observer queries stop until the app is launched again.

## Database schema

The app creates these tables automatically:

| Table | Contents |
|-------|----------|
| `health_quantity_samples` | All quantity type measurements (steps, heart rate, etc.) |
| `health_category_samples` | Category type events (sleep, symptoms, etc.) |
| `health_workouts` | Workout sessions |
| `health_blood_pressure` | Blood pressure correlation pairs |
| `health_ecg` | ECG recordings with voltage data |
| `health_audiograms` | Hearing sensitivity measurements |
| `health_activity_summaries` | Daily activity ring data |
| `health_workout_routes` | GPS coordinates from workout routes |
| `health_medications` | Medication records |
| `location_tracks` | GPS location history |
| `location_geofence_events` | Geofence entry/exit events |

All tables use UUID-based primary keys for deduplication and `DATETIME(3)` for millisecond precision timestamps in UTC.

## License

This project is released under the MIT License. See [LICENSE](LICENSE) for details.
