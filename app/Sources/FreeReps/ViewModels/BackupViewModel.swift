import Foundation

@MainActor
class BackupViewModel: ObservableObject {
    @Published var config = BackupConfig.load()
    @Published var backups: [BackupMetadata] = []
    @Published var isCreatingBackup = false
    @Published var isRestoringBackup = false
    @Published var lastActionMessage: String?
    @Published var backupForSelectiveRestore: BackupMetadata?
    @Published var selectedRestoreKeys: Set<String> = []

    private let manager = BackupManager.shared

    init() {
        refreshBackups()

        NotificationCenter.default.addObserver(
            forName: .iCloudSettingsDidChange,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor [weak self] in
                self?.config = .load()
            }
        }
    }

    func refreshBackups() {
        backups = manager.listBackups()
    }

    func saveConfig() {
        config.save()
    }

    // MARK: - Manual Backup

    func createManualBackup() {
        isCreatingBackup = true
        lastActionMessage = nil

        if let metadata = manager.createBackup(trigger: .manual) {
            lastActionMessage = "Backup created successfully at \(Self.formatDate(metadata.createdAt))"
        } else {
            lastActionMessage = "Failed to create backup"
        }

        refreshBackups()
        isCreatingBackup = false
    }

    // MARK: - Restore

    func confirmRestore(_ backup: BackupMetadata) {
        let available = manager.availableCategories(id: backup.id)
        selectedRestoreKeys = Set(available.map(\.key))
        backupForSelectiveRestore = backup
    }

    func executeSelectiveRestore() {
        guard let backup = backupForSelectiveRestore else { return }
        isRestoringBackup = true
        lastActionMessage = nil

        let success = manager.restoreBackup(id: backup.id, keys: selectedRestoreKeys)

        if success {
            config = BackupConfig.load()
            let count = selectedRestoreKeys.count
            lastActionMessage = "Restored \(count) categor\(count == 1 ? "y" : "ies") from backup (\(Self.formatDate(backup.createdAt))). Restart the app for all changes to take effect."
        } else {
            lastActionMessage = "Failed to restore backup"
        }

        refreshBackups()
        isRestoringBackup = false
        backupForSelectiveRestore = nil
        selectedRestoreKeys = []
    }

    func availableCategories(for backup: BackupMetadata) -> [BackupCategory] {
        manager.availableCategories(id: backup.id)
    }

    // MARK: - Delete

    func deleteBackup(_ backup: BackupMetadata) {
        _ = manager.deleteBackup(id: backup.id)
        refreshBackups()
    }

    // MARK: - Helpers

    func backupSize(_ backup: BackupMetadata) -> String {
        guard let bytes = manager.backupSize(id: backup.id) else { return "Unknown" }
        return ByteCountFormatter.string(fromByteCount: bytes, countStyle: .file)
    }

    func backupContents(_ backup: BackupMetadata) -> [String]? {
        manager.backupContents(id: backup.id)
    }

    static func formatDate(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .short
        return formatter.string(from: date)
    }

    static func relativeDate(_ date: Date) -> String {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .abbreviated
        return formatter.localizedString(for: date, relativeTo: Date())
    }

    var isUsingiCloud: Bool { manager.isUsingiCloud }

    var totalBackupCount: Int { backups.count }

    var latestBackupDate: Date? { backups.first?.createdAt }
}
