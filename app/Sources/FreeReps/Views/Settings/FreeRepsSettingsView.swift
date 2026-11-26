import SwiftUI

struct FreeRepsSettingsView: View {
    @ObservedObject var vm: SettingsViewModel
    @State private var manualTokenExpanded = false
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
                if let identity = vm.signedInAs {
                    Label("Signed in as \(identity.label)", systemImage: "person.crop.circle.badge.checkmark")
                    Button("Sign Out", role: .destructive) { vm.signOut() }
                        .disabled(vm.isSigningIn)
                } else {
                    if vm.accessToken.isEmpty {
                        Button {
                            vm.signIn()
                        } label: {
                            HStack {
                                Label("Sign In", systemImage: "person.crop.circle")
                                    .font(.body.weight(.semibold))
                                Spacer()
                                if vm.isSigningIn { ProgressView() }
                            }
                        }
                        .disabled(vm.isSigningIn)
                    } else {
                        Label("Token stored", systemImage: "key")
                        Button("Remove Token", role: .destructive) { vm.removeToken() }
                    }
                    DisclosureGroup("Enter a token manually", isExpanded: $manualTokenExpanded) {
                        SecureField("Access token", text: Binding(
                            get: { vm.accessToken },
                            set: { vm.setManualToken($0) }
                        ))
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .textContentType(.password)
                    }
                }
                if let notice = vm.signInNotice {
                    Text(notice).font(.caption).foregroundStyle(.secondary)
                }
                if let error = vm.signInError {
                    Text(error).font(.caption).foregroundStyle(.red)
                }
            } header: {
                Text("Access")
            } footer: {
                Text("Needed when the server has login enabled. Sign In opens the server's login page and stores a token for this iPhone; alternatively create a token under Settings → Account in the FreeReps web UI and enter it manually. A Tailscale-only server needs neither.")
            }

            Section {
                Button {
                    vm.testConnection()
                } label: {
                    HStack(spacing: 8) {
                        if vm.connectionTestState == .testing {
                            ProgressView().tint(.white)
                        } else {
                            Image(systemName: "network")
                        }
                        Text("Test Connection")
                    }
                    .font(.body.weight(.semibold))
                    .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .tint(Color("Brand"))
                .disabled(vm.connectionTestState == .testing)
                .listRowInsets(EdgeInsets())
                .listRowBackground(Color.clear)
            }

            connectionTestResult

            if let serverVersion = vm.serverVersion {
                Section("Server") {
                    LabeledContent("Version", value: serverVersion)
                }
            }
        }
        .navigationTitle("FreeReps Settings")
        .onChange(of: vm.config) { vm.saveConfig() }
    }

    @ViewBuilder
    private var connectionTestResult: some View {
        switch vm.connectionTestState {
        case .idle, .testing:
            EmptyView()
        case .success(let msg):
            resultRow(msg, icon: "checkmark.circle.fill", color: .green)
        case .failure(let msg):
            resultRow(msg, icon: "xmark.octagon.fill", color: .red)
        }
    }

    private func resultRow(_ message: String, icon: String, color: Color) -> some View {
        Section("Result") {
            Label {
                Text(message)
                    .font(.subheadline)
                    .fixedSize(horizontal: false, vertical: true)
            } icon: {
                Image(systemName: icon).foregroundStyle(color)
            }
        }
    }
}
