import SwiftUI
import BackgroundTasks
import UserNotifications

@main
struct FreeRepsApp: App {

    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}

// MARK: - AppDelegate

final class AppDelegate: NSObject, UIApplicationDelegate {

    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
    ) -> Bool {
        UserDefaults.standard.register(defaults: ["backgroundSyncEnabled": true])
        registerBackgroundTasks()
        // Request notification permission for sync failure alerts
        BackgroundSyncManager.requestNotificationPermission()

        // Start HKObserverQuery-based background delivery for continuous HealthKit sync.
        // When new health data is written, HealthKit wakes the app and triggers an incremental sync.
        Task { @MainActor in
            BackgroundSyncManager.shared.startObserving()
        }

        return true
    }

    func applicationDidEnterBackground(_ application: UIApplication) {
        scheduleNextBackgroundSync()
    }

    private func registerBackgroundTasks() {
        BGTaskScheduler.shared.register(
            forTaskWithIdentifier: "com.meltforce.freereps.sync",
            using: nil
        ) { task in
            self.handleBackgroundSync(task: task as! BGProcessingTask)
        }
    }

    private func handleBackgroundSync(task: BGProcessingTask) {
        scheduleNextBackgroundSync()

        guard UserDefaults.standard.bool(forKey: "backgroundSyncEnabled") else {
            task.setTaskCompleted(success: true)
            return
        }

        let config = FreeRepsConfig.load()
        let isFullSyncResume = UserDefaults.standard.bool(forKey: "pendingFullSyncResume")

        let syncTask: Task<Void, Never> = Task { @MainActor in
            // If a foreground sync is already running, skip — it will handle persisting
            // state and scheduling follow-up work on its own.
            guard !SyncService.isSyncRunning else {
                task.setTaskCompleted(success: true)
                return
            }
            let state = SyncState()
            let service = SyncService(syncState: state)
            service.isBackgroundSync = true
            if isFullSyncResume {
                UserDefaults.standard.set(false, forKey: "pendingFullSyncResume")
                await service.runFullSync(config: config)
            } else {
                await service.runIncrementalSync(config: config)
            }
            if let error = state.errorMessage {
                BackgroundSyncManager.shared.postFailureNotification(error)
            }
            task.setTaskCompleted(success: true)
        }

        task.expirationHandler = {
            syncTask.cancel()
            // Give SyncService a moment to persist state before marking the task complete.
            // The cancellation propagates through Task.checkCancellation() calls, which
            // triggers syncState.persist() in the catch block, saving per-category progress.
            Task {
                try? await Task.sleep(nanoseconds: 500_000_000)
                task.setTaskCompleted(success: false)
            }
        }
    }

    func scheduleNextBackgroundSync() {
        let request = BGProcessingTaskRequest(identifier: "com.meltforce.freereps.sync")
        request.requiresNetworkConnectivity = true
        request.requiresExternalPower = false
        request.earliestBeginDate = Date(timeIntervalSinceNow: 15 * 60) // 15 min
        try? BGTaskScheduler.shared.submit(request)
    }
}
