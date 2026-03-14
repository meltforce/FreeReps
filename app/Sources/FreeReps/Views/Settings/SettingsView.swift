import SwiftUI

struct SettingsView: View {
    let syncViewModel: SyncViewModel
    @StateObject private var vm = SettingsViewModel()
    @AppStorage("keepScreenOnDuringSync") private var keepScreenOnDuringSync = true
    @AppStorage("backgroundSyncEnabled") private var backgroundSyncEnabled = true
    @State private var showResetSyncConfirmation = false

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
                                Text(verbatim: "\(vm.config.host):\(vm.config.port)")
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
                        DataValidationView(syncViewModel: syncViewModel)
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("checkmark.shield.fill", color: .green)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Data Validation")
                                    .font(.subheadline.weight(.semibold))
                                Text("Compare Apple Health vs FreeReps")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }

                    Button(role: .destructive) {
                        showResetSyncConfirmation = true
                    } label: {
                        HStack(spacing: 12) {
                            iconBox("arrow.counterclockwise", color: .red)
                            VStack(alignment: .leading, spacing: 2) {
                                Text("Reset Sync State")
                                    .font(.subheadline.weight(.semibold))
                                Text("Clears all sync progress. Next sync will re-send all data.")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .disabled(syncViewModel.isAnySyncRunning)
                }

                Section("About") {
                    LabeledContent("Version", value: appVersion)
                    LabeledContent("Backend", value: "FreeReps HTTP API")
                    LabeledContent("HealthKit Types", value: "\(HealthDataTypes.allQuantityTypes.count + HealthDataTypes.allCategoryTypes.count)")
                }

                BrandFooter()
            }
            .navigationTitle("Settings")
            .onAppear { vm.refreshPermissionsState() }
            .alert("Reset Sync State", isPresented: $showResetSyncConfirmation) {
                Button("Reset", role: .destructive) {
                    syncViewModel.resetAllSyncState()
                }
                Button("Cancel", role: .cancel) { }
            } message: {
                Text("This will clear all sync progress and cursors. The next sync will re-send all health data to FreeReps. Server-side data is not affected.")
            }
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
