import Foundation

struct BackupCategory: Identifiable {
    let key: String
    let displayName: String
    let icon: String
    let colorName: String

    var id: String { key }

    static let all: [BackupCategory] = [
        BackupCategory(key: "freerepsConfig_v1", displayName: "FreeReps Connection", icon: "server.rack", colorName: "orange"),
        BackupCategory(key: "com.freereps.syncSnapshot", displayName: "Sync Statuses", icon: "arrow.triangle.2.circlepath", colorName: "teal"),
        BackupCategory(key: "hk_permissions_requested", displayName: "Health Permissions", icon: "heart.fill", colorName: "red"),
        BackupCategory(key: "backupConfig_v1", displayName: "Backup Settings", icon: "gearshape.fill", colorName: "gray"),
    ]
}

struct BackupMetadata: Codable, Identifiable {
    let id: UUID
    let createdAt: Date
    let trigger: BackupTrigger
    let appVersion: String
    let dataKeysCount: Int

    enum BackupTrigger: String, Codable {
        case manual
        case automatic
        case preRestore
    }
}

struct BackupBundle: Codable {
    let metadata: BackupMetadata
    let payload: [String: Data]
}

final class BackupManager {
    static let shared = BackupManager()

    /// All UserDefaults keys the app uses for persistent data.
    private let backupKeys: [String] = [
        "freerepsConfig_v1",
        "com.freereps.syncSnapshot",
        "hk_permissions_requested",
        "backupConfig_v1",
    ]

    private let fileManager = FileManager.default

    private init() {}

    // MARK: - Directory

    /// Uses the iCloud ubiquity container for backup storage, with a local fallback
    /// if iCloud is unavailable (e.g. not signed in, or no entitlement).
    private var backupsDirectory: URL {
        let dir: URL
        if let iCloudURL = fileManager.url(forUbiquityContainerIdentifier: nil) {
            dir = iCloudURL
                .appendingPathComponent("Documents", isDirectory: true)
                .appendingPathComponent("Backups", isDirectory: true)
        } else {
            // Fallback to local Documents if iCloud is unavailable
            let docs = fileManager.urls(for: .documentDirectory, in: .userDomainMask).first!
            dir = docs.appendingPathComponent("Backups", isDirectory: true)
        }
        if !fileManager.fileExists(atPath: dir.path) {
            try? fileManager.createDirectory(at: dir, withIntermediateDirectories: true)
        }
        return dir
    }

    /// Whether backups are currently stored in iCloud.
    var isUsingiCloud: Bool {
        fileManager.url(forUbiquityContainerIdentifier: nil) != nil
    }

    // MARK: - Create Backup

    @discardableResult
    func createBackup(trigger: BackupMetadata.BackupTrigger = .manual) -> BackupMetadata? {
        var payload: [String: Data] = [:]
        let defaults = UserDefaults.standard

        for key in backupKeys {
            if let data = defaults.data(forKey: key) {
                payload[key] = data
            } else if let boolValue = defaults.object(forKey: key) as? Bool {
                // Handle non-Data values like booleans
                payload[key] = try? JSONEncoder().encode(boolValue)
            }
        }

        let appVersion = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0"
        let metadata = BackupMetadata(
            id: UUID(),
            createdAt: Date(),
            trigger: trigger,
            appVersion: appVersion,
            dataKeysCount: payload.count
        )

        let bundle = BackupBundle(metadata: metadata, payload: payload)

        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        encoder.outputFormatting = .prettyPrinted

        guard let data = try? encoder.encode(bundle) else { return nil }

        let filename = backupFilename(for: metadata)
        let fileURL = backupsDirectory.appendingPathComponent(filename)

        do {
            try data.write(to: fileURL, options: .atomic)
            pruneOldBackups()
            return metadata
        } catch {
            return nil
        }
    }

    // MARK: - List Backups

    func listBackups() -> [BackupMetadata] {
        guard let files = try? fileManager.contentsOfDirectory(
            at: backupsDirectory,
            includingPropertiesForKeys: [.creationDateKey],
            options: .skipsHiddenFiles
        ) else { return [] }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601

        var backups: [BackupMetadata] = []
        for file in files where file.pathExtension == "json" {
            guard let data = try? Data(contentsOf: file),
                  let bundle = try? decoder.decode(BackupBundle.self, from: data) else { continue }
            backups.append(bundle.metadata)
        }

        return backups.sorted { $0.createdAt > $1.createdAt }
    }

