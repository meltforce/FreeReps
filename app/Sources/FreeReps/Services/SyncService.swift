import ActivityKit
import BackgroundTasks
import CoreLocation
import Foundation
import HealthKit
import UIKit

// Batch size for HTTP requests
private let batchSize = 500

// MARK: - AsyncSemaphore

/// Limits concurrent access to a resource (e.g. cap HealthKit queries at 5).
actor AsyncSemaphore {
    private var count: Int
    private var waiters: [CheckedContinuation<Void, Never>] = []

    init(value: Int) { self.count = value }

    func wait() async {
        if count > 0 {
            count -= 1
        } else {
            await withCheckedContinuation { cont in
                waiters.append(cont)
            }
        }
    }

    func signal() {
        if let next = waiters.first {
            waiters.removeFirst()
            next.resume()
        } else {
            count += 1
        }
    }
}

// MARK: - SyncService

@MainActor
final class SyncService: ObservableObject {

    private let healthKit = HealthKitService.shared
    let syncState: SyncState
    private var freereps: FreeRepsService?

    /// Sparse categories that have very few records — skip 90-day windowing, query full range at once.
    private static let sparseCategories: Set<String> = [
        "cat_ecg", "cat_audiogram", "cat_vision", "cat_state_of_mind", "cat_medications"
    ]

    // When true, skip live activity and allow resumable sync across background task invocations
    var isBackgroundSync = false

    // When true, never create or update a Live Activity (used for observer-triggered real-time syncs)
    var suppressLiveActivity = false

    /// Set by the caller before `runHistoricalBackfill` so the background-task expiry
    /// handler can cancel the Swift Task when iOS reclaims background time.
    var taskForCancellation: Task<Void, Never>?

    // Class-level flag so BackgroundSyncManager can check whether ANY SyncService instance
    // (foreground or background) is currently running, preventing concurrent syncs.
    @MainActor static private(set) var isSyncRunning = false

    // Live Activity
    private var liveActivity: Activity<SyncActivityAttributes>?
    private var lastLiveActivityUpdate: Date = .distantPast

    init(syncState: SyncState) {
        self.syncState = syncState
        setupCategories()
        syncState.restore()
    }

    private func setupCategories() {
        var cats: [CategorySyncState] = []
        // Quantity categories
        for (cat, types) in HealthDataTypes.quantityTypesByCategory {
            let count = types.count
            cats.append(CategorySyncState(
                id: "qty_\(cat.rawValue)",
                displayName: cat.rawValue,
                systemImage: cat.systemImage,
                status: .idle,
                recordCount: 0,
                lastSyncDate: nil,
                currentProgress: 0,
                totalEstimated: count
            ))
        }
        // Special categories
        let specials: [(String, String, String)] = [
            ("cat_category", "Health Events", "heart.text.square.fill"),
            ("cat_workouts", "Workouts", "dumbbell.fill"),
            ("cat_bp", "Blood Pressure", "drop.fill"),
            ("cat_ecg", "ECG", "waveform.path.ecg.rectangle.fill"),
            ("cat_audiogram", "Audiogram", "ear.badge.waveform"),
            ("cat_activity_summaries", "Activity Rings", "chart.bar.fill"),
            ("cat_workout_routes", "Workout Routes", "map.fill"),
            ("cat_medications", "Medications", "pills.fill"),
            ("cat_vision", "Vision Prescriptions", "eye.fill"),
            ("cat_state_of_mind", "State of Mind", "brain.head.profile"),
        ]
        for (id, name, icon) in specials {
            cats.append(CategorySyncState(
                id: id,
                displayName: name,
                systemImage: icon,
                status: .idle,
                recordCount: 0,
                lastSyncDate: nil,
                currentProgress: 0,
                totalEstimated: 1
            ))
        }
        syncState.categories = cats
    }

    // MARK: - Live Activity

    private func startLiveActivity(isFullSync: Bool) {
        guard !suppressLiveActivity else { return }
        if isBackgroundSync {
            liveActivity = Activity<SyncActivityAttributes>.activities.first
            if liveActivity != nil { return }
            // No existing activity — only create one if the app is currently active.
            // BGProcessingTask keeps the app in .background state, so this only fires when
            // the user has the app open (e.g. they opened the app mid-background-sync).
            guard UIApplication.shared.applicationState == .active else { return }
            // Fall through to create a new activity
        }
        guard ActivityAuthorizationInfo().areActivitiesEnabled else { return }
        let initial = SyncActivityAttributes.ContentState(
            phase: "Connecting",
            operation: "Connecting to FreeReps\u{2026}",
            recordsInserted: 0,
            isFullSync: isFullSync
        )
        do {
            liveActivity = try Activity.request(
                attributes: SyncActivityAttributes(),
                content: ActivityContent(state: initial, staleDate: nil),
                pushType: nil
            )
        } catch {
            // Live Activities not available or denied — sync continues without it
        }
    }

    private func updateLiveActivity(phase: String, operation: String, records: Int) {
        guard !suppressLiveActivity else { return }
        // If no activity yet and we're now in the foreground, try to create one.
        // This covers the case where the user opens the app mid-background-sync.
        if liveActivity == nil {
            startLiveActivity(isFullSync: syncState.isFullSyncRunning)
        }
        guard Date().timeIntervalSince(lastLiveActivityUpdate) >= 1.0 else { return }
        lastLiveActivityUpdate = Date()
        let activity = liveActivity ?? Activity<SyncActivityAttributes>.activities.first
        guard let activity else { return }
        let isFullSync = syncState.isFullSyncRunning
        let state = SyncActivityAttributes.ContentState(
            phase: phase,
            operation: operation,
            recordsInserted: records,
            isFullSync: isFullSync
        )
        let content = ActivityContent(state: state, staleDate: nil)
        // Await the update directly to ensure it completes before moving on
        Task { @MainActor in
            await activity.update(content)
        }
    }

    private func endLiveActivity(totalRecords: Int) {
        guard !suppressLiveActivity else { return }
        let activity = liveActivity ?? Activity<SyncActivityAttributes>.activities.first
        guard let activity else { return }
        let isFullSync = syncState.isFullSyncRunning
        let finalState = SyncActivityAttributes.ContentState(
            phase: "Done",
            operation: "Synced \(totalRecords.formatted()) records",
            recordsInserted: totalRecords,
            isFullSync: isFullSync
        )
        let finalContent = ActivityContent(state: finalState, staleDate: nil)
        // Capture reference and nil out immediately to prevent double-end
        self.liveActivity = nil
        // End with a short delay so the "Done" state is visible before dismissal
        Task { @MainActor in
            await activity.end(finalContent, dismissalPolicy: .after(.now + 5))
        }
    }

    // MARK: - Connection management

    func connectFreeReps(config: FreeRepsConfig) {
        self.freereps = FreeRepsService(config: config)
    }

    func disconnectFreeReps() {
        self.freereps = nil
    }

    // MARK: - Pre-sync validation

    /// Check HealthKit authorization and FreeReps connectivity before syncing.
    /// Returns a list of issues that need user attention.
    func validatePrerequisites(config: FreeRepsConfig) async -> [SyncPrerequisiteIssue] {
        var issues: [SyncPrerequisiteIssue] = []

        // Check HealthKit availability
        if !healthKit.isAvailable {
            issues.append(.healthDataUnavailable)
            return issues
        }

        // Check if permissions were ever requested
        let permissionsRequested = UserDefaults.standard.bool(forKey: "hk_permissions_requested")
        if !permissionsRequested {
            issues.append(.healthPermissionsNotRequested)
        }

        // Check a sample of key HealthKit types for authorization.
        // authorizationStatus only tracks write permission. For read-only types,
        // .notDetermined means the dialog was never shown (truly not requested),
        // while .sharingDenied means the dialog was shown (read grant/deny is hidden by iOS).
        let criticalTypes: [HKObjectType] = [
            HKObjectType.quantityType(forIdentifier: .heartRate)!,
            HKObjectType.quantityType(forIdentifier: .stepCount)!,
            HKObjectType.quantityType(forIdentifier: .bodyMass)!,
            HKObjectType.workoutType(),
            HKObjectType.quantityType(forIdentifier: .appleSleepingWristTemperature)!,
        ]
        let notRequestedTypes = criticalTypes.filter {
            healthKit.authorizationStatus(for: $0) == .notDetermined
        }
        if !notRequestedTypes.isEmpty {
            issues.append(.somePermissionsDenied(count: notRequestedTypes.count))
        }

        // Test FreeReps connectivity
        do {
            let service = FreeRepsService(config: config)
            _ = try await service.ping()
        } catch {
            issues.append(.connectionFailed(error.localizedDescription))
        }

        return issues
    }

