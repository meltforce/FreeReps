import AppIntents
import UIKit

/// "Sync Health Data" for Shortcuts, Siri and the Action button. Runs in the app's
/// process without opening it, through the same SyncViewModel as the dashboard.
struct SyncHealthDataIntent: AppIntent {
    static let title: LocalizedStringResource = "Sync Health Data"
    static let description = IntentDescription(
        "Sends new Apple Health data to your FreeReps server. HealthKit is readable only while the iPhone is unlocked, so an automation should use a trigger that fires while the phone is in use."
    )

    @MainActor
    func perform() async throws -> some IntentResult & ProvidesDialog {
        // HealthKit rejects every read while the device is locked; say so instead of
        // reporting a sync that failed in every category.
        guard UIApplication.shared.isProtectedDataAvailable else {
            return .result(dialog: "iPhone is locked, so Health data cannot be read. Sync skipped.")
        }
        let vm = SyncViewModel.shared
        guard !vm.isAnySyncRunning else {
            return .result(dialog: "A FreeReps sync is already running.")
        }
        let record = await vm.runSync()
        switch record.outcome {
        case .succeeded:
            return .result(dialog: "FreeReps sync finished.")
        case .cancelled:
            return .result(dialog: "FreeReps sync was cancelled.")
        case .failed:
            return .result(dialog: "FreeReps sync failed: \(record.message ?? "unknown error")")
        }
    }
}

struct FreeRepsShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: SyncHealthDataIntent(),
            // German variants live in Resources/AppShortcuts.xcstrings, keyed by these phrases.
            phrases: [
                "Sync \(.applicationName)",
                "Sync health data with \(.applicationName)",
                "Start a \(.applicationName) sync",
            ],
            shortTitle: "Sync Health Data",
            systemImageName: "arrow.triangle.2.circlepath"
        )
    }
}
