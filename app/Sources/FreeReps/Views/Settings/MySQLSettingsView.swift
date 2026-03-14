import SwiftUI

struct FreeRepsSettingsView: View {
    @ObservedObject var vm: SettingsViewModel
    var body: some View {
        Form {
            Section {
                LabeledContent("Host") {
                    TextField("freereps", text: $vm.config.host)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                        .multilineTextAlignment(.trailing)
                }
                LabeledContent("Port") {
                    TextField("443", value: Binding(
                        get: { Int(vm.config.port) },
                        set: { vm.config.port = UInt16(clamping: $0) }
                    ), format: .number.grouping(.never))
                        .keyboardType(.numberPad)
                        .multilineTextAlignment(.trailing)
                }
                Toggle("Use HTTPS", isOn: $vm.config.useHTTPS)
            } header: {
                Text("Connection")
            } footer: {
                Text("Authentication is handled automatically via Tailscale. No credentials needed.")
            }

            Section {
                Picker("Initial Backfill", selection: Binding(
                    get: { vm.config.backfillYears ?? 0 },
                    set: { vm.config.backfillYears = $0 == 0 ? nil : $0 }
                )) {
                    Text("1 Year").tag(1)
                    Text("2 Years").tag(2)
                    Text("5 Years").tag(5)
                    Text("10 Years").tag(10)
                    Text("All Data").tag(0)
                }
            } header: {
                Text("Sync")
            } footer: {
                Text("How far back to sync HealthKit data on first full sync. Subsequent syncs only check recent data.")
            }

            Section {
                Button {
                    vm.testConnection()
                } label: {
                    HStack {
                        Label("Test Connection", systemImage: "network")
                        Spacer()
                        connectionTestIndicator
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
        }
        .navigationTitle("FreeReps Settings")
        .onChange(of: vm.config) { vm.saveConfig() }
    }

    @ViewBuilder
    private var connectionTestIndicator: some View {
        switch vm.connectionTestState {
        case .idle:
            EmptyView()
        case .testing:
            ProgressView().scaleEffect(0.7)
        case .success(let msg):
            Text(msg).font(.caption).foregroundStyle(.green).lineLimit(2)
        case .failure(let msg):
            Text(msg).font(.caption).foregroundStyle(.red).lineLimit(2)
        }
    }
}