    // MARK: - Full sync

    func runFullSync(config: FreeRepsConfig) async {
        await runHistoricalBackfill(config: config)
    }

    // MARK: - Single-category sync

    func runSingleCategorySync(categoryID: String, config: FreeRepsConfig) async {
        guard !syncState.isAnySyncRunning else { return }
        syncState.isFullSyncRunning = true
        SyncService.isSyncRunning = true
        defer { SyncService.isSyncRunning = false }
        syncState.errorMessage = nil
        syncState.currentOperation = "Connecting\u{2026}"
        startLiveActivity(isFullSync: false)

        let anchor = Date()
        let epoch = Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!

        do {
            connectFreeReps(config: config)
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }

            syncState.updateCategory(categoryID, status: .syncing)
            syncState.currentOperation = "Syncing\u{2026}"

            let count: Int
            if categoryID.hasPrefix("qty_") {
                let rawCat = String(categoryID.dropFirst(4))
                guard let cat = HealthCategory(rawValue: rawCat),
                      let types = HealthDataTypes.quantityTypesByCategory.first(where: { $0.0 == cat })?.1 else {
                    throw FreeRepsError.connectionFailed("Unknown category: \(categoryID)")
                }
                count = try await backfillQuantityCategory(
                    catID: categoryID, cat: cat, types: types,
                    from: epoch, until: anchor, config: config
                )
            } else if Self.sparseCategories.contains(categoryID) {
                // Sparse categories: skip windowing, query full range directly
                switch categoryID {
                case "cat_ecg":           count = try await syncECG(since: epoch, until: anchor)
                case "cat_audiogram":     count = try await syncAudiograms(since: epoch, until: anchor)
                case "cat_medications":   count = try await syncMedications(since: epoch, until: anchor)
                case "cat_vision":        count = try await syncVisionPrescriptions(since: epoch, until: anchor)
                case "cat_state_of_mind": count = try await syncStateOfMind(since: epoch, until: anchor)
                default: count = 0
                }
            } else {
                count = try await backfillSpecialCategory(
                    catID: categoryID, from: epoch, until: anchor, config: config
                ) { [self] windowStart, windowEnd in
                    switch categoryID {
                    case "cat_category":          return try await syncCategorySamples(since: windowStart, until: windowEnd, insertBatchSize: 50)
                    case "cat_workouts":          return try await syncWorkouts(since: windowStart, until: windowEnd)
                    case "cat_bp":                return try await syncBloodPressure(since: windowStart, until: windowEnd)
                    case "cat_activity_summaries": return try await syncActivitySummaries(since: windowStart, until: windowEnd)
                    case "cat_workout_routes":    return try await syncWorkoutRoutes(since: windowStart, until: windowEnd)
                    default: return 0
                    }
                }
            }

            syncState.updateCategory(categoryID, status: .completed, recordCount: count, lastSyncDate: Date())
            syncState.lastSyncDate = Date()
            syncState.currentOperation = ""
            // Clear cursor so a future full sync re-visits this category from the beginning
            syncState.backfillCursors.removeValue(forKey: categoryID)
            syncState.persist()
            endLiveActivity(totalRecords: syncState.totalRecords)
            disconnectFreeReps()

        } catch is CancellationError {
            disconnectFreeReps()
            endLiveActivity(totalRecords: syncState.totalRecords)
            syncState.currentOperation = "Sync cancelled"
            if case .syncing = syncState.categories.first(where: { $0.id == categoryID })?.status {
                syncState.updateCategory(categoryID, status: .idle)
            }
            syncState.persist()
        } catch {
            disconnectFreeReps()
            endLiveActivity(totalRecords: syncState.totalRecords)
            syncState.errorMessage = error.localizedDescription
            syncState.currentOperation = ""
            syncState.updateCategory(categoryID, status: .failed(error.localizedDescription))
            syncState.persist()
        }

