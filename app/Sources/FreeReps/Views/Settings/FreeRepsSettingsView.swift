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
                Text("The server must be reachable from this iPhone (Tailscale or a public address).")
            }

            Section {
                SecureField("Access token", text: $vm.accessToken)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .textContentType(.password)
            } header: {
                Text("Access Token")
            } footer: {
                Text("Needed when the server has login enabled: create a token under Settings → Account in the FreeReps web UI. Leave empty for a Tailscale-only server. Not sent in test mode.")
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
        .onChange(of: vm.config) { vm.saveConfig() }
        .onChange(of: vm.accessToken) { vm.saveAccessToken() }
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
