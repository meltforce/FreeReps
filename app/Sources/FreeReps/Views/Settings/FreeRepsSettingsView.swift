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
    }

    @ViewBuilder
    private var connectionTestIndicator: some View {
        switch vm.connectionTestState {
        case .idle:
            EmptyView()
        case .testing:
            ProgressView().scaleEffect(0.7)
        case .success(let msg):
            Text(msg).font(.caption).foregroundStyle(.green).lineLimit(4).multilineTextAlignment(.trailing)
        case .failure(let msg):
            Text(msg).font(.caption).foregroundStyle(.red).lineLimit(4).multilineTextAlignment(.trailing)
        }
    }
}
