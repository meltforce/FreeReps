import SwiftUI

struct SettingsView: View {
    let syncViewModel: SyncViewModel
    @StateObject private var vm = SettingsViewModel()
    @AppStorage("keepScreenOnDuringSync") private var keepScreenOnDuringSync = true
    @AppStorage("backgroundSyncEnabled") private var backgroundSyncEnabled = true

    var body: some View {
        NavigationStack {
            List {
                Section("Connection") {
                    NavigationLink {
                        FreeRepsSettingsView(vm: vm)
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("server.rack", color: .orange)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("FreeReps Connection")
                                    .font(.subheadline.weight(.semibold))
                                Text(verbatim: vm.config.host)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                            }
                        }
                    }

                    NavigationLink {
                        HealthPermissionsView(vm: vm)
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("heart.fill", color: .red)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Apple Health Permissions")
                                    .font(.subheadline.weight(.semibold))
                                Text(vm.permissionsRequested
                     ? (vm.deniedTypes.isEmpty ? "All permissions granted" : "\(vm.deniedTypes.count) permission(s) missing")
                     : "Tap to request permissions")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }

                Section("Sync") {
                    Toggle(isOn: $backgroundSyncEnabled) {
                        HStack(spacing: 12) {
                            iconBox("arrow.triangle.2.circlepath", color: backgroundSyncEnabled ? .blue : .secondary)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Background Sync")
                                    .font(.subheadline.weight(.semibold))
                                Text(backgroundSyncEnabled
                                    ? "Health data syncs automatically via FreeReps"
                                    : "No data is synced in the background")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }

                    Toggle(isOn: $keepScreenOnDuringSync) {
                        HStack(spacing: 12) {
                            iconBox("sun.max.fill", color: .yellow)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Keep Screen On")
                                    .font(.subheadline.weight(.semibold))
                                Text("Prevent display sleep during full sync")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }

                    NavigationLink {
                        SyncAdvancedView(vm: vm, syncViewModel: syncViewModel)
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("gearshape.fill", color: .gray)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Advanced")
                                    .font(.subheadline.weight(.semibold))
                                Text("Backfill settings, reset sync state")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }

                Section("About") {
                    Link(destination: URL(string: "https://github.com/meltforce/FreeReps/releases/tag/\(appVersion)")!) {
                        LabeledContent("App Version") {
                            HStack(spacing: 4) {
                                Text(appVersion)
                                Image(systemName: "arrow.up.right.square")
                                    .font(.caption2)
                            }
                            .foregroundStyle(.secondary)
                        }
                    }
                    if let serverVersion = vm.serverVersion {
                        Link(destination: URL(string: "https://github.com/meltforce/FreeReps/releases/tag/\(serverVersion)")!) {
                            LabeledContent("Server Version") {
                                HStack(spacing: 4) {
                                    Text(serverVersion)
                                    Image(systemName: "arrow.up.right.square")
                                        .font(.caption2)
                                }
                                .foregroundStyle(.secondary)
                            }
                        }
                    }
                    LabeledContent("HealthKit Types", value: "\(HealthDataTypes.allQuantityTypes.count + HealthDataTypes.allCategoryTypes.count)")
                    NavigationLink {
                        AcknowledgementsView()
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("doc.text.fill", color: .indigo)
                            Text("Acknowledgements")
                                .font(.subheadline.weight(.semibold))
                        }
                    }
                }

                BrandFooter()
            }
            .navigationTitle("Settings")
            .safeAreaInset(edge: .top) {
                if vm.config.testMode {
                    HStack {
                        Image(systemName: "wrench.and.screwdriver")
                        Text("Test Mode — \(vm.config.testHost)")
                            .font(.caption.weight(.semibold))
                    }
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 6)
                    .background(.orange.opacity(0.2))
                    .foregroundStyle(.orange)
                }
            }
            .onAppear { vm.refreshPermissionsState() }
            .onChange(of: vm.config) { vm.saveConfig() }
        }
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0"
    }

    private func iconBox(_ systemName: String, color: Color) -> some View {
        ZStack {
            RoundedRectangle(cornerRadius: 8)
                .fill(color)
                .frame(width: 36, height: 36)
            Image(systemName: systemName)
                .font(.system(size: 18))
                .foregroundStyle(.white)
        }
    }
}
