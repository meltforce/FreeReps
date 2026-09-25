import ActivityKit
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
        "cat_state_of_mind"
    ]

    /// Set by the caller before `runHistoricalBackfill` so the background-task expiry
    /// handler can cancel the Swift Task when iOS reclaims background time.
    var taskForCancellation: Task<Void, Never>?

    /// The "Keep Screen On" setting; on unless switched off. HealthKit rejects reads
    /// once the device locks, so an awake display keeps a long sync readable.
    static var keepScreenOn: Bool {
        UserDefaults.standard.object(forKey: "keepScreenOnDuringSync") as? Bool ?? true
    }

    // Class-level flag: true while any SyncService instance is syncing.
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
            ("cat_activity_summaries", "Activity Rings", "chart.bar.fill"),
            ("cat_workout_routes", "Workout Routes", "map.fill"),
            ("cat_state_of_mind", "State of Mind", "brain.head.profile"),
            ("cat_strength", "Weight Training", "figure.strengthtraining.traditional"),
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

    /// Metric names the server would reject for this user; loaded after each connect.
    private var disabledMetrics: Set<String> = []

    private func loadDisabledMetrics() async throws {
        guard let freereps else { return }
        disabledMetrics = try await freereps.fetchDisabledMetrics()
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

    /// The one entry point for every sync. After a completed backfill only what
    /// HealthKit reports as added is sent; until then the windowed backfill runs,
    /// or resumes.
    func runFullSync(config: FreeRepsConfig) async {
        if syncState.hasCompletedFullSync {
            await runAnchoredSync(config: config)
        } else {
            await runHistoricalBackfill(config: config)
        }
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
            try await loadDisabledMetrics()

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

        UIApplication.shared.isIdleTimerDisabled = Self.keepScreenOn
        defer { UIApplication.shared.isIdleTimerDisabled = false }

        // Extra time when the user leaves the app mid-sync. On expiry the sync is cancelled
        // and its cursors persisted; the next run resumes from them.
        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "health-full-sync") {
            self.taskForCancellation?.cancel()
            self.syncState.persist()
            UIApplication.shared.endBackgroundTask(bgTaskID)
            bgTaskID = .invalid
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
            }
        }

        let earliest = config.backfillStartDate
        let historicalStart: Date

        if syncState.backfillAnchorDate != nil {
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
            try await loadDisabledMetrics()

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

    // MARK: - Anchored sync

    /// A HealthKit sample type the anchored sync tracks, and how to re-send a time
    /// range of it.
    private struct AnchoredTarget {
        let id: String
        let name: String
        let categoryID: String
        let type: HKSampleType
        /// Bucket length the type is aggregated into on the way out; a changed range is
        /// widened to whole buckets so a bucket is always re-sent complete.
        let bucket: TimeInterval?
        /// Metric names the server may have disabled; the target is skipped when all are.
        let metrics: [String]
        let sync: (Date, Date) async throws -> Int
    }

    private func anchoredTargets() -> [AnchoredTarget] {
        var targets: [AnchoredTarget] = []
        for desc in HealthDataTypes.allQuantityTypes {
            guard let type = desc.hkType else { continue }
            let bucket: TimeInterval?
            switch desc.syncStrategy {
            case .individual: bucket = nil
            case .aggregate(let interval), .aggregateCumulative(let interval): bucket = interval
            }
            targets.append(AnchoredTarget(
                id: desc.id, name: desc.displayName, categoryID: "qty_\(desc.category.rawValue)",
                type: type, bucket: bucket, metrics: hkToFreeRepsMetricName[desc.id].map { [$0] } ?? [],
                sync: { [self] start, end in try await syncQuantityType(typeDesc: desc, since: start, until: end) }
            ))
        }
        for desc in HealthDataTypes.allCategoryTypes {
            guard let type = desc.hkType else { continue }
            let isSleep = desc.id == HKCategoryTypeIdentifier.sleepAnalysis.rawValue
            targets.append(AnchoredTarget(
                id: desc.id, name: desc.displayName, categoryID: "cat_category",
                type: type, bucket: nil, metrics: isSleep ? ["sleep_analysis"] : [],
                sync: { [self] start, end in try await syncCategorySamples(since: start, until: end, types: [desc]) }
            ))
        }
        targets.append(AnchoredTarget(
            id: "workouts", name: "Workouts", categoryID: "cat_workouts",
            type: HKObjectType.workoutType(), bucket: nil, metrics: [],
            sync: { [self] start, end in try await syncWorkouts(since: start, until: end) }
        ))
        targets.append(AnchoredTarget(
            id: "workout_routes", name: "Workout Routes", categoryID: "cat_workout_routes",
            type: HKSeriesType.workoutRoute(), bucket: nil, metrics: [],
            sync: { [self] start, end in try await syncWorkoutRoutes(since: start, until: end) }
        ))
        if let bp = HKObjectType.correlationType(forIdentifier: .bloodPressure) {
            targets.append(AnchoredTarget(
                id: "blood_pressure", name: "Blood Pressure", categoryID: "cat_bp",
                type: bp, bucket: nil, metrics: ["blood_pressure_systolic", "blood_pressure_diastolic"],
                sync: { [self] start, end in try await syncBloodPressure(since: start, until: end) }
            ))
        }
        if #available(iOS 18, *) {
            targets.append(AnchoredTarget(
                id: "state_of_mind", name: "State of Mind", categoryID: "cat_state_of_mind",
                type: HKObjectType.stateOfMindType(), bucket: nil, metrics: [],
                sync: { [self] start, end in try await syncStateOfMind(since: start, until: end) }
            ))
        }
        return targets
    }

    /// Widens each span to whole buckets (aligned to local midnight, as the statistics
    /// queries are) and merges spans less than an hour apart, so a burst of samples
    /// becomes one query instead of hundreds.
    static func mergedRanges(_ spans: [DateInterval], bucket: TimeInterval?) -> [DateInterval] {
        let calendar = Calendar.current
        func floorToBucket(_ date: Date) -> Date {
            guard let bucket else { return date }
            let day = calendar.startOfDay(for: date)
            return day.addingTimeInterval(floor(date.timeIntervalSince(day) / bucket) * bucket)
        }
        let widened = spans.map { span -> DateInterval in
            let start = floorToBucket(span.start)
            // The range end is exclusive; one second past the last start keeps an
            // individual sample, and a full bucket past the floor keeps an aggregate.
            let end = bucket.map { floorToBucket(span.end).addingTimeInterval($0) } ?? span.end.addingTimeInterval(1)
            return DateInterval(start: start, end: max(end, start.addingTimeInterval(1)))
        }.sorted { $0.start < $1.start }

        var merged: [DateInterval] = []
        for range in widened {
            if let last = merged.last, range.start <= last.end.addingTimeInterval(3600) {
                merged[merged.count - 1] = DateInterval(start: last.start, end: max(last.end, range.end))
            } else {
                merged.append(range)
            }
        }
        return merged
    }

    /// How far back an added sample may be dated and still be sent. A sample HealthKit
    /// receives later than this after its own time is not sent — the same bound the
    /// 7-day re-send had. It also bounds the first anchored sync of a type, which has
    /// no anchor yet and reads this window once.
    static let anchoredLookback: TimeInterval = 7 * 24 * 3600

    /// Sends what HealthKit added since each type's anchor, then stores the new anchor.
    /// Once a day the last 24 hours are re-sent in full as well, for changes an anchor
    /// does not report (a deleted sample, an anchor lost to a crash between send and save).
    func runAnchoredSync(config: FreeRepsConfig) async {
        guard !syncState.isAnySyncRunning else { return }
        syncState.isFullSyncRunning = true
        SyncService.isSyncRunning = true
        defer { SyncService.isSyncRunning = false }
        syncState.errorMessage = nil
        syncState.currentOperation = "Connecting\u{2026}"
        startLiveActivity(isFullSync: false)

        UIApplication.shared.isIdleTimerDisabled = Self.keepScreenOn
        defer { UIApplication.shared.isIdleTimerDisabled = false }

        var bgTaskID: UIBackgroundTaskIdentifier = .invalid
        bgTaskID = UIApplication.shared.beginBackgroundTask(withName: "health-anchored-sync") {
            self.taskForCancellation?.cancel()
            UIApplication.shared.endBackgroundTask(bgTaskID)
            bgTaskID = .invalid
        }
        defer {
            if bgTaskID != .invalid {
                UIApplication.shared.endBackgroundTask(bgTaskID)
            }
        }

        let now = Date()
        let dailyWindow: DateInterval? = {
            if let last = SyncAnchors.lastDailyResend, now.timeIntervalSince(last) < 24 * 3600 { return nil }
            return DateInterval(start: now.addingTimeInterval(-24 * 3600), end: now)
        }()
        let anchors = SyncAnchors.load()

        do {
            connectFreeReps(config: config)
            guard freereps != nil else { throw FreeRepsError.connectionFailed("FreeReps not initialized") }
            try await loadDisabledMetrics()

            var failed: [String] = []
            var total = 0
            for target in anchoredTargets() {
                try Task.checkCancellation()
                // A disabled type keeps its anchor, so re-enabling it sends what was
                // added in the meantime.
                if !target.metrics.isEmpty, target.metrics.allSatisfy({ disabledMetrics.contains($0) }) { continue }
                syncState.currentOperation = "Syncing \(target.name)\u{2026}"
                do {
                    let (spans, newAnchor) = try await healthKit.addedSampleSpans(
                        for: target.type, since: anchors[target.id],
                        notBefore: now.addingTimeInterval(-Self.anchoredLookback)
                    )
                    let ranges = Self.mergedRanges(spans + (dailyWindow.map { [$0] } ?? []), bucket: target.bucket)
                    if !spans.isEmpty {
                        print("Anchored sync: \(target.name): \(spans.count) added samples, \(ranges.count) range(s)")
                    }
                    var count = 0
                    for range in ranges {
                        try Task.checkCancellation()
                        count += try await target.sync(range.start, range.end)
                    }
                    if let newAnchor { SyncAnchors.save(newAnchor, for: target.id) }
                    total += count
                    if count > 0 {
                        let existing = syncState.categories.first(where: { $0.id == target.categoryID })?.recordCount ?? 0
                        syncState.updateCategory(target.categoryID, status: .completed, recordCount: existing + count, lastSyncDate: now)
                        updateLiveActivity(phase: target.name, operation: "Synced \(target.name)", records: total)
                    }
                } catch is CancellationError {
                    throw CancellationError()
                } catch {
                    failed.append(target.name)
                    syncState.updateCategory(target.categoryID, status: .failed(error.localizedDescription))
                    print("Anchored sync failed for \(target.name): \(error.localizedDescription)")
                }
            }

            // Activity summaries are not samples and have no anchor; today and
            // yesterday are two rows.
            do {
                let yesterday = Calendar.current.startOfDay(for: now.addingTimeInterval(-24 * 3600))
                total += try await syncActivitySummaries(since: yesterday, until: now)
            } catch is CancellationError {
                throw CancellationError()
            } catch {
                failed.append("Activity Rings")
            }

            if failed.isEmpty, dailyWindow != nil { SyncAnchors.lastDailyResend = now }
            syncState.lastSyncDate = now
            if failed.isEmpty {
                syncState.currentOperation = "Synced \(total.formatted()) records"
            } else {
                syncState.errorMessage = "Sync failed for: \(failed.joined(separator: ", ")). They are retried on the next sync."
                syncState.currentOperation = ""
            }
            syncState.persist()
            endLiveActivity(totalRecords: total)
            disconnectFreeReps()
        } catch is CancellationError {
            disconnectFreeReps()
            endLiveActivity(totalRecords: 0)
            syncState.currentOperation = "Sync cancelled"
            syncState.persist()
        } catch {
            disconnectFreeReps()
            endLiveActivity(totalRecords: 0)
            syncState.errorMessage = error.localizedDescription
            syncState.currentOperation = ""
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
        guard let metricName = hkToFreeRepsMetricName[typeDesc.id],
              !disabledMetrics.contains(metricName) else { return 0 }

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
        // No fallback to individual samples here: the server would then hold hourly
        // sums and raw samples under the same source, and a daily total counts both.
        if case .aggregateCumulative(let interval) = typeDesc.syncStrategy {
            return try await syncQuantityTypeCumulative(
                typeDesc: typeDesc, metricName: metricName, interval: interval,
                since: since, until: until, insertBatchSize: insertBatchSize,
                onBatchInserted: onBatchInserted
            )
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

        let buckets: [HealthKitService.CumulativeBucket]
        do {
            buckets = try await healthKit.queryCumulativeStatistics(
                typeID: typeDesc.hkIdentifier,
                unit: typeDesc.unit,
                from: start, until: end,
                interval: interval
            )
        } catch {
            print("Cumulative statistics failed for \(metricName), querying per bucket: \(error.localizedDescription)")
            buckets = try await healthKit.queryCumulativeStatisticsPerBucket(
                typeID: typeDesc.hkIdentifier,
                unit: typeDesc.unit,
                from: start, until: end,
                interval: interval
            )
        }

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

    private func syncCategorySamples(
        since: Date?, until: Date? = nil, insertBatchSize: Int = batchSize,
        types: [CategoryTypeDescriptor] = HealthDataTypes.allCategoryTypes
    ) async throws -> Int {
        var total = 0
        for typeDesc in types {
            // Sleep stages arrive as category samples but are governed by the sleep_analysis metric.
            if typeDesc.id == HKCategoryTypeIdentifier.sleepAnalysis.rawValue,
               disabledMetrics.contains("sleep_analysis") { continue }
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
                        let buckets: [HealthKitService.AggregatedBucket]
                        do {
                            buckets = try await self.healthKit.queryAggregatedStatistics(
                                typeID: .heartRate, unit: hrUnit,
                                from: w.startDate, until: w.endDate,
                                interval: 60 // 1-minute buckets, matching HAE format
                            )
                        } catch {
                            print("HR statistics failed for workout \(w.uuid), aggregating samples: \(error.localizedDescription)")
                            buckets = try await self.healthKit.aggregateSamples(
                                typeID: .heartRate, unit: hrUnit,
                                from: w.startDate, until: w.endDate,
                                interval: 60
                            )
                        }
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
        if disabledMetrics.contains("blood_pressure_systolic"),
           disabledMetrics.contains("blood_pressure_diastolic") { return 0 }
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
