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
