import SwiftUI

struct FreeRepsSettingsView: View {
    @ObservedObject var vm: SettingsViewModel
    var body: some View {
        Form {
            Section {
                LabeledContent("Host") {
                    TextField("freereps.your-tailnet.ts.net", text: $vm.config.host)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.URL)
                        .multilineTextAlignment(.trailing)
                }
            } header: {
                Text("Connection")
            } footer: {
                VStack(alignment: .leading, spacing: 4) {
                    Text("Your device must be on the same Tailnet as the server for authentication.")
                    if let warning = vm.tailnetWarning {
                        Label(warning, systemImage: "exclamationmark.triangle.fill")
                            .font(.caption2)
                            .foregroundStyle(.orange)
                    }
                }
            }

            Section {
                Toggle(isOn: $vm.config.testMode) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Test Mode")
                            .font(.subheadline.weight(.semibold))
                        Text("Connect to a separate test server")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
                if vm.config.testMode {
                    LabeledContent("Host") {
                        TextField("freereps-test.example.com", text: $vm.config.testHost)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .keyboardType(.URL)
                            .multilineTextAlignment(.trailing)
                    }
                    LabeledContent("Port") {
                        TextField("443", value: $vm.config.testPort, format: .number)
                            .keyboardType(.numberPad)
                            .multilineTextAlignment(.trailing)
                    }
                }
            } header: {
                Text("Testing")
            } footer: {
                if vm.config.testMode {
                    Text("Test mode overrides the connection above and connects to the test server instead. The test server must use HTTPS with a valid certificate (e.g., Let's Encrypt).")
                        .font(.caption2)
                }
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

                if let serverVersion = vm.serverVersion {
                    LabeledContent("Server Version", value: serverVersion)
                }
            }
        }
        .navigationTitle("FreeReps Settings")
        .onAppear { vm.validateTailnet() }
        .onChange(of: vm.config.host) { vm.validateTailnet() }
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
