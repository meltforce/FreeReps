import SwiftUI

struct iCloudSyncSettingsView: View {
    @ObservedObject private var service = iCloudSyncService.shared

    var body: some View {
        List {
            Section {
                Toggle("iCloud Sync", isOn: Binding(
                    get: { service.iCloudSyncEnabled },
                    set: { service.setEnabled($0) }
                ))
            } footer: {
                Text("Sync MySQL connection, sync history, and all other settings across your devices signed into iCloud.")
            }

            if service.iCloudSyncEnabled {
                Section {
                    let sorted = service.registeredDevices.sorted { $0.lastSeen > $1.lastSeen }
                    ForEach(sorted) { device in
                        DeviceRow(
                            device: device,
                            isActive: service.activeAutoSyncDeviceID == device.id,
                            isCurrent: service.currentDeviceID == device.id
                        )
                    }

                    if service.activeAutoSyncDeviceID != service.currentDeviceID {
                        Button("Make This Device Active") {
                            service.claimAutoSync()
                        }
                    }
                } header: {
                    Text("Auto-Sync Device")
                } footer: {
                    Text("Only one device can automatically sync to MySQL to avoid conflicts. Manual syncs are available on all devices.")
                }
            }
        }
        .navigationTitle("iCloud Sync")
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct DeviceRow: View {
    let device: iCloudDevice
    let isActive: Bool
    let isCurrent: Bool

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 4) {
                    Text(device.name)
                        .font(.subheadline.weight(.medium))
                    if isCurrent {
                        Text("(This Device)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                Text("Last seen \(device.lastSeen.formatted(.relative(presentation: .named)))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            if isActive {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.green)
            }
        }
    }
}