        syncState.isFullSyncRunning = false
    }

    // MARK: - Historical backfill (windowed, resumable)

    func runHistoricalBackfill(config: FreeRepsConfig) async {
        guard !syncState.isAnySyncRunning else { return }
        syncState.isFullSyncRunning = true
        SyncService.isSyncRunning = true
        defer { SyncService.isSyncRunning = false }
        syncState.errorMessage = nil
        syncState.currentOperation = "Connecting\u{2026}"
        startLiveActivity(isFullSync: true)

        if !isBackgroundSync {
            UIApplication.shared.isIdleTimerDisabled = true
        }
        defer {
            if !isBackgroundSync {
                UIApplication.shared.isIdleTimerDisabled = false
            }
        }

        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        if !isBackgroundSync {
            bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "health-full-sync") {
                self.taskForCancellation?.cancel()
                self.syncState.persist()
                UserDefaults.standard.set(true, forKey: "pendingFullSyncResume")
                let req = BGProcessingTaskRequest(identifier: "com.meltforce.freereps.sync")
                req.requiresNetworkConnectivity = true
                req.requiresExternalPower = false
                req.earliestBeginDate = nil
                try? BGTaskScheduler.shared.submit(req)
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
            }
        }

        let earliest = config.backfillStartDate
        let historicalStart: Date

        if let previousAnchor = syncState.backfillAnchorDate, syncState.hasCompletedFullSync {
            // A full backfill previously completed. Re-run with a 7-day lookback so samples
            // that arrived in HealthKit after the previous anchor (with past startDates) are
            // captured. FreeReps uses ON CONFLICT DO NOTHING, making this safe.
            historicalStart = previousAnchor.addingTimeInterval(-7 * 24 * 3600)
            syncState.backfillAnchorDate = Date()
            syncState.backfillCursors.removeAll()
            syncState.hasCompletedFullSync = false
            syncState.persist()
        } else if syncState.backfillAnchorDate != nil {
            // Anchor exists but sync hasn't completed — resuming an interrupted backfill.
            // If backfill range was shortened, clear cursors that predate the new start.
            historicalStart = earliest
            for (key, cursor) in syncState.backfillCursors where cursor < earliest {
                syncState.backfillCursors[key] = nil
            }
        } else {
            // First-time full sync: backfill from configured start date.
            syncState.backfillAnchorDate = Date()
            syncState.persist()
            historicalStart = earliest
        }
        let anchor = syncState.backfillAnchorDate!

        do {
            connectFreeReps(config: config)
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }

            var failedCategories: [String] = []

            // Quantity categories — 90-day windowed backfill
            for (cat, types) in HealthDataTypes.quantityTypesByCategory {
                let catID = "qty_\(cat.rawValue)"
                try Task.checkCancellation()
                if syncState.backfillCursors[catID] == anchor { continue }

                syncState.updateCategory(catID, status: .syncing)
                syncState.currentOperation = "Backfilling \(cat.rawValue)\u{2026}"
                do {
                    let count = try await backfillQuantityCategory(
                        catID: catID, cat: cat, types: types,
                        from: historicalStart, until: anchor, config: config
                    )
                    syncState.updateCategory(catID, status: .completed, recordCount: count, lastSyncDate: Date())
                    updateLiveActivity(phase: cat.rawValue, operation: "Backfilled \(cat.rawValue) (\(count.formatted()) records)", records: count)
                } catch is CancellationError {
                    throw CancellationError()
                } catch {
                    syncState.updateCategory(catID, status: .failed(error.localizedDescription))
                    failedCategories.append(cat.rawValue)
                    print("Category \(cat.rawValue) failed: \(error.localizedDescription)")
                }
            }

            // Special categories — sequential heavy categories use 90-day windowed backfill
            let heavySpecials: [(String, String)] = [
                ("cat_category", "Health Events"),
                ("cat_workouts", "Workouts"),
                ("cat_bp", "Blood Pressure"),
                ("cat_activity_summaries", "Activity Rings"),
                ("cat_workout_routes", "Workout Routes"),
            ]
            for (catID, displayName) in heavySpecials {
                try Task.checkCancellation()
                if syncState.backfillCursors[catID] == anchor { continue }

                syncState.updateCategory(catID, status: .syncing)
                syncState.currentOperation = "Backfilling \(displayName)\u{2026}"
                do {
                    let count = try await backfillSpecialCategory(
                        catID: catID, from: historicalStart, until: anchor, config: config
                    ) { [self] windowStart, windowEnd in
                        switch catID {
                        case "cat_category":
                            return try await syncCategorySamples(since: windowStart, until: windowEnd, insertBatchSize: 50)
                        case "cat_workouts":
                            return try await syncWorkouts(since: windowStart, until: windowEnd)
                        case "cat_bp":
                            return try await syncBloodPressure(since: windowStart, until: windowEnd)
                        case "cat_activity_summaries":
                            return try await syncActivitySummaries(since: windowStart, until: windowEnd)
                        case "cat_workout_routes":
                            return try await syncWorkoutRoutes(since: windowStart, until: windowEnd)
                        default:
                            return 0
                        }
                    }
                    syncState.updateCategory(catID, status: .completed, recordCount: count, lastSyncDate: Date())
                    updateLiveActivity(phase: displayName, operation: "Backfilled \(displayName) (\(count.formatted()) records)", records: count)
                } catch is CancellationError {
                    throw CancellationError()
                } catch {
                    syncState.updateCategory(catID, status: .failed(error.localizedDescription))
                    failedCategories.append(displayName)
                    print("Category \(displayName) failed: \(error.localizedDescription)")
                }
            }

            // Sparse categories — skip windowing, query full range, run in parallel
            let sparseSpecials: [(String, String)] = [
                ("cat_ecg", "ECG"),
                ("cat_audiogram", "Audiograms"),
                ("cat_medications", "Medications"),
                ("cat_vision", "Vision Prescriptions"),
                ("cat_state_of_mind", "State of Mind"),
            ]
            try Task.checkCancellation()
            syncState.currentOperation = "Backfilling sparse categories\u{2026}"
            do {
                try await withThrowingTaskGroup(of: (String, String, Int).self) { group in
                    for (catID, displayName) in sparseSpecials {
                        if syncState.backfillCursors[catID] == anchor { continue }
                        syncState.updateCategory(catID, status: .syncing)

                        group.addTask { [self] in
                            let count: Int
                            switch catID {
                            case "cat_ecg":       count = try await syncECG(since: historicalStart, until: anchor)
                            case "cat_audiogram": count = try await syncAudiograms(since: historicalStart, until: anchor)
                            case "cat_medications": count = try await syncMedications(since: historicalStart, until: anchor)
                            case "cat_vision":    count = try await syncVisionPrescriptions(since: historicalStart, until: anchor)
                            case "cat_state_of_mind": count = try await syncStateOfMind(since: historicalStart, until: anchor)
                            default: count = 0
                            }
                            return (catID, displayName, count)
                        }
                    }
                    for try await (catID, displayName, count) in group {
                        syncState.updateCategory(catID, status: .completed, recordCount: count, lastSyncDate: Date())
                        syncState.backfillCursors[catID] = anchor
                        updateLiveActivity(phase: displayName, operation: "Backfilled \(displayName) (\(count.formatted()) records)", records: count)
                    }
                }
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                // Individual sparse category failures are caught within the task group
                failedCategories.append("Sparse categories")
                print("Sparse categories failed: \(error.localizedDescription)")
            }
            syncState.persist()

            // Mark complete even if some categories failed — successful ones keep their progress.
            syncState.hasCompletedFullSync = failedCategories.isEmpty
            syncState.lastSyncDate = Date()
            if failedCategories.isEmpty {
                syncState.currentOperation = "Backfill complete"
            } else {
                syncState.errorMessage = "\(failedCategories.count) category(ies) failed: \(failedCategories.joined(separator: ", ")). Successfully synced categories are saved."
                syncState.currentOperation = "Backfill complete with errors"
            }

            syncState.persist()
            endLiveActivity(totalRecords: syncState.totalRecords)
            disconnectFreeReps()

        } catch is CancellationError {
            disconnectFreeReps()
            endLiveActivity(totalRecords: syncState.totalRecords)
            syncState.currentOperation = "Sync cancelled"
            for i in syncState.categories.indices {
                if case .syncing = syncState.categories[i].status {
                    syncState.categories[i].status = .idle
                }
            }
            syncState.persist()
        } catch {
            disconnectFreeReps()
            endLiveActivity(totalRecords: syncState.totalRecords)
            syncState.errorMessage = error.localizedDescription
            syncState.currentOperation = ""
            for i in syncState.categories.indices {
                if case .syncing = syncState.categories[i].status {
                    syncState.categories[i].status = .failed(error.localizedDescription)
                }
            }
            syncState.persist()
        }

        syncState.isFullSyncRunning = false
    }

    // MARK: - Backfill helpers

    /// Backfills a quantity category in 90-day windows from `historicalStart` to `anchor`,
    /// resuming from `syncState.backfillCursors[catID]` if set.
    private func backfillQuantityCategory(
        catID: String,
        cat: HealthCategory,
        types: [QuantityTypeDescriptor],
        from historicalStart: Date,
        until anchor: Date,
        config: FreeRepsConfig
    ) async throws -> Int {
        let windowSize: TimeInterval = 90 * 24 * 60 * 60
        var cursor = syncState.backfillCursors[catID] ?? historicalStart
        var total = 0
        let totalWindows = Int(ceil(anchor.timeIntervalSince(historicalStart) / windowSize))
        var windowIdx = cursor > historicalStart
            ? Int(ceil(cursor.timeIntervalSince(historicalStart) / windowSize))
            : 0

        while cursor < anchor {
            try Task.checkCancellation()
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }

            let windowEnd = min(cursor.addingTimeInterval(windowSize), anchor)
            var windowTotal = 0
            var retries = 0
            while true {
                do {
                    let semaphore = AsyncSemaphore(value: 5)
                    windowTotal = try await withThrowingTaskGroup(of: Int.self) { group in
                        for typeDesc in types {
                            group.addTask {
                                await semaphore.wait()
                                defer { Task { await semaphore.signal() } }
                                return try await self.syncQuantityType(
                                    typeDesc: typeDesc,
                                    since: cursor, until: windowEnd,
                                    insertBatchSize: 50
                                )
                            }
                        }
                        var sum = 0
                        for try await count in group { sum += count }
                        return sum
                    }
                    break
                } catch is CancellationError {
                    throw CancellationError()
                } catch where retries < 3 {
                    // Generic retry with backoff
                    retries += 1
                    try await Task.sleep(nanoseconds: UInt64(retries) * 500_000_000)
                }
            }
            total += windowTotal

            cursor = windowEnd
            windowIdx += 1
            syncState.backfillCursors[catID] = cursor
            syncState.persist()
            syncState.updateCategory(catID, status: .syncing, progress: windowIdx, total: totalWindows)
            let op = "Backfilling \(cat.rawValue): window \(windowIdx)/\(totalWindows)\u{2026}"
            syncState.currentOperation = op
            updateLiveActivity(phase: cat.rawValue, operation: op, records: total)
        }
        return total
    }

    /// Backfills a special (non-quantity) category in 90-day windows, resuming from cursor.
    private func backfillSpecialCategory(
        catID: String,
        from historicalStart: Date,
        until anchor: Date,
        config: FreeRepsConfig,
        syncWindow: (Date, Date) async throws -> Int
    ) async throws -> Int {
        let windowSize: TimeInterval = 90 * 24 * 60 * 60
        var cursor = syncState.backfillCursors[catID] ?? historicalStart
        var total = 0
        let totalWindows = Int(ceil(anchor.timeIntervalSince(historicalStart) / windowSize))
        var windowIdx = cursor > historicalStart
            ? Int(ceil(cursor.timeIntervalSince(historicalStart) / windowSize))
            : 0

        while cursor < anchor {
            try Task.checkCancellation()
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }

            let windowEnd = min(cursor.addingTimeInterval(windowSize), anchor)
            var retries = 0
            var windowTotal = 0
            while true {
                do {
                    windowTotal = try await syncWindow(cursor, windowEnd)
                    break
                } catch is CancellationError {
                    throw CancellationError()
                } catch where retries < 3 {
                    // Generic retry with backoff
                    retries += 1
                    try await Task.sleep(nanoseconds: UInt64(retries) * 500_000_000)
                }
            }
            total += windowTotal

            cursor = windowEnd
            windowIdx += 1
            syncState.backfillCursors[catID] = cursor
            syncState.persist()
            syncState.updateCategory(catID, status: .syncing, progress: windowIdx, total: totalWindows)
            let displayName = syncState.categories.first(where: { $0.id == catID })?.displayName ?? catID
            let op = "Backfilling \(displayName): window \(windowIdx)/\(totalWindows)\u{2026}"
            syncState.currentOperation = op
            updateLiveActivity(phase: displayName, operation: op, records: total)
        }
        return total
    }

    // MARK: - Incremental sync

    func runIncrementalSync(config: FreeRepsConfig) async {
        guard !syncState.isAnySyncRunning else { return }
        syncState.isIncrementalSyncRunning = true
        SyncService.isSyncRunning = true
        defer { SyncService.isSyncRunning = false }
        syncState.errorMessage = nil
        startLiveActivity(isFullSync: false)

        // Keep screen awake during foreground sync to prevent auto-lock killing HealthKit access
        if !isBackgroundSync {
            UIApplication.shared.isIdleTimerDisabled = true
        }
        defer {
            if !isBackgroundSync {
                UIApplication.shared.isIdleTimerDisabled = false
            }
        }

        // Request extra background execution time if user switches away during sync
        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        if !isBackgroundSync {
            bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "health-incremental-sync") {
                self.syncState.persist()
                let req = BGProcessingTaskRequest(identifier: "com.meltforce.freereps.sync")
                req.requiresNetworkConnectivity = true
                req.earliestBeginDate = nil
                try? BGTaskScheduler.shared.submit(req)
                UIApplication.shared.endBackgroundTask(bgTaskID)
                bgTaskID = .invalid
            }
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
            }
        }

        do {
            // Pre-first-unlock guard: isProtectedDataAvailable is false only before the very
            // first unlock after boot. The errorDatabaseInaccessible suppression below handles
            // the common screen-locked case (device unlocked at least once since boot).
            if isBackgroundSync {
                guard UIApplication.shared.isProtectedDataAvailable else {
                    syncState.isIncrementalSyncRunning = false
                    return
                }
            }

            connectFreeReps(config: config)
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }

            // Find last sync date from UserDefaults-backed syncState.
            let distantPast = Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
            let since = syncState.lastSyncDate ?? distantPast
            // Apply a 7-day lookback for HealthKit queries so late-arriving samples (e.g. apps
            // that backfill historical entries into HealthKit after the fact) are captured.
            // FreeReps uses ON CONFLICT DO NOTHING, making re-syncing the overlap window safe.
            let querySince = syncState.lastSyncDate.map { $0.addingTimeInterval(-7 * 24 * 3600) } ?? distantPast

            let opLabel = syncState.lastSyncDate != nil
                ? "Incremental sync from \(since.formatted(date: .abbreviated, time: .shortened))\u{2026}"
                : "Full historical sync (fetching all data since 2000)\u{2026}"
            syncState.currentOperation = opLabel

            var total = 0
            var failedCategories: [String] = []

            for (cat, types) in HealthDataTypes.quantityTypesByCategory {
                let catID = "qty_\(cat.rawValue)"
                try Task.checkCancellation()

                syncState.updateCategory(catID, status: .syncing)
                var catDelta = 0
                var failedTypes: [String] = []
                for typeDesc in types {
                    try Task.checkCancellation()
                    do {
                        catDelta += try await syncQuantityType(typeDesc: typeDesc, since: querySince)
                    } catch is CancellationError {
                        throw CancellationError()
                    } catch {
                        if isBackgroundSync, (error as? HKError)?.code == .errorDatabaseInaccessible {
                            // Device is locked — silently skip this type, don't mark category as failed
                        } else {
                            failedTypes.append(typeDesc.displayName)
                        }
                    }
                }
                let existing = syncState.categories.first(where: { $0.id == catID })?.recordCount ?? 0
                if failedTypes.isEmpty {
                    syncState.updateCategory(catID, status: .completed, recordCount: existing + catDelta, lastSyncDate: Date())
                } else {
                    failedCategories.append(cat.rawValue)
                    syncState.updateCategory(catID,
                        status: .failed("Failed types: \(failedTypes.joined(separator: ", "))"),
                        recordCount: existing + catDelta, lastSyncDate: Date())
                }
                total += catDelta
                updateLiveActivity(phase: cat.rawValue, operation: "Synced \(cat.rawValue) (\(catDelta) records)", records: total)
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_category", status: .syncing)
            do {
                let catCount = try await syncCategorySamples(since: querySince)
                let existingCat = syncState.categories.first(where: { $0.id == "cat_category" })?.recordCount ?? 0
                syncState.updateCategory("cat_category", status: .completed, recordCount: existingCat + catCount, lastSyncDate: Date())
                total += catCount
                updateLiveActivity(phase: "Health Events", operation: "Synced Health Events (\(catCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Category Samples")
                    syncState.updateCategory("cat_category", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_workouts", status: .syncing)
            do {
                let workoutCount = try await syncWorkouts(since: querySince)
                let existingWorkouts = syncState.categories.first(where: { $0.id == "cat_workouts" })?.recordCount ?? 0
                syncState.updateCategory("cat_workouts", status: .completed, recordCount: existingWorkouts + workoutCount, lastSyncDate: Date())
                total += workoutCount
                updateLiveActivity(phase: "Workouts", operation: "Synced Workouts (\(workoutCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Workouts")
                    syncState.updateCategory("cat_workouts", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_bp", status: .syncing)
            do {
                let bpCount = try await syncBloodPressure(since: querySince)
                let existingBP = syncState.categories.first(where: { $0.id == "cat_bp" })?.recordCount ?? 0
                syncState.updateCategory("cat_bp", status: .completed, recordCount: existingBP + bpCount, lastSyncDate: Date())
                total += bpCount
                updateLiveActivity(phase: "Blood Pressure", operation: "Synced Blood Pressure (\(bpCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Blood Pressure")
                    syncState.updateCategory("cat_bp", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_ecg", status: .syncing)
            do {
                let ecgCount = try await syncECG(since: querySince)
                let existingECG = syncState.categories.first(where: { $0.id == "cat_ecg" })?.recordCount ?? 0
                syncState.updateCategory("cat_ecg", status: .completed, recordCount: existingECG + ecgCount, lastSyncDate: Date())
                total += ecgCount
                updateLiveActivity(phase: "ECG", operation: "Synced ECG (\(ecgCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("ECG")
                    syncState.updateCategory("cat_ecg", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_audiogram", status: .syncing)
            do {
                let audioCount = try await syncAudiograms(since: querySince)
                let existingAudio = syncState.categories.first(where: { $0.id == "cat_audiogram" })?.recordCount ?? 0
                syncState.updateCategory("cat_audiogram", status: .completed, recordCount: existingAudio + audioCount, lastSyncDate: Date())
                total += audioCount
                updateLiveActivity(phase: "Audiograms", operation: "Synced Audiograms (\(audioCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Audiograms")
                    syncState.updateCategory("cat_audiogram", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_activity_summaries", status: .syncing)
            do {
                let activityCount = try await syncActivitySummaries(since: querySince)
                let existingActivity = syncState.categories.first(where: { $0.id == "cat_activity_summaries" })?.recordCount ?? 0
                syncState.updateCategory("cat_activity_summaries", status: .completed, recordCount: existingActivity + activityCount, lastSyncDate: Date())
                total += activityCount
                updateLiveActivity(phase: "Activity Rings", operation: "Synced Activity Rings (\(activityCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Activity Summaries")
                    syncState.updateCategory("cat_activity_summaries", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_workout_routes", status: .syncing)
            do {
                let routeCount = try await syncWorkoutRoutes(since: querySince)
                let existingRoutes = syncState.categories.first(where: { $0.id == "cat_workout_routes" })?.recordCount ?? 0
                syncState.updateCategory("cat_workout_routes", status: .completed, recordCount: existingRoutes + routeCount, lastSyncDate: Date())
                total += routeCount
                updateLiveActivity(phase: "Workout Routes", operation: "Synced Workout Routes (\(routeCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Workout Routes")
                    syncState.updateCategory("cat_workout_routes", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_medications", status: .syncing)
            do {
                let medCount = try await syncMedications(since: querySince)
                let existingMeds = syncState.categories.first(where: { $0.id == "cat_medications" })?.recordCount ?? 0
                syncState.updateCategory("cat_medications", status: .completed, recordCount: existingMeds + medCount, lastSyncDate: Date())
                total += medCount
                updateLiveActivity(phase: "Medications", operation: "Synced Medications (\(medCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Medications")
                    syncState.updateCategory("cat_medications", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_vision", status: .syncing)
            do {
                let visionCount = try await syncVisionPrescriptions(since: querySince)
                let existingVision = syncState.categories.first(where: { $0.id == "cat_vision" })?.recordCount ?? 0
                syncState.updateCategory("cat_vision", status: .completed, recordCount: existingVision + visionCount, lastSyncDate: Date())
                total += visionCount
                updateLiveActivity(phase: "Vision", operation: "Synced Vision Prescriptions (\(visionCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("Vision Prescriptions")
                    syncState.updateCategory("cat_vision", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            try Task.checkCancellation()
            syncState.updateCategory("cat_state_of_mind", status: .syncing)
            do {
                let somCount = try await syncStateOfMind(since: querySince)
                let existingSOM = syncState.categories.first(where: { $0.id == "cat_state_of_mind" })?.recordCount ?? 0
                syncState.updateCategory("cat_state_of_mind", status: .completed, recordCount: existingSOM + somCount, lastSyncDate: Date())
                total += somCount
                updateLiveActivity(phase: "State of Mind", operation: "Synced State of Mind (\(somCount) records)", records: total)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                if !(isBackgroundSync && (error as? HKError)?.code == .errorDatabaseInaccessible) {
                    failedCategories.append("State of Mind")
                    syncState.updateCategory("cat_state_of_mind", status: .failed(error.localizedDescription), lastSyncDate: Date())
                }
            }

            if !failedCategories.isEmpty {
                syncState.errorMessage = "Sync completed with errors in: \(failedCategories.joined(separator: ", "))"
            }

            syncState.lastSyncDate = Date()
            syncState.currentOperation = "Incremental sync done (\(total) records)"
            if !isBackgroundSync { syncState.persist() }
            endLiveActivity(totalRecords: total)
            disconnectFreeReps()

        } catch is CancellationError {
            disconnectFreeReps()
            endLiveActivity(totalRecords: 0)
            syncState.currentOperation = "Sync cancelled"
            if !isBackgroundSync { syncState.persist() }
        } catch {
            disconnectFreeReps()
            endLiveActivity(totalRecords: 0)
            syncState.errorMessage = error.localizedDescription
            syncState.currentOperation = ""
            if !isBackgroundSync { syncState.persist() }
        }

        syncState.isIncrementalSyncRunning = false
    }

    // MARK: - Ingest helper

    /// Safely sends a payload to FreeReps, throwing if the service is not initialized.
    private func ingest(_ payload: FreeRepsPayload) async throws -> IngestResult {
        guard let freereps else {
            throw FreeRepsError.connectionFailed("FreeReps service not initialized")
        }
        return try await freereps.ingest(payload)
    }

    // MARK: - Activity summary sync

    private func syncActivitySummaries(since: Date?, until: Date? = nil) async throws -> Int {
        let summaries = try await healthKit.fetchActivitySummaries(from: since, until: until)
        guard !summaries.isEmpty else { return 0 }
        let calendar = Calendar.current
        var total = 0

        for batch in summaries.chunked(into: batchSize) {
            let records: [FreeRepsActivitySummary] = batch.compactMap { summary in
                guard let date = calendar.date(from: summary.dateComponents(for: calendar)) else { return nil }
                return FreeRepsActivitySummary(
                    date: haeDateOnly(date),
                    active_energy: summary.activeEnergyBurned.doubleValue(for: .kilocalorie()),
                    active_energy_goal: summary.activeEnergyBurnedGoal.doubleValue(for: .kilocalorie()),
                    exercise_time: summary.appleExerciseTime.doubleValue(for: .minute()),
                    exercise_time_goal: summary.appleExerciseTimeGoal.doubleValue(for: .minute()),
                    stand_hours: summary.appleStandHours.doubleValue(for: .count()),
                    stand_hours_goal: summary.appleStandHoursGoal.doubleValue(for: .count())
                )
            }
            guard !records.isEmpty else { continue }
            let payload = FreeRepsPayload(data: FreeRepsData(activity_summaries: records))
            try Task.checkCancellation()
            let result = try await ingest(payload)
            total += result.activity_summaries_inserted ?? batch.count
        }
        return total
    }

    // MARK: - Workout route sync

    private func syncWorkoutRoutes(since: Date?, until: Date? = nil) async throws -> Int {
        var total = 0
        try await healthKit.streamWorkouts(from: since, until: until) { [self] workouts in
            for workout in workouts {
                let routes: [HKWorkoutRoute]
                do { routes = try await healthKit.fetchWorkoutRoutes(for: workout) } catch { continue }
                for route in routes {
                    try Task.checkCancellation()
                    let locations: [CLLocation]
                    do { locations = try await healthKit.fetchRouteLocations(for: route) } catch { continue }
                    guard !locations.isEmpty else { continue }

                    let routePoints = locations.map { loc in
                        FreeRepsRoutePoint(
                            latitude: loc.coordinate.latitude,
                            longitude: loc.coordinate.longitude,
                            altitude: loc.altitude,
                            course: loc.course,
                            courseAccuracy: loc.courseAccuracy,
                            horizontalAccuracy: loc.horizontalAccuracy,
                            verticalAccuracy: loc.verticalAccuracy,
                            timestamp: haeDate(loc.timestamp),
                            speed: loc.speed,
                            speedAccuracy: loc.speedAccuracy
                        )
                    }
                    // Send workout with route data — FreeReps uses ON CONFLICT DO NOTHING for the workout itself
                    let hbWorkout = FreeRepsWorkout(
                        id: workout.uuid.uuidString,
                        name: workout.activityTypeName,
                        start: haeDate(workout.startDate),
                        end: haeDate(workout.endDate),
                        duration: workout.duration,
                        route: routePoints
                    )
                    let payload = FreeRepsPayload(data: FreeRepsData(workouts: [hbWorkout]))
                    _ = try await ingest(payload)
                    total += 1
                }
            }
        }
        return total
    }

    // MARK: - Medication sync

    private func syncMedications(since: Date?, until: Date? = nil) async throws -> Int {
        if #available(iOS 26, *) {
            return try await syncMedicationsIOS26(since: since, until: until)
        }
        return 0
    }

    @available(iOS 26, *)
    private func syncMedicationsIOS26(since: Date?, until: Date? = nil) async throws -> Int {
        var total = 0
        let medications = (try? await healthKit.fetchUserAnnotatedMedications()) ?? []

        if medications.isEmpty {
            let events = try await healthKit.fetchMedicationDoseEvents(from: since, until: until)
            for event in events {
                try Task.checkCancellation()
                total += try await ingestMedicationDoseEvent(event, medicationName: nil)
            }
            return total
        }

        for annotated in medications {
            let concept = annotated.medication
            let conceptPredicate = NSPredicate(
                format: "%K == %@",
                HKPredicateKeyPathMedicationConceptIdentifier,
                concept.identifier
            )
            let events = try await healthKit.fetchMedicationDoseEvents(
                from: since, until: until, additionalPredicate: conceptPredicate
            )
            for event in events {
                try Task.checkCancellation()
                total += try await ingestMedicationDoseEvent(event, medicationName: concept.displayText)
            }
        }
        return total
    }

    @available(iOS 26, *)
    private func ingestMedicationDoseEvent(_ event: HKMedicationDoseEvent, medicationName: String?) async throws -> Int {
        let record = FreeRepsMedication(
            id: event.uuid.uuidString,
            name: medicationName ?? "Unknown",
            dosage: event.doseQuantity.map { "\($0) \(event.unit.unitString)" },
            log_status: logStatusString(event.logStatus),
            start_date: haeDate(event.startDate),
            end_date: haeDate(event.endDate),
            source: event.sourceRevision.source.name
        )
        let payload = FreeRepsPayload(data: FreeRepsData(medications: [record]))
        _ = try await ingest(payload)
        return 1
    }

    @available(iOS 26, *)
    private func logStatusString(_ status: HKMedicationDoseEvent.LogStatus) -> String {
        switch status {
        case .taken:               return "taken"
        case .skipped:             return "skipped"
        case .snoozed:             return "snoozed"
        case .notInteracted:       return "notInteracted"
        case .notificationNotSent: return "notificationNotSent"
        case .notLogged:           return "notLogged"
        @unknown default:          return "unknown"
        }
    }

    // MARK: - Quantity sync

    /// Streams HealthKit samples in pages using cursor-based HKSampleQuery pagination,
    /// inserting each page via FreeReps HTTP before requesting the next. Peak memory stays
    /// flat regardless of total record count.
    private func syncQuantityType(
        typeDesc: QuantityTypeDescriptor,
        since: Date?,
        until: Date? = nil,
        insertBatchSize: Int = batchSize,
        onBatchInserted: ((Int) -> Void)? = nil
    ) async throws -> Int {
        guard let metricName = hkToFreeRepsMetricName[typeDesc.id] else { return 0 }

        // Use on-device aggregation for high-frequency discrete types (e.g. heart rate).
        if case .aggregate(let interval) = typeDesc.syncStrategy {
            do {
                return try await syncQuantityTypeAggregated(
                    typeDesc: typeDesc, metricName: metricName, interval: interval,
                    since: since, until: until, insertBatchSize: insertBatchSize,
                    onBatchInserted: onBatchInserted
                )
            } catch {
                print("Aggregation failed for \(metricName), falling back to individual samples: \(error.localizedDescription)")
            }
        }

        // Use cumulative SUM aggregation for step/energy/distance types.
        if case .aggregateCumulative(let interval) = typeDesc.syncStrategy {
            do {
                return try await syncQuantityTypeCumulative(
                    typeDesc: typeDesc, metricName: metricName, interval: interval,
                    since: since, until: until, insertBatchSize: insertBatchSize,
                    onBatchInserted: onBatchInserted
                )
            } catch {
                print("Cumulative agg failed for \(metricName), falling back to individual samples: \(error.localizedDescription)")
            }
        }

        // Individual samples path — skip empty windows to avoid unnecessary streaming.
        if let start = since, let end = until, let hkType = typeDesc.hkType {
            if !(await healthKit.sampleExists(for: hkType, from: start, to: end)) { return 0 }
        }

        var total = 0
        try await healthKit.streamQuantitySamples(typeID: typeDesc.hkIdentifier, from: since, until: until) { hkBatch in
            for batch in hkBatch.chunked(into: insertBatchSize) {
                let points = batch.map { s in
                    FreeRepsMetricDataPoint(
                        date: haeDate(s.startDate),
                        qty: s.quantity.doubleValue(for: typeDesc.unit),
                        source_uuid: s.uuid.uuidString
                    )
                }
                let metric = FreeRepsMetric(name: metricName, units: typeDesc.unitString, data: points)
                let payload = FreeRepsPayload(data: FreeRepsData(metrics: [metric]))
                try Task.checkCancellation()
                let result = try await ingest(payload)
                total += result.metrics_inserted ?? batch.count
                onBatchInserted?(total)
            }
        }
        return total
    }

    private func syncQuantityTypeAggregated(
        typeDesc: QuantityTypeDescriptor,
        metricName: String,
        interval: TimeInterval,
        since: Date?,
        until: Date?,
        insertBatchSize: Int,
        onBatchInserted: ((Int) -> Void)?
    ) async throws -> Int {
        let start = since ?? Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
        let end = until ?? Date()

        let buckets = try await healthKit.queryAggregatedStatistics(
            typeID: typeDesc.hkIdentifier,
            unit: typeDesc.unit,
            from: start, until: end,
            interval: interval
        )

        var total = 0
        for batch in buckets.chunked(into: insertBatchSize) {
            let points = batch.map { b in
                FreeRepsMetricDataPoint(
                    date: haeDate(b.startDate),
                    Min: b.min, Avg: b.avg, Max: b.max
                )
            }
            let metric = FreeRepsMetric(name: metricName, units: typeDesc.unitString, data: points)
            let payload = FreeRepsPayload(data: FreeRepsData(metrics: [metric]))
            try Task.checkCancellation()
            let result = try await ingest(payload)
            total += result.metrics_inserted ?? batch.count
            onBatchInserted?(total)
        }
        return total
    }

    private func syncQuantityTypeCumulative(
        typeDesc: QuantityTypeDescriptor,
        metricName: String,
        interval: TimeInterval,
        since: Date?,
        until: Date?,
        insertBatchSize: Int,
        onBatchInserted: ((Int) -> Void)?
    ) async throws -> Int {
        let start = since ?? Calendar.current.date(from: DateComponents(year: 2000, month: 1, day: 1))!
        let end = until ?? Date()

        let buckets = try await healthKit.queryCumulativeStatistics(
            typeID: typeDesc.hkIdentifier,
            unit: typeDesc.unit,
            from: start, until: end,
            interval: interval
        )

        var total = 0
        for batch in buckets.chunked(into: insertBatchSize) {
            let points = batch.map { b in
                FreeRepsMetricDataPoint(
                    date: haeDate(b.startDate),
                    qty: b.sum
                )
            }
            let metric = FreeRepsMetric(name: metricName, units: typeDesc.unitString, data: points)
            let payload = FreeRepsPayload(data: FreeRepsData(metrics: [metric]))
            try Task.checkCancellation()
            let result = try await ingest(payload)
            total += result.metrics_inserted ?? batch.count
            onBatchInserted?(total)
        }
        return total
    }

    // MARK: - Category sync

    private func syncCategorySamples(since: Date?, until: Date? = nil, insertBatchSize: Int = batchSize) async throws -> Int {
        var total = 0
        for typeDesc in HealthDataTypes.allCategoryTypes {
            try await healthKit.streamCategorySamples(typeID: typeDesc.hkIdentifier, from: since, until: until) { hkBatch in
                for batch in hkBatch.chunked(into: insertBatchSize) {
                    let samples = batch.map { s in
                        FreeRepsCategorySample(
                            id: s.uuid.uuidString,
                            type: typeDesc.id,
                            value: s.value,
                            value_label: typeDesc.valueLabels[s.value],
                            start_date: haeDate(s.startDate),
                            end_date: haeDate(s.endDate),
                            source: s.sourceDisplayName
                        )
                    }
                    let payload = FreeRepsPayload(data: FreeRepsData(category_samples: samples))
                    try Task.checkCancellation()
                    let result = try await ingest(payload)
                    total += result.category_samples_inserted ?? batch.count
                }
            }
        }
        return total
    }

    // MARK: - Workout sync

    private func syncWorkouts(since: Date?, until: Date? = nil) async throws -> Int {
        var total = 0
        let hrUnit = HKUnit(from: "count/min")
        try await healthKit.streamWorkouts(from: since, until: until) { workouts in
            for batch in workouts.chunked(into: batchSize) {
                var hbWorkouts: [FreeRepsWorkout] = []
                for w in batch {
                    // Query per-minute HR aggregates for this workout's time window
                    var hrData: [FreeRepsWorkoutHRPoint]?
                    if w.duration > 0 {
                        let buckets = try await self.healthKit.queryAggregatedStatistics(
                            typeID: .heartRate, unit: hrUnit,
                            from: w.startDate, until: w.endDate,
                            interval: 60 // 1-minute buckets, matching HAE format
                        )
                        if !buckets.isEmpty {
                            hrData = buckets.map { b in
                                FreeRepsWorkoutHRPoint(
                                    date: haeDate(b.startDate),
                                    Min: b.min, Avg: b.avg, Max: b.max,
                                    units: "bpm",
                                    source: w.sourceDisplayName
                                )
                            }
                        }
                    }

                    let activeEnergy = w.statistics(for: HKQuantityType(.activeEnergyBurned))?.sumQuantity()

                    // Location type (indoor/outdoor)
                    let locationType = w.workoutActivities.first?.workoutConfiguration.locationType
                    let isIndoor = locationType == .indoor ? true : locationType == .outdoor ? false : nil
                    let location = locationType == .indoor ? "Indoor" : locationType == .outdoor ? "Outdoor" : nil

                    // Elevation from workout metadata
                    let elevUp = (w.metadata?[HKMetadataKeyElevationAscended] as? HKQuantity)
                        .map { FreeRepsQuantity(qty: $0.doubleValue(for: .meter()), units: "m") }
                    let elevDown = (w.metadata?[HKMetadataKeyElevationDescended] as? HKQuantity)
                        .map { FreeRepsQuantity(qty: $0.doubleValue(for: .meter()), units: "m") }

                    // HR summary from per-minute buckets
                    var hrSummary: FreeRepsHRSummary?
                    if let hrs = hrData, !hrs.isEmpty {
                        let count = Double(hrs.count)
                        let avgBPM = hrs.reduce(0.0) { $0 + $1.Avg } / count
                        let maxBPM = hrs.map(\.Max).max()!
                        let minBPM = hrs.map(\.Min).min()!
                        hrSummary = FreeRepsHRSummary(
                            min: FreeRepsQuantity(qty: minBPM, units: "bpm"),
                            avg: FreeRepsQuantity(qty: avgBPM, units: "bpm"),
                            max: FreeRepsQuantity(qty: maxBPM, units: "bpm")
                        )
                    }

                    hbWorkouts.append(FreeRepsWorkout(
                        id: w.uuid.uuidString,
                        name: w.activityTypeName,
                        start: haeDate(w.startDate),
                        end: haeDate(w.endDate),
                        duration: w.duration,
                        location: location,
                        isIndoor: isIndoor,
                        activeEnergyBurned: activeEnergy.map { FreeRepsQuantity(qty: $0.doubleValue(for: .kilocalorie()), units: "kcal") },
                        distance: w.totalDistance.map { FreeRepsQuantity(qty: $0.doubleValue(for: .meter()), units: "m") },
                        elevationUp: elevUp,
                        elevationDown: elevDown,
                        heartRate: hrSummary,
                        heartRateData: hrData
                    ))
                }
                let payload = FreeRepsPayload(data: FreeRepsData(workouts: hbWorkouts))
                try Task.checkCancellation()
                let result = try await ingest(payload)
                total += result.workouts_inserted ?? batch.count
            }
        }
        return total
    }

    // MARK: - Blood pressure sync

    private func syncBloodPressure(since: Date?, until: Date? = nil) async throws -> Int {
        let correlations = try await healthKit.fetchBloodPressure(from: since, until: until)
        guard !correlations.isEmpty else { return 0 }

        let systolicType = HKObjectType.quantityType(forIdentifier: .bloodPressureSystolic)!
        let diastolicType = HKObjectType.quantityType(forIdentifier: .bloodPressureDiastolic)!
        var total = 0

        for batch in correlations.chunked(into: batchSize) {
            // Send systolic and diastolic as separate metrics
            var sysPoints: [FreeRepsMetricDataPoint] = []
            var diaPoints: [FreeRepsMetricDataPoint] = []
            for corr in batch {
                guard let sys = (corr.objects(for: systolicType) as? Set<HKQuantitySample>)?.first,
                      let dia = (corr.objects(for: diastolicType) as? Set<HKQuantitySample>)?.first else { continue }
                sysPoints.append(FreeRepsMetricDataPoint(date: haeDate(corr.startDate), qty: sys.quantity.doubleValue(for: .millimeterOfMercury()), source_uuid: corr.uuid.uuidString))
                diaPoints.append(FreeRepsMetricDataPoint(date: haeDate(corr.startDate), qty: dia.quantity.doubleValue(for: .millimeterOfMercury()), source_uuid: corr.uuid.uuidString))
            }
            if sysPoints.isEmpty { continue }
            let metrics = [
                FreeRepsMetric(name: "blood_pressure_systolic", units: "mmHg", data: sysPoints),
                FreeRepsMetric(name: "blood_pressure_diastolic", units: "mmHg", data: diaPoints),
            ]
            let payload = FreeRepsPayload(data: FreeRepsData(metrics: metrics))
            let result = try await ingest(payload)
            total += result.metrics_inserted ?? sysPoints.count
        }
        return total
    }

    // MARK: - ECG sync

    private func syncECG(since: Date?, until: Date? = nil) async throws -> Int {
        let recordings = try await healthKit.fetchECG(from: since, until: until)
        guard !recordings.isEmpty else { return 0 }

        var total = 0
        // ECG recordings include voltage data, so batch more conservatively
        for batch in recordings.chunked(into: 50) {
            var items: [FreeRepsECG] = []
            for ecg in batch {
                let voltages = try await healthKit.fetchECGVoltageMeasurements(for: ecg)
                let mvUnit = HKUnit(from: "mV")
                let voltageArray = voltages.compactMap { v -> Double? in
                    v.quantity(for: .appleWatchSimilarToLeadI)?.doubleValue(for: mvUnit)
                }

                items.append(FreeRepsECG(
                    id: ecg.uuid.uuidString,
                    classification: ecg.classification.label,
                    average_heart_rate: ecg.averageHeartRate?.doubleValue(for: HKUnit(from: "count/min")),
                    sampling_frequency: ecg.samplingFrequency?.doubleValue(for: HKUnit(from: "Hz")),
                    voltage_measurements: voltageArray.isEmpty ? nil : voltageArray,
                    start_date: haeDate(ecg.startDate),
                    source: ecg.sourceRevision.source.name
                ))
            }
            guard !items.isEmpty else { continue }
            try Task.checkCancellation()
            let payload = FreeRepsPayload(data: FreeRepsData(ecg_recordings: items))
            let result = try await ingest(payload)
            total += result.ecg_recordings_inserted ?? items.count
        }
        return total
    }

    // MARK: - Audiogram sync

    private func syncAudiograms(since: Date?, until: Date? = nil) async throws -> Int {
        let audiograms = try await healthKit.fetchAudiograms(from: since, until: until)
        guard !audiograms.isEmpty else { return 0 }

        var total = 0
        for batch in audiograms.chunked(into: batchSize) {
            let items: [FreeRepsAudiogram] = batch.map { ag in
                let points = ag.sensitivityPoints.map { pt in
                    AudiogramPoint(
                        hz: pt.frequency.doubleValue(for: .hertz()),
                        left_db: pt.leftEarSensitivity?.doubleValue(for: HKUnit.decibelHearingLevel()),
                        right_db: pt.rightEarSensitivity?.doubleValue(for: HKUnit.decibelHearingLevel())
                    )
                }
                return FreeRepsAudiogram(
                    id: ag.uuid.uuidString,
                    sensitivity_points: points,
                    start_date: haeDate(ag.startDate),
                    source: ag.sourceRevision.source.name
                )
            }
            try Task.checkCancellation()
            let payload = FreeRepsPayload(data: FreeRepsData(audiograms: items))
            let result = try await ingest(payload)
            total += result.audiograms_inserted ?? items.count
        }
        return total
    }

    // MARK: - Vision prescription sync

    private func syncVisionPrescriptions(since: Date?, until: Date? = nil) async throws -> Int {
        let prescriptions = try await healthKit.fetchVisionPrescriptions(from: since, until: until)
        guard !prescriptions.isEmpty else { return 0 }

        let diopterUnit = HKUnit(from: "D")
        let degreeUnit = HKUnit.count()
        let mmUnit = HKUnit.meterUnit(with: .milli)

        var total = 0
        for batch in prescriptions.chunked(into: batchSize) {
            let items: [FreeRepsVisionPrescription] = batch.map { p in
                var rightEye: [String: Double]?
                var leftEye: [String: Double]?

                if let glasses = p as? HKGlassesPrescription {
                    if let r = glasses.rightEye {
                        var eye: [String: Double] = ["sphere": r.sphere.doubleValue(for: diopterUnit)]
                        if let c = r.cylinder { eye["cylinder"] = c.doubleValue(for: diopterUnit) }
                        if let a = r.axis { eye["axis"] = a.doubleValue(for: degreeUnit) }
                        if let add = r.addPower { eye["add"] = add.doubleValue(for: diopterUnit) }
                        rightEye = eye
                    }
                    if let l = glasses.leftEye {
                        var eye: [String: Double] = ["sphere": l.sphere.doubleValue(for: diopterUnit)]
                        if let c = l.cylinder { eye["cylinder"] = c.doubleValue(for: diopterUnit) }
                        if let a = l.axis { eye["axis"] = a.doubleValue(for: degreeUnit) }
                        if let add = l.addPower { eye["add"] = add.doubleValue(for: diopterUnit) }
                        leftEye = eye
                    }
                } else if let contacts = p as? HKContactsPrescription {
                    if let r = contacts.rightEye {
                        var eye: [String: Double] = ["sphere": r.sphere.doubleValue(for: diopterUnit)]
                        if let c = r.cylinder { eye["cylinder"] = c.doubleValue(for: diopterUnit) }
                        if let a = r.axis { eye["axis"] = a.doubleValue(for: degreeUnit) }
                        if let add = r.addPower { eye["add"] = add.doubleValue(for: diopterUnit) }
                        if let bc = r.baseCurve { eye["base_curve"] = bc.doubleValue(for: mmUnit) }
                        if let d = r.diameter { eye["diameter"] = d.doubleValue(for: mmUnit) }
                        rightEye = eye
                    }
                    if let l = contacts.leftEye {
                        var eye: [String: Double] = ["sphere": l.sphere.doubleValue(for: diopterUnit)]
                        if let c = l.cylinder { eye["cylinder"] = c.doubleValue(for: diopterUnit) }
                        if let a = l.axis { eye["axis"] = a.doubleValue(for: degreeUnit) }
                        if let add = l.addPower { eye["add"] = add.doubleValue(for: diopterUnit) }
                        if let bc = l.baseCurve { eye["base_curve"] = bc.doubleValue(for: mmUnit) }
                        if let d = l.diameter { eye["diameter"] = d.doubleValue(for: mmUnit) }
                        leftEye = eye
                    }
                }

                let prescType: String?
                switch p.prescriptionType {
                case .glasses: prescType = "glasses"
                case .contacts: prescType = "contacts"
                @unknown default: prescType = nil
                }

                return FreeRepsVisionPrescription(
                    id: p.uuid.uuidString,
                    date_issued: haeDate(p.startDate),
                    expiration_date: p.expirationDate.map { haeDate($0) },
                    prescription_type: prescType,
                    right_eye: rightEye,
                    left_eye: leftEye,
                    source: p.sourceRevision.source.name
                )
            }
            try Task.checkCancellation()
            let payload = FreeRepsPayload(data: FreeRepsData(vision_prescriptions: items))
            let result = try await ingest(payload)
            total += result.vision_prescriptions_inserted ?? items.count
        }
        return total
    }

    // MARK: - State of Mind sync

    private func syncStateOfMind(since: Date?, until: Date? = nil) async throws -> Int {
        if #available(iOS 18, *) {
            return try await syncStateOfMindIOS18(since: since, until: until)
        }
        return 0
    }

    @available(iOS 18, *)
    private func syncStateOfMindIOS18(since: Date?, until: Date? = nil) async throws -> Int {
        let samples = try await healthKit.fetchStateOfMind(from: since, until: until)
        guard !samples.isEmpty else { return 0 }

        var total = 0
        for batch in samples.chunked(into: batchSize) {
            let items: [FreeRepsStateOfMind] = batch.map { sample in
                FreeRepsStateOfMind(
                    id: sample.uuid.uuidString,
                    kind: sample.kind.rawValue,
                    valence: sample.valence,
                    labels: sample.labels.map { $0.rawValue },
                    associations: sample.associations.map { $0.rawValue },
                    start_date: haeDate(sample.startDate),
                    source: sample.sourceRevision.source.name
                )
            }
            try Task.checkCancellation()
            let payload = FreeRepsPayload(data: FreeRepsData(state_of_mind: items))
            let result = try await ingest(payload)
            total += result.state_of_mind_inserted ?? items.count
        }
        return total
    }
}

// MARK: - Sync prerequisite issues

enum SyncPrerequisiteIssue: Identifiable {
    case healthDataUnavailable
    case healthPermissionsNotRequested
    case somePermissionsDenied(count: Int)
    case connectionFailed(String)

    var id: String {
        switch self {
        case .healthDataUnavailable: return "healthUnavailable"
        case .healthPermissionsNotRequested: return "permissionsNotRequested"
        case .somePermissionsDenied: return "permissionsDenied"
        case .connectionFailed: return "connectionFailed"
        }
    }

    var title: String {
        switch self {
        case .healthDataUnavailable:
            return "Health Data Unavailable"
        case .healthPermissionsNotRequested:
            return "Health Permissions Not Requested"
        case .somePermissionsDenied(let count):
            return "\(count) Health Permission(s) Denied"
        case .connectionFailed:
            return "FreeReps Connection Failed"
        }
    }

    var message: String {
        switch self {
        case .healthDataUnavailable:
            return "HealthKit is not available on this device."
        case .healthPermissionsNotRequested:
            return "Go to Settings \u{2192} Apple Health Permissions and request access to sync all your health data."
        case .somePermissionsDenied:
            return "Some health data types were denied. Go to Settings \u{2192} Health Permissions to review and re-request missing permissions."
        case .connectionFailed(let err):
            return "Could not connect to FreeReps: \(err). Check your connection settings."
        }
    }

    var actionLabel: String {
        switch self {
        case .healthDataUnavailable: return ""
        case .healthPermissionsNotRequested: return "Review Permissions"
        case .somePermissionsDenied: return "Review Permissions"
        case .connectionFailed: return "Check Settings"
        }
    }
}

// MARK: - Array chunking

extension Array {
    func chunked(into size: Int) -> [[Element]] {
        stride(from: 0, to: count, by: size).map {
            Array(self[$0..<Swift.min($0 + size, count)])
        }
    }
}
