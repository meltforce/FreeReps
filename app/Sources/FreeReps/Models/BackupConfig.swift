import Foundation

struct BackupConfig: Codable, Equatable {
    var autoBackupEnabled: Bool = false
    var autoBackupIntervalHours: Int = 24
    var maxBackupVersions: Int = 10

    static let intervalOptions = [6, 12, 24, 48, 72, 168]

    static let maxVersionOptions = [3, 5, 10, 15, 20, 30]

    private static let userDefaultsKey = "backupConfig_v1"

    static func load() -> BackupConfig {
        guard let data = UserDefaults.standard.data(forKey: userDefaultsKey),
              let config = try? JSONDecoder().decode(BackupConfig.self, from: data) else {
            return BackupConfig()
        }
        return config
    }

    func save() {
        if let data = try? JSONEncoder().encode(self) {
            UserDefaults.standard.set(data, forKey: BackupConfig.userDefaultsKey)
        }
        Task { @MainActor in iCloudSyncService.shared.pushBackupConfig(self) }
    }

    var intervalLabel: String {
        if autoBackupIntervalHours < 24 {
            return "Every \(autoBackupIntervalHours) hours"
        } else if autoBackupIntervalHours == 24 {
            return "Daily"
        } else if autoBackupIntervalHours % 24 == 0 {
            let days = autoBackupIntervalHours / 24
            return "Every \(days) days"
        } else {
            return "Every \(autoBackupIntervalHours) hours"
        }
    }
}
