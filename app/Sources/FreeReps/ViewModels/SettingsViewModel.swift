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

    // MARK: - Tailnet validation

    /// Checks whether the production host appears to be on a Tailnet by verifying
    /// the hostname suffix (.ts.net) and that it resolves to the Tailscale CGNAT range (100.64.0.0/10).
    func validateTailnet() {
        let host = config.host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !host.isEmpty else {
            tailnetWarning = nil
            return
        }

        // Quick suffix check
        if host.hasSuffix(".ts.net") {
            tailnetWarning = nil
            return
        }

        // Resolve DNS and check for Tailscale CGNAT range
        Task.detached { [weak self] in
            let isTailscale = Self.resolvesToTailscale(host: host)
            await MainActor.run {
                if isTailscale {
                    self?.tailnetWarning = nil
                } else {
                    self?.tailnetWarning = "This host doesn't appear to be on your Tailnet. FreeReps requires Tailscale for authentication."
                }
            }
        }
    }

    /// Resolves a hostname and checks if any address falls in 100.64.0.0/10.
    private nonisolated static func resolvesToTailscale(host: String) -> Bool {
        let hostRef = CFHostCreateWithName(nil, host as CFString).takeRetainedValue()
        var resolved = DarwinBoolean(false)
        CFHostStartInfoResolution(hostRef, .addresses, nil)
        guard let addresses = CFHostGetAddressing(hostRef, &resolved)?.takeUnretainedValue() as? [Data], resolved.boolValue else {
            return false
        }
        for addrData in addresses {
            if addrData.count >= MemoryLayout<sockaddr_in>.size {
                let family = addrData.withUnsafeBytes { $0.load(as: sockaddr.self).sa_family }
                if family == UInt8(AF_INET) {
                    let ip4 = addrData.withUnsafeBytes { $0.load(as: sockaddr_in.self).sin_addr.s_addr }
                    let byte0 = ip4 & 0xFF
                    let byte1 = (ip4 >> 8) & 0xFF
                    // 100.64.0.0/10 = first byte 100, top 2 bits of second byte = 01 (64..127)
                    if byte0 == 100 && (byte1 & 0xC0) == 0x40 {
                        return true
                    }
                }
            }
        }
        return false
    }

    @Published var tailnetWarning: String?

    // MARK: - Connection test

    func testConnection() {
        guard connectionTestState != .testing else { return }
        connectionTestState = .testing
        serverVersion = nil
        let cfg = config
        Task {
            let service = FreeRepsService(config: cfg)
            do {
                let response = try await service.ping()
                connectionTestState = .success("Connected! \(response)")
                // Fetch server version after successful connection
                await fetchServerVersion(service: service)
            } catch {
                connectionTestState = .failure(error.localizedDescription)
            }
        }
    }

    private func fetchServerVersion(service: FreeRepsService) async {
        do {
            let data = try await service.get(path: "api/v1/version")
            if let json = try JSONSerialization.jsonObject(with: data) as? [String: String],
               let version = json["version"] {
                serverVersion = version
            }
        } catch {
            // Non-critical — just don't show version
        }
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

    // MARK: - Per-object authorization (medications & vision prescriptions)

    func requestVisionPrescriptionAccess() {
        Task {
            await healthKit.requestVisionPrescriptionAuthorization()
        }
    }

    func requestMedicationAccess() {
        Task {
            if #available(iOS 26, *) {
                await healthKit.requestMedicationAuthorization()
            }
        }
    }
}
