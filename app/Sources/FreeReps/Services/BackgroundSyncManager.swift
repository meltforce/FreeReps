import Foundation
import HealthKit
import UIKit
import UserNotifications

/// Manages HKObserverQuery-based background delivery for continuous HealthKit → FreeReps sync.
///
/// Other health apps use this pattern: register observer queries for each data type at launch,
/// enable background delivery, and HealthKit wakes the app when new data is written. Unlike
/// BGProcessingTask (which runs when the device is idle/locked), observer callbacks fire
/// close to when data is recorded — the device is typically unlocked, so HealthKit data is accessible.
@MainActor
final class BackgroundSyncManager {

    static let shared = BackgroundSyncManager()

    private let healthStore = HealthKitService.shared.store
    private var observerQueries: [HKObserverQuery] = []
    private var pendingTypes: Set<String> = []
    private var debounceTask: Task<Void, Never>?
    private var isSyncing = false

    private init() {}

    // MARK: - Public API

    /// Call once from AppDelegate.didFinishLaunchingWithOptions to start monitoring HealthKit.
    func startObserving() {
        setupObserverQueries()
        enableBackgroundDelivery()
    }

    // MARK: - Observer Queries

    private func setupObserverQueries() {
        let readTypes = HealthDataTypes.allReadTypes

        for type in readTypes {
            guard let sampleType = type as? HKSampleType else { continue }

            let query = HKObserverQuery(sampleType: sampleType, predicate: nil) {
                [weak self] _, completionHandler, error in
                Task { @MainActor in
                    self?.handleObserverUpdate(sampleType: sampleType, error: error)
                    // MUST always call completionHandler or iOS thinks the query is still running
                    completionHandler()
                }
            }

            healthStore.execute(query)
            observerQueries.append(query)
        }
    }

    private func enableBackgroundDelivery() {
        let readTypes = HealthDataTypes.allReadTypes

        for type in readTypes {
            guard let sampleType = type as? HKSampleType else { continue }

            healthStore.enableBackgroundDelivery(for: sampleType, frequency: .immediate) { success, error in
                if let error = error {
                    print("[BackgroundSyncManager] enableBackgroundDelivery failed for \(sampleType.identifier): \(error.localizedDescription)")
                }
            }
        }
    }

    // MARK: - Observer Callback Handling

    private func handleObserverUpdate(sampleType: HKSampleType, error: Error?) {
        if let error = error {
            postFailureNotification("HealthKit observer error: \(error.localizedDescription)")
            return
        }

        pendingTypes.insert(sampleType.identifier)
        debounceAndSync()
    }

    /// Debounce rapid-fire observer callbacks. Multiple types can change at once
    /// (e.g. workout saves distance, energy, heart rate simultaneously). Wait 5s
    /// for all updates to arrive, then trigger a single incremental sync.
    private func debounceAndSync() {
        debounceTask?.cancel()
        debounceTask = Task {
            try? await Task.sleep(nanoseconds: 5_000_000_000) // 5 seconds
            guard !Task.isCancelled else { return }

            let types = pendingTypes
            pendingTypes.removeAll()

            guard !types.isEmpty else { return }
            await triggerIncrementalSync()
        }
    }

    // MARK: - Trigger Sync

    private func triggerIncrementalSync() async {
        guard UserDefaults.standard.bool(forKey: "backgroundSyncEnabled") else { return }
        guard !isSyncing, !SyncService.isSyncRunning else { return }
        isSyncing = true
        defer { isSyncing = false }

        let config = FreeRepsConfig.load()

        // Request extra background execution time from iOS.
        // The expiry handler cancels the sync task so runIncrementalSync exits cleanly
        // via CancellationError (no failure notification), then ends the background task.
        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        var syncTask: Task<Void, Never>?
        bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "observer-sync") {
            syncTask?.cancel()
            UIApplication.shared.endBackgroundTask(bgTaskID)
            bgTaskID = .invalid
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
            }
        }

        let state = SyncState()
        let service = SyncService(syncState: state)
        service.isBackgroundSync = true
        service.suppressLiveActivity = true
        let task = Task { await service.runIncrementalSync(config: config) }
        syncTask = task
        await task.value

        // Post notification on failure. Cancellation (expiry) sets no errorMessage,
        // so this only fires on genuine sync errors.
        if let error = state.errorMessage {
            postFailureNotification(error)
        }
    }

    // MARK: - Failure Notifications

    func postFailureNotification(_ message: String) {
        let content = UNMutableNotificationContent()
        content.title = "FreeReps Sync Failed"
        content.body = message
        content.sound = .default

        let request = UNNotificationRequest(
            identifier: "sync-failure",
            content: content,
            trigger: nil
        )

        UNUserNotificationCenter.current().add(request) { error in
            if let error = error {
                print("[BackgroundSyncManager] Failed to post notification: \(error.localizedDescription)")
            }
        }
    }

    /// Request notification permission. Call once at app launch.
    static func requestNotificationPermission() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { _, error in
            if let error = error {
                print("[BackgroundSyncManager] Notification permission error: \(error.localizedDescription)")
            }
        }
    }
}
