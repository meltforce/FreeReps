import SwiftUI

struct BackupSettingsView: View {
    @StateObject private var vm = BackupViewModel()
    @ObservedObject private var iCloud = iCloudSyncService.shared

    var body: some View {
        List {
            // MARK: - Storage Location
            Section {
                HStack {
                    Label {
                        Text(vm.isUsingiCloud ? "iCloud Drive" : "Local Storage")
                    } icon: {
                        Image(systemName: vm.isUsingiCloud ? "icloud.fill" : "iphone")
                            .foregroundStyle(vm.isUsingiCloud ? .blue : .secondary)
                    }
                    Spacer()
                    if vm.isUsingiCloud {
                        Text("Connected")
                            .font(.caption)
                            .foregroundStyle(.green)
                    } else {
                        Text("iCloud unavailable")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            } header: {
                Text("Storage")
            } footer: {
                Text(vm.isUsingiCloud
                    ? "Backups are stored in iCloud Drive and sync across your devices."
                    : "Sign in to iCloud in Settings to enable cloud backup. Backups are currently stored on this device only.")
            }

            // MARK: - Manual Backup
            Section {
                Button {
                    vm.createManualBackup()
                } label: {
                    HStack {
                        Label("Create Backup Now", systemImage: "arrow.down.doc.fill")
                        Spacer()
                        if vm.isCreatingBackup {
                            ProgressView()
                        }
                    }
                }
                .disabled(vm.isCreatingBackup || vm.isRestoringBackup)
            } header: {
                Text("Manual Backup")
            } footer: {
                Text("Creates a snapshot of all app settings, locations, geo-fences, sync statuses, and database configuration.")
            }

            // MARK: - Status Banner
            if let message = vm.lastActionMessage {
                Section {
                    Label(message, systemImage: message.contains("Failed") ? "exclamationmark.triangle.fill" : "checkmark.circle.fill")
                        .font(.subheadline)
                        .foregroundStyle(message.contains("Failed") ? .red : .green)
                }
            }

            // MARK: - Automatic Backup
            Section {
                if !iCloud.isCurrentDeviceActiveForAutoSync {
                    HStack(spacing: 12) {
                        Image(systemName: "externaldrive.badge.xmark")
                            .foregroundStyle(.orange)
                            .frame(width: 24)
                        VStack(alignment: .leading, spacing: 3) {
                            Text("Auto Backup Inactive on This Device")
                                .font(.subheadline.weight(.semibold))
                            Text("Automatic backups only run on \(iCloud.activeDeviceName ?? "the active device"). Go to iCloud Sync settings to make this device active.")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                    .padding(.vertical, 4)
                }

                Toggle("Enable Automatic Backups", isOn: $vm.config.autoBackupEnabled)
                    .onChange(of: vm.config.autoBackupEnabled) { vm.saveConfig() }
                    .disabled(!iCloud.isCurrentDeviceActiveForAutoSync)

                if vm.config.autoBackupEnabled {
                    Picker("Backup Frequency", selection: $vm.config.autoBackupIntervalHours) {
                        ForEach(BackupConfig.intervalOptions, id: \.self) { hours in
                            Text(intervalLabel(hours)).tag(hours)
                        }
                    }
                    .onChange(of: vm.config.autoBackupIntervalHours) { vm.saveConfig() }
                    .disabled(!iCloud.isCurrentDeviceActiveForAutoSync)
                }

                Picker("Keep Last", selection: $vm.config.maxBackupVersions) {
                    ForEach(BackupConfig.maxVersionOptions, id: \.self) { count in
                        Text("\(count) backups").tag(count)
                    }
                }
                .onChange(of: vm.config.maxBackupVersions) { vm.saveConfig() }
                .disabled(!iCloud.isCurrentDeviceActiveForAutoSync)
            } header: {
                Text("Automatic Backup")
            } footer: {
                if !iCloud.isCurrentDeviceActiveForAutoSync {
                    EmptyView()
                } else if vm.config.autoBackupEnabled {
                    Text("Backups are created automatically when the app enters the background or during background sync. Older backups beyond the limit are removed automatically.")
                } else {
                    Text("When enabled, backups are created automatically at the selected interval.")
                }
            }

            // MARK: - Backup Versions
            Section {
                if vm.backups.isEmpty {
                    HStack {
                        Spacer()
                        VStack(spacing: 8) {
                            Image(systemName: "tray")
                                .font(.title2)
                                .foregroundStyle(.secondary)
                            Text("No backups yet")
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                        .padding(.vertical, 12)
                        Spacer()
                    }
                } else {
                    ForEach(vm.backups) { backup in
                        BackupRow(backup: backup, vm: vm)
                    }
                    .onDelete { indexSet in
                        for index in indexSet {
                            vm.deleteBackup(vm.backups[index])
                        }
                    }
                }
            } header: {
                HStack {
                    Text("Backup Versions (\(vm.totalBackupCount))")
                    Spacer()
                    if let latest = vm.latestBackupDate {
                        Text("Latest: \(BackupViewModel.relativeDate(latest))")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            } footer: {
                if !vm.backups.isEmpty {
                    Text("Swipe left to delete a backup. Tap to restore from that version.")
                }
            }

            // MARK: - What's Included
            Section("What's Included in Backups") {
                includedItem("cylinder.fill", "MySQL connection settings", .orange)
                includedItem("location.fill", "Location configuration", .blue)
                includedItem("mappin.and.ellipse", "Geo-fences & places", .green)
                includedItem("tag.fill", "Place categories", .purple)
                includedItem("arrow.triangle.2.circlepath", "Synchronization statuses", .teal)
                includedItem("heart.fill", "Health permission flags", .red)
                includedItem("gearshape.fill", "Backup settings", .gray)
            }

            Section {
                VStack(alignment: .leading, spacing: 8) {
                    Label("Note", systemImage: "info.circle")
                        .font(.subheadline.weight(.semibold))
                    Text("Backups include all app configuration and metadata. Actual health data from Apple Health is not included as it remains in the Health app and your MySQL database.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 4)
            }
        }
        .navigationTitle("Backup & Recovery")
        .sheet(item: $vm.backupForSelectiveRestore) { backup in
            BackupRestoreSheet(backup: backup, vm: vm)
        }
        .onAppear { vm.refreshBackups() }
    }

    // MARK: - Subviews

    private func includedItem(_ icon: String, _ label: String, _ color: Color) -> some View {
        HStack(spacing: 12) {
            Image(systemName: icon)
                .font(.system(size: 14))
                .foregroundStyle(color)
                .frame(width: 24)
            Text(label)
                .font(.subheadline)
        }
    }

    private func intervalLabel(_ hours: Int) -> String {
        switch hours {
        case let h where h < 24: return "Every \(h) hours"
        case 24: return "Daily"
        case 168: return "Weekly"
        default: return "Every \(hours / 24) days"
        }
    }
}

// MARK: - BackupRow

private struct BackupRow: View {
    let backup: BackupMetadata
    @ObservedObject var vm: BackupViewModel

    var body: some View {
        Button {
            vm.confirmRestore(backup)
        } label: {
            HStack(spacing: 12) {
                triggerIcon
                    .frame(width: 32, height: 32)

                VStack(alignment: .leading, spacing: 3) {
                    Text(BackupViewModel.formatDate(backup.createdAt))
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(.primary)

                    HStack(spacing: 8) {
                        Text(triggerLabel)
                            .font(.caption2)
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(triggerColor.opacity(0.15))
                            .foregroundStyle(triggerColor)
                            .clipShape(Capsule())

                        Text("\(backup.dataKeysCount) items")
                            .font(.caption)
                            .foregroundStyle(.secondary)

                        Text(vm.backupSize(backup))
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }

                Spacer()

                Image(systemName: "arrow.counterclockwise")
                    .font(.system(size: 14))
                    .foregroundStyle(.blue)
            }
            .contentShape(Rectangle())
        }
        .disabled(vm.isRestoringBackup)
    }

    private var triggerLabel: String {
        switch backup.trigger {
        case .manual: return "Manual"
        case .automatic: return "Auto"
        case .preRestore: return "Safety"
        }
    }

    private var triggerColor: Color {
        switch backup.trigger {
        case .manual: return .blue
        case .automatic: return .green
        case .preRestore: return .orange
        }
    }

    @ViewBuilder
    private var triggerIcon: some View {
        ZStack {
            Circle()
                .fill(triggerColor.opacity(0.15))
            Image(systemName: iconName)
                .font(.system(size: 14))
                .foregroundStyle(triggerColor)
        }
    }

    private var iconName: String {
        switch backup.trigger {
        case .manual: return "hand.tap.fill"
        case .automatic: return "clock.arrow.circlepath"
        case .preRestore: return "shield.fill"
        }
    }
}

// MARK: - BackupRestoreSheet

private struct BackupRestoreSheet: View {
    let backup: BackupMetadata
    @ObservedObject var vm: BackupViewModel
    @Environment(\.dismiss) private var dismiss

    private var availableCategories: [BackupCategory] {
        vm.availableCategories(for: backup)
    }

    private var availableKeys: Set<String> {
        Set(availableCategories.map(\.key))
    }

    private var allSelected: Bool {
        availableKeys.isSubset(of: vm.selectedRestoreKeys)
    }

    var body: some View {
        NavigationStack {
            List {
                // MARK: Backup Info
                Section {
                    HStack(spacing: 12) {
                        triggerIcon
                            .frame(width: 40, height: 40)

                        VStack(alignment: .leading, spacing: 4) {
                            Text(BackupViewModel.formatDate(backup.createdAt))
                                .font(.headline)
                            HStack(spacing: 8) {
                                Text(triggerLabel)
                                    .font(.caption2)
                                    .padding(.horizontal, 6)
                                    .padding(.vertical, 2)
                                    .background(triggerColor.opacity(0.15))
                                    .foregroundStyle(triggerColor)
                                    .clipShape(Capsule())
                                Text("\(backup.dataKeysCount) items")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Text(vm.backupSize(backup))
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .padding(.vertical, 4)
                }

                // MARK: Category Selection
                Section {
                    ForEach(BackupCategory.all) { category in
                        let isAvailable = availableKeys.contains(category.key)
                        let isSelected = vm.selectedRestoreKeys.contains(category.key)

                        Button {
                            if isAvailable {
                                if isSelected {
                                    vm.selectedRestoreKeys.remove(category.key)
                                } else {
                                    vm.selectedRestoreKeys.insert(category.key)
                                }
                            }
                        } label: {
                            HStack(spacing: 12) {
                                Image(systemName: category.icon)
                                    .font(.system(size: 14))
                                    .foregroundStyle(isAvailable ? categoryColor(category.colorName) : .gray)
                                    .frame(width: 24)
                                Text(category.displayName)
                                    .font(.subheadline)
                                    .foregroundStyle(isAvailable ? .primary : .secondary)
                                Spacer()
                                if isAvailable {
                                    Image(systemName: isSelected ? "checkmark.circle.fill" : "circle")
                                        .foregroundStyle(isSelected ? .blue : .secondary)
                                } else {
                                    Text("Not in backup")
                                        .font(.caption2)
                                        .foregroundStyle(.secondary)
                                }
                            }
                        }
                        .disabled(!isAvailable)
                    }
                } header: {
                    HStack {
                        Text("Categories to Restore")
                        Spacer()
                        Button(allSelected ? "Deselect All" : "Select All") {
                            if allSelected {
                                vm.selectedRestoreKeys.subtract(availableKeys)
                            } else {
                                vm.selectedRestoreKeys.formUnion(availableKeys)
                            }
                        }
                        .font(.caption)
                    }
                } footer: {
                    Text("A safety backup of your current settings will be created before restoring.")
                }

                // MARK: Restore Button
                Section {
                    Button {
                        vm.executeSelectiveRestore()
                        dismiss()
                    } label: {
                        HStack {
                            Spacer()
                            let count = vm.selectedRestoreKeys.intersection(availableKeys).count
                            Text("Restore \(count) Categor\(count == 1 ? "y" : "ies")")
                                .font(.headline)
                            Spacer()
                        }
                    }
                    .disabled(vm.selectedRestoreKeys.intersection(availableKeys).isEmpty || vm.isRestoringBackup)
                }
            }
            .navigationTitle("Restore Backup")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        vm.backupForSelectiveRestore = nil
                        dismiss()
                    }
                }
            }
        }
    }

    private func categoryColor(_ name: String) -> Color {
        switch name {
        case "orange": return .orange
        case "blue": return .blue
        case "green": return .green
        case "purple": return .purple
        case "teal": return .teal
        case "red": return .red
        case "gray": return .gray
        default: return .primary
        }
    }

    private var triggerLabel: String {
        switch backup.trigger {
        case .manual: return "Manual"
        case .automatic: return "Auto"
        case .preRestore: return "Safety"
        }
    }

    private var triggerColor: Color {
        switch backup.trigger {
        case .manual: return .blue
        case .automatic: return .green
        case .preRestore: return .orange
        }
    }

    @ViewBuilder
    private var triggerIcon: some View {
        ZStack {
            Circle()
                .fill(triggerColor.opacity(0.15))
            Image(systemName: iconName)
                .font(.system(size: 16))
                .foregroundStyle(triggerColor)
        }
    }

    private var iconName: String {
        switch backup.trigger {
        case .manual: return "hand.tap.fill"
        case .automatic: return "clock.arrow.circlepath"
        case .preRestore: return "shield.fill"
        }
    }
}
