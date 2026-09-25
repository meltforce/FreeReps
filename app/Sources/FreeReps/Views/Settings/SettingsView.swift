import SwiftUI

struct SettingsView: View {
    let syncViewModel: SyncViewModel
    @StateObject private var vm = SettingsViewModel()
    @AppStorage("keepScreenOnDuringSync") private var keepScreenOnDuringSync = true

    var body: some View {
        NavigationStack {
            List {
                Section {
                    NavigationLink {
                        FreeRepsSettingsView(vm: vm)
                    } label: {
                        HStack(spacing: 12) {
                            Image("Logo")
                                .resizable()
                                .frame(width: 52, height: 52)
                                .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                            VStack(alignment: .leading, spacing: 3) {
                                Text(verbatim: vm.config.host)
                                    .font(.headline)
                                    .lineLimit(1)
                                HStack(spacing: 6) {
                                    Circle()
                                        .fill(connectionColor)
                                        .frame(width: 7, height: 7)
                                    Text(connectionLabel)
                                        .font(.footnote)
                                        .foregroundStyle(.secondary)
                                        .lineLimit(1)
                                }
                            }
                        }
                        .padding(.vertical, 2)
                    }
                }

                Section("Health & Sync") {
                    NavigationLink {
                        HealthPermissionsView(vm: vm)
                    } label: {
                        settingsRow(
                            "heart.fill",
                            title: "Apple Health Permissions",
                            subtitle: vm.permissionsRequested
                                ? (vm.deniedTypes.isEmpty ? "All permissions granted" : "\(vm.deniedTypes.count) permission(s) missing")
                                : "Tap to request permissions"
                        )
                    }

                    Toggle(isOn: $keepScreenOnDuringSync) {
                        settingsRow("sun.max.fill", title: "Keep Screen On", subtitle: "Prevent display sleep during full sync")
                    }
                    .tint(Color("Brand"))

                    NavigationLink {
                        SyncAdvancedView(vm: vm, syncViewModel: syncViewModel)
                    } label: {
                        settingsRow("slider.horizontal.3", title: "Advanced", subtitle: "Backfill settings, reset sync state")
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
                    .foregroundStyle(.primary)
                    LabeledContent("HealthKit Types", value: "\(HealthDataTypes.allQuantityTypes.count + HealthDataTypes.allCategoryTypes.count)")
                    NavigationLink("Acknowledgements") {
                        AcknowledgementsView()
                    }
                }
            }
            .listStyle(.insetGrouped)
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
            .onAppear {
                vm.refreshPermissionsState()
                if vm.connectionTestState == .idle { vm.testConnection() }
            }
            .onChange(of: vm.config) { vm.saveConfig() }
        }
    }

    private var appVersion: String {
        Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0"
    }

    private var connectionColor: Color {
        switch vm.connectionTestState {
        case .success: return .green
        case .failure: return .red
        case .idle, .testing: return .secondary
        }
    }

    private var connectionLabel: String {
        switch vm.connectionTestState {
        case .success:
            if let serverVersion = vm.serverVersion { return "Connected · Server \(serverVersion)" }
            return "Connected"
        case .failure: return "Not reachable"
        case .testing: return "Connecting…"
        case .idle: return "Not checked"
        }
    }

    private func settingsRow(_ systemName: String, title: String, subtitle: String) -> some View {
        HStack(spacing: 12) {
            iconBox(systemName)
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                Text(subtitle)
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private func iconBox(_ systemName: String) -> some View {
        Image(systemName: systemName)
            .font(.system(size: 19))
            .foregroundStyle(Color("Brand"))
            .frame(width: 30, height: 30)
            .background(Color("Brand").opacity(0.14), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
    }
}
