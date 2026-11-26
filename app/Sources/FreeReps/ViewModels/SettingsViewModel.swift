import Foundation
import HealthKit
import SwiftUI

enum ConnectionTestState: Equatable {
    case idle
    case testing
    case success(String)
    case failure(String)
}

@MainActor
final class SettingsViewModel: ObservableObject {

    @Published var config: FreeRepsConfig = .load()
    /// Kept in the Keychain (`AccessTokenStore`), not in `config`.
    @Published var accessToken: String = AccessTokenStore.load()
    /// Account of a token obtained by sign-in; nil without a token or with a manually entered one.
    @Published var signedInAs: AppLoginIdentity? = AccessTokenStore.load().isEmpty ? nil : AppLoginIdentity.load()
    @Published var isSigningIn = false
    @Published var signInError: String?
    @Published var signInNotice: String?
    @Published var connectionTestState: ConnectionTestState = .idle
    @Published var serverVersion: String?
    @Published var permissionsRequested: Bool = UserDefaults.standard.bool(forKey: "hk_permissions_requested")
    @Published var deniedTypes: [HKObjectType] = []
    @Published var grantedTypes: [HKObjectType] = []
    @Published var errorMessage: String?
    @Published var isRequestingPermissions = false

    private let healthKit = HealthKitService.shared

    init() { }

    func saveConfig() {
        config.save()
    }

    /// Stores a token typed into the manual field. Such a token has no identity.
    func setManualToken(_ token: String) {
        accessToken = token
        AppLoginIdentity.clear()
        signedInAs = nil
        signInError = nil
        signInNotice = nil
        if !AccessTokenStore.save(token) {
            connectionTestState = .failure("The access token could not be saved to the Keychain.")
        }
    }

    // MARK: - Sign-in

    func signIn() {
        guard !isSigningIn else { return }
        isSigningIn = true
        signInError = nil
        signInNotice = nil
        let baseURL = config.baseURL
        Task {
            defer { isSigningIn = false }
            do {
                let result = try await AppLogin.signIn(baseURL: baseURL)
                guard AccessTokenStore.save(result.token) else { throw AppLoginError.keychain }
                result.identity.save()
                accessToken = result.token
                signedInAs = result.identity
                testConnection()
            } catch AppLoginError.cancelled {
                // The user closed the sheet.
            } catch AppLoginError.noLogin {
                signInNotice = AppLoginError.noLogin.localizedDescription
            } catch {
                signInError = error.localizedDescription
            }
        }
    }

    /// Revokes the token on the server and deletes it locally, whatever the server answers.
    func signOut() {
        guard !isSigningIn else { return }
        isSigningIn = true
        signInError = nil
        signInNotice = nil
        let baseURL = config.baseURL
        let token = AccessTokenStore.load()
        Task {
            defer { isSigningIn = false }
            var revokeFailure: String?
            if !token.isEmpty {
                do {
                    try await AppLogin.revoke(baseURL: baseURL, token: token)
                } catch {
                    revokeFailure = error.localizedDescription
                }
            }
            removeToken()
            if let revokeFailure {
                signInError = "Signed out on this iPhone, but the server did not confirm the revocation (\(revokeFailure)). The token may still be listed under Settings → Account in the web UI."
            }
        }
    }

    /// Deletes the stored token without contacting the server.
    func removeToken() {
        accessToken = ""
        AppLoginIdentity.clear()
        signedInAs = nil
        serverVersion = nil
        connectionTestState = .idle
        if !AccessTokenStore.save("") {
            signInError = "The access token could not be removed from the Keychain."
        }
    }

    // MARK: - Connection test

    func testConnection() {
        guard connectionTestState != .testing else { return }
        serverVersion = nil
        // The service reads the token from the Keychain.
        guard AccessTokenStore.save(accessToken) else {
            connectionTestState = .failure("The access token could not be saved to the Keychain.")
            return
        }
        connectionTestState = .testing
        let cfg = config
        Task {
            let service = FreeRepsService(config: cfg)
            // Step 1 needs no credentials: /api/v1/version answers on every
            // FreeReps server, so an unreachable or foreign host is told apart
            // from a refused sign-in.
            let version: String
            do {
                version = try await Self.identifyServer(service)
            } catch let error as FreeRepsError {
                switch error {
                case .connectionFailed(let msg):
                    connectionTestState = .failure("No server reachable at this address: \(msg)")
                default:
                    connectionTestState = .failure("A server answers at this address, but it is not a FreeReps server.")
                }
                return
            } catch {
                connectionTestState = .failure("No server reachable at this address: \(error.localizedDescription)")
                return
            }
            serverVersion = version

            // Step 2: the authenticated identity.
            do {
                let name = try await Self.identity(service)
                connectionTestState = .success("Connected as \(name)")
            } catch FreeRepsError.unauthorized {
                connectionTestState = .failure(AccessTokenStore.load().isEmpty
                    ? "FreeReps server found. It requires sign-in."
                    : "FreeReps server found, but it rejects the stored token. Sign in again.")
            } catch FreeRepsError.forbidden {
                connectionTestState = .failure("FreeReps server found, but access is denied: the token lacks the api scope or the account is disabled.")
            } catch {
                connectionTestState = .failure("FreeReps server found, but the identity check failed: \(error.localizedDescription)")
            }
        }
    }

    /// Returns the version a FreeReps server reports. Throws `.decodingError`
    /// when the answer is not the version document.
    private static func identifyServer(_ service: FreeRepsService) async throws -> String {
        let data = try await service.get(path: "api/v1/version")
        guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let version = json["version"] as? String else {
            throw FreeRepsError.decodingError("not a FreeReps version document")
        }
        return version
    }

    /// The display name or login `/api/v1/me` reports.
    private static func identity(_ service: FreeRepsService) async throws -> String {
        let raw = try await service.ping()
        let json = (try? JSONSerialization.jsonObject(with: Data(raw.utf8))) as? [String: Any]
        let display = json?["display_name"] as? String ?? ""
        let login = json?["login"] as? String ?? ""
        return display.isEmpty ? (login.isEmpty ? "unknown user" : login) : display
    }

    // MARK: - HealthKit permissions

    func refreshPermissionsState() {
        let (granted, denied) = healthKit.checkAllPermissionStatuses()
        self.grantedTypes = granted
        self.deniedTypes = denied
        if !granted.isEmpty {
            permissionsRequested = true
            UserDefaults.standard.set(true, forKey: "hk_permissions_requested")
        } else {
            permissionsRequested = UserDefaults.standard.bool(forKey: "hk_permissions_requested")
        }
    }

    var hasDeniedPermissions: Bool {
        !deniedTypes.isEmpty
    }

    func requestAllPermissions() {
        guard !isRequestingPermissions else { return }
        isRequestingPermissions = true
        Task {
            do {
                try await healthKit.requestAllPermissions()
            } catch {
                errorMessage = "HealthKit authorization failed: \(error.localizedDescription)"
            }
            UserDefaults.standard.set(true, forKey: "hk_permissions_requested")
            permissionsRequested = true
            refreshPermissionsState()
            isRequestingPermissions = false
        }
    }

    func requestMissingPermissions() {
        guard !deniedTypes.isEmpty, !isRequestingPermissions else { return }
        isRequestingPermissions = true
        let types = Set(deniedTypes)
        Task {
            do {
                try await healthKit.requestPermissions(for: types)
            } catch {
                errorMessage = "HealthKit authorization failed: \(error.localizedDescription)"
            }
            refreshPermissionsState()
            isRequestingPermissions = false
        }
    }
}