    // MARK: - Restore Backup

    func restoreBackup(id: UUID, createSafetyBackup: Bool = true) -> Bool {
        let allKeys = Set(backupKeys)
        return restoreBackup(id: id, keys: allKeys, createSafetyBackup: createSafetyBackup)
    }

    func restoreBackup(id: UUID, keys: Set<String>, createSafetyBackup: Bool = true) -> Bool {
        // Create a safety backup of current state before restoring
        if createSafetyBackup {
            createBackup(trigger: .preRestore)
        }

        guard let bundle = loadBundle(id: id) else { return false }

        let defaults = UserDefaults.standard

        for (key, data) in bundle.payload where keys.contains(key) {
            if key == "hk_permissions_requested" {
                // Decode boolean values
                if let value = try? JSONDecoder().decode(Bool.self, from: data) {
                    defaults.set(value, forKey: key)
                }
            } else {
                defaults.set(data, forKey: key)
            }
        }

        defaults.synchronize()
        return true
    }

    // MARK: - Available Categories

    func availableCategories(id: UUID) -> [BackupCategory] {
        guard let bundle = loadBundle(id: id) else { return [] }
        let presentKeys = Set(bundle.payload.keys)
        return BackupCategory.all.filter { presentKeys.contains($0.key) }
    }

    // MARK: - Delete Backup

    @discardableResult
    func deleteBackup(id: UUID) -> Bool {
        guard let url = findBackupFile(id: id) else { return false }
        do {
            try fileManager.removeItem(at: url)
            return true
        } catch {
            return false
        }
    }

    // MARK: - Backup Details

    func backupSize(id: UUID) -> Int64? {
        guard let url = findBackupFile(id: id),
              let attrs = try? fileManager.attributesOfItem(atPath: url.path),
              let size = attrs[.size] as? Int64 else { return nil }
        return size
    }

    func backupContents(id: UUID) -> [String]? {
        guard let bundle = loadBundle(id: id) else { return nil }
        return Array(bundle.payload.keys).sorted()
    }

    // MARK: - Auto Backup Check

    /// Returns true if an automatic backup is due based on the config and last backup date.
    func isAutoBackupDue() -> Bool {
        let config = BackupConfig.load()
        guard config.autoBackupEnabled else { return false }

        let backups = listBackups()
        let lastAuto = backups.first { $0.trigger == .automatic || $0.trigger == .manual }

        guard let lastDate = lastAuto?.createdAt else {
            // No backups exist yet, one is due
            return true
        }

        let intervalSeconds = TimeInterval(config.autoBackupIntervalHours * 3600)
        return Date().timeIntervalSince(lastDate) >= intervalSeconds
    }

    /// Runs an automatic backup if one is due and this device is the active auto-sync device.
    @MainActor @discardableResult
    func runAutoBackupIfNeeded() -> BackupMetadata? {
        guard iCloudSyncService.shared.isCurrentDeviceActiveForAutoSync else { return nil }
        guard isAutoBackupDue() else { return nil }
        return createBackup(trigger: .automatic)
    }

    // MARK: - Private Helpers

    private func backupFilename(for metadata: BackupMetadata) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "yyyy-MM-dd_HH-mm-ss"
        let dateStr = formatter.string(from: metadata.createdAt)
        return "backup_\(dateStr)_\(metadata.id.uuidString.prefix(8)).json"
    }

    private func findBackupFile(id: UUID) -> URL? {
        guard let files = try? fileManager.contentsOfDirectory(
            at: backupsDirectory,
            includingPropertiesForKeys: nil,
            options: .skipsHiddenFiles
        ) else { return nil }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601

        for file in files where file.pathExtension == "json" {
            guard let data = try? Data(contentsOf: file),
                  let bundle = try? decoder.decode(BackupBundle.self, from: data),
                  bundle.metadata.id == id else { continue }
            return file
        }
        return nil
    }

    private func loadBundle(id: UUID) -> BackupBundle? {
        guard let url = findBackupFile(id: id),
              let data = try? Data(contentsOf: url) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(BackupBundle.self, from: data)
    }

    private func pruneOldBackups() {
        let config = BackupConfig.load()
        let maxVersions = config.maxBackupVersions
        var backups = listBackups()

        while backups.count > maxVersions {
            let oldest = backups.removeLast()
            deleteBackup(id: oldest.id)
        }
    }
}
