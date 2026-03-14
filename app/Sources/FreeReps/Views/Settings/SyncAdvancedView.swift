import SwiftUI

struct SyncAdvancedView: View {
    @ObservedObject var vm: SettingsViewModel
    let syncViewModel: SyncViewModel
    @State private var showResetSyncConfirmation = false

    var body: some View {
        List {
            Section {
                Picker(selection: Binding(
                    get: { vm.config.backfillYears ?? 0 },
                    set: { vm.config.backfillYears = $0 == 0 ? nil : $0 }
                )) {
                    Text("1 Year").tag(1)
                    Text("2 Years").tag(2)
                    Text("5 Years").tag(5)
                    Text("10 Years").tag(10)
                    Text("All Data").tag(0)
                } label: {
                    Text("Initial Backfill")
                }
            } footer: {
                Text("How far back to sync HealthKit data on first full sync. Subsequent syncs only check recent data.")
            }

            Section {
                Button(role: .destructive) {
                    showResetSyncConfirmation = true
                } label: {
                    HStack(spacing: 12) {
                        Image(systemName: "arrow.counterclockwise")
                            .foregroundStyle(.red)
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
        }
        .navigationTitle("Advanced")
        .navigationBarTitleDisplayMode(.inline)
        .onChange(of: vm.config) { vm.saveConfig() }
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
