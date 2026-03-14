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
    @Published var permissionsRequested: Bool = UserDefaults.standard.bool(forKey: "hk_permissions_requested")
    @Published var deniedTypes: [HKObjectType] = []
    @Published var grantedTypes: [HKObjectType] = []
    @Published var errorMessage: String?

    private let healthKit = HealthKitService.shared

    init() {
        NotificationCenter.default.addObserver(
            forName: .iCloudSettingsDidChange,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            Task { @MainActor [weak self] in
                self?.config = .load()
            }
        }
    }

    func saveConfig() {
        config.save()
    }

    // MARK: - Connection test

    func testConnection() {
        guard connectionTestState != .testing else { return }
        connectionTestState = .testing
        let cfg = config
        Task {
            let service = FreeRepsService(config: cfg)
            do {
                let response = try await service.ping()
                connectionTestState = .success("Connected! \(response)")
            } catch {
                connectionTestState = .failure(error.localizedDescription)
            }
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
        Task {
            do {
                try await healthKit.requestAllPermissions()
            } catch {
                errorMessage = "HealthKit authorization failed: \(error.localizedDescription)"
            }
            UserDefaults.standard.set(true, forKey: "hk_permissions_requested")
            permissionsRequested = true
            refreshPermissionsState()
        }
    }

    func requestMissingPermissions() {
        guard !deniedTypes.isEmpty else { return }
        let types = Set(deniedTypes)
        Task {
            do {
                try await healthKit.requestPermissions(for: types)
            } catch {
                errorMessage = "HealthKit authorization failed: \(error.localizedDescription)"
            }
            refreshPermissionsState()
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
