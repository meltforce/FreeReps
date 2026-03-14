import Foundation
import UIKit

struct iCloudDevice: Codable, Identifiable {
    let id: String       // stable UUID
    let name: String     // UIDevice.current.name
    let model: String    // "iPhone" / "iPad"
    var lastSeen: Date
}

@MainActor
final class iCloudSyncService: ObservableObject {

    static let shared = iCloudSyncService()

    // MARK: - Published state

    @Published private(set) var iCloudSyncEnabled: Bool = true
    @Published private(set) var activeAutoSyncDeviceID: String?
    @Published private(set) var registeredDevices: [iCloudDevice] = []

    // MARK: - Device identity

    let currentDeviceID: String

    // MARK: - KV store keys

    private enum KVKey {
        static let enabled        = "icloud_enabled"
        static let activeDevice   = "icloud_active_device_id"
        static let devices        = "icloud_devices"
        static let freerepsConfig = "freereps_config_v1"
        static let syncSnapshot   = "sync_snapshot_v1"
        static let backupConfig   = "backup_config_v1"
    }

    // Local UserDefaults keys (must match the keys used in each model file)
    private enum UDKey {
        static let freerepsConfig  = "freerepsConfig_v1"
        static let syncSnapshot    = "com.freereps.syncSnapshot"
        static let backupConfig    = "backupConfig_v1"
    }

    private static let deviceIDKey = "icloud_local_device_id"

    private var kv: NSUbiquitousKeyValueStore { .default }

    private init() {
        if let existing = UserDefaults.standard.string(forKey: iCloudSyncService.deviceIDKey) {
            currentDeviceID = existing
        } else {
            let newID = UUID().uuidString
            UserDefaults.standard.set(newID, forKey: iCloudSyncService.deviceIDKey)
            currentDeviceID = newID
        }
    }

    // MARK: - Computed

    var isCurrentDeviceActiveForAutoSync: Bool {
        guard iCloudSyncEnabled else { return true }
        guard let active = activeAutoSyncDeviceID else { return true }
        return active == currentDeviceID
    }

    var activeDeviceName: String? {
        guard let id = activeAutoSyncDeviceID else { return nil }
        return registeredDevices.first(where: { $0.id == id })?.name
    }

    // MARK: - Lifecycle

    func start() {
        NotificationCenter.default.addObserver(
            forName: NSUbiquitousKeyValueStore.didChangeExternallyNotification,
            object: kv,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor [weak self] in
                self?.loadPublishedState()
                self?.registerDevice()
                self?.pullAllToUserDefaults()
                NotificationCenter.default.post(name: .iCloudSettingsDidChange, object: nil)
            }
        }

        kv.synchronize()
        loadPublishedState()
        registerDevice()

        // First device to launch claims auto-sync
        if activeAutoSyncDeviceID == nil && iCloudSyncEnabled {
            claimAutoSync()
        }

        pullAllToUserDefaults()
        NotificationCenter.default.post(name: .iCloudSettingsDidChange, object: nil)
    }

    // MARK: - Published state loading

    private func loadPublishedState() {
        // Default is enabled; only set to false if the key explicitly exists and is false
        if kv.object(forKey: KVKey.enabled) != nil {
            iCloudSyncEnabled = kv.bool(forKey: KVKey.enabled)
        }
        activeAutoSyncDeviceID = kv.string(forKey: KVKey.activeDevice)
        registeredDevices = decodeDevices()
    }

    // MARK: - Device registration

    private func registerDevice() {
        var devices = decodeDevices()
        let device = iCloudDevice(
            id: currentDeviceID,
            name: UIDevice.current.name,
            model: UIDevice.current.model,
            lastSeen: Date()
        )
        if let idx = devices.firstIndex(where: { $0.id == currentDeviceID }) {
            devices[idx] = device
        } else {
            devices.append(device)
        }
        encodeAndSetDevices(devices)
        registeredDevices = devices
        kv.synchronize()
    }

    private func decodeDevices() -> [iCloudDevice] {
        guard let data = kv.data(forKey: KVKey.devices) else { return [] }
        return (try? JSONDecoder().decode([iCloudDevice].self, from: data)) ?? []
    }

    private func encodeAndSetDevices(_ devices: [iCloudDevice]) {
        if let data = try? JSONEncoder().encode(devices) {
            kv.set(data, forKey: KVKey.devices)
        }
    }

    // MARK: - Auto-sync claiming

    func claimAutoSync() {
        activeAutoSyncDeviceID = currentDeviceID
        kv.set(currentDeviceID, forKey: KVKey.activeDevice)
        kv.synchronize()
    }

    func releaseAutoSync() {
        activeAutoSyncDeviceID = nil
        kv.removeObject(forKey: KVKey.activeDevice)
        kv.synchronize()
    }

    // MARK: - iCloud sync toggle

    func setEnabled(_ enabled: Bool) {
        iCloudSyncEnabled = enabled
        kv.set(enabled, forKey: KVKey.enabled)
        kv.synchronize()
    }

    // MARK: - Push methods (called from model save() sites)

    func pushFreeRepsConfig(_ config: FreeRepsConfig) {
        guard iCloudSyncEnabled else { return }
        if let data = try? JSONEncoder().encode(config) {
            kv.set(data, forKey: KVKey.freerepsConfig)
            kv.synchronize()
        }
    }

    func pushSyncSnapshot(_ snapshot: PersistedSnapshot) {
        guard iCloudSyncEnabled else { return }
        if let data = try? JSONEncoder().encode(snapshot) {
            kv.set(data, forKey: KVKey.syncSnapshot)
            kv.synchronize()
        }
    }

    func pushBackupConfig(_ config: BackupConfig) {
        guard iCloudSyncEnabled else { return }
        if let data = try? JSONEncoder().encode(config) {
            kv.set(data, forKey: KVKey.backupConfig)
            kv.synchronize()
        }
    }

    // MARK: - Pull snapshot with merge (called from SyncState.restore())

    func pullSyncSnapshot() {
        guard let remoteData = kv.data(forKey: KVKey.syncSnapshot),
              let remote = try? JSONDecoder().decode(PersistedSnapshot.self, from: remoteData)
        else { return }

        if let localData = UserDefaults.standard.data(forKey: UDKey.syncSnapshot),
           let local = try? JSONDecoder().decode(PersistedSnapshot.self, from: localData) {
            let merged = mergeSnapshots(local: local, remote: remote)
            if let data = try? JSONEncoder().encode(merged) {
                UserDefaults.standard.set(data, forKey: UDKey.syncSnapshot)
            }
        } else {
            UserDefaults.standard.set(remoteData, forKey: UDKey.syncSnapshot)
        }
    }

    private func mergeSnapshots(local: PersistedSnapshot, remote: PersistedSnapshot) -> PersistedSnapshot {
        let mergedLastSync: Date? = maxDate(local.lastSyncDate, remote.lastSyncDate)
        let mergedTotal: Int? = maxOptional(local.totalRecords, remote.totalRecords)
        let mergedFullSync = (local.hasCompletedFullSync ?? false) || (remote.hasCompletedFullSync ?? false)

        var categoryMap: [String: PersistedCategory] = [:]
        for cat in local.categories { categoryMap[cat.id] = cat }
        for remoteCat in remote.categories {
            if let localCat = categoryMap[remoteCat.id] {
                categoryMap[remoteCat.id] = PersistedCategory(
                    id: remoteCat.id,
                    recordCount: max(localCat.recordCount, remoteCat.recordCount),
                    lastSyncDate: maxDate(localCat.lastSyncDate, remoteCat.lastSyncDate),
                    completed: localCat.completed || remoteCat.completed
                )
            } else {
                categoryMap[remoteCat.id] = remoteCat
            }
        }

        // Prefer local cursor state — backfill progress is device-specific and should
        // not be discarded when iCloud delivers remote changes from another session.
        // Fall back to remote if local has none (e.g. fresh install restoring from iCloud).
        let mergedCursors = local.backfillCursors ?? remote.backfillCursors
        let mergedAnchor = local.backfillAnchorDate ?? remote.backfillAnchorDate

        return PersistedSnapshot(
            lastSyncDate: mergedLastSync,
            categories: Array(categoryMap.values),
            totalRecords: mergedTotal,
            hasCompletedFullSync: mergedFullSync,
            backfillCursors: mergedCursors,
            backfillAnchorDate: mergedAnchor
        )
    }

    private func maxDate(_ a: Date?, _ b: Date?) -> Date? {
        switch (a, b) {
        case (.none, let d), (let d, .none): return d
        case (.some(let l), .some(let r)): return max(l, r)
        }
    }

    private func maxOptional(_ a: Int?, _ b: Int?) -> Int? {
        switch (a, b) {
        case (.none, let n), (let n, .none): return n
        case (.some(let l), .some(let r)): return max(l, r)
        }
    }

    // MARK: - Pull all remote values into UserDefaults

    private func pullAllToUserDefaults() {
        if let data = kv.data(forKey: KVKey.freerepsConfig) {
            UserDefaults.standard.set(data, forKey: UDKey.freerepsConfig)
        }
        pullSyncSnapshot()
        if let data = kv.data(forKey: KVKey.backupConfig) {
            UserDefaults.standard.set(data, forKey: UDKey.backupConfig)
        }
    }
}

extension Notification.Name {
    static let iCloudSettingsDidChange = Notification.Name("iCloudSettingsDidChange")
}
