import CoreLocation
import Foundation
import HealthKit

final class HealthKitService {

    static let shared = HealthKitService()
    let store = HKHealthStore()

    private init() {}

    // MARK: - Availability

    var isAvailable: Bool { HKHealthStore.isHealthDataAvailable() }

    // MARK: - Authorization

    func requestAllPermissions() async throws {
        guard isAvailable else { throw HKError(.errorHealthDataUnavailable) }
        let readTypes = HealthDataTypes.allReadTypes
        try await store.requestAuthorization(toShare: [], read: readTypes)
    }

    func authorizationStatus(for type: HKObjectType) -> HKAuthorizationStatus {
        store.authorizationStatus(for: type)
    }

    /// Request authorization for a specific subset of read types.
    func requestPermissions(for types: Set<HKObjectType>) async throws {
        guard isAvailable else { throw HKError(.errorHealthDataUnavailable) }
        try await store.requestAuthorization(toShare: [], read: types)
    }

    /// Check authorization status for all requested types.
    /// Returns two arrays: "processed" types and "not yet requested" types.
    ///
    /// HealthKit's `authorizationStatus(for:)` only tracks *write* authorization.
    /// Since this app requests read-only access (`toShare: []`), the statuses mean:
    ///   - `.notDetermined`   → HealthKit dialog has never been shown for this type (needs requesting)
    ///   - `.sharingDenied`   → dialog was shown and user went through it; read grant/deny is hidden by iOS
    ///   - `.sharingAuthorized` → write was also granted (not expected here)
    ///
    /// After the user approves the HealthKit dialog, all read-only types transition
    /// from `.notDetermined` to `.sharingDenied`. Treating `.sharingDenied` as "denied"
    /// is therefore incorrect — it just means the dialog was already shown.
    ///
    /// "denied" here means `.notDetermined` (never shown the dialog), which is the
    /// only case where calling `requestAuthorization` will actually surface the iOS prompt.
    func checkAllPermissionStatuses() -> (granted: [HKObjectType], denied: [HKObjectType]) {
        let allTypes = HealthDataTypes.allReadTypes
        var granted: [HKObjectType] = []
        var denied: [HKObjectType] = []
        for type in allTypes {
            let status = store.authorizationStatus(for: type)
            if status == .notDetermined {
                denied.append(type)
            } else {
                granted.append(type)
            }
        }
        return (granted, denied)
    }

    // MARK: - Quantity Samples

    func fetchQuantitySamples(
        typeID: HKQuantityTypeIdentifier,
        unit: HKUnit,
        from startDate: Date? = nil,
        limit: Int = HKObjectQueryNoLimit,
        ascending: Bool = true
    ) async throws -> [HKQuantitySample] {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return [] }

        let predicate: NSPredicate?
        if let start = startDate {
            predicate = HKQuery.predicateForSamples(withStart: start, end: nil)
        } else {
            predicate = nil
        }

        let sortDesc = NSSortDescriptor(
            key: HKSampleSortIdentifierStartDate,
            ascending: ascending
        )

        return try await withCheckedThrowingContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: type,
                predicate: predicate,
                limit: limit,
                sortDescriptors: [sortDesc]
            ) { _, samples, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: (samples as? [HKQuantitySample]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Category Samples

    func fetchCategorySamples(
        typeID: HKCategoryTypeIdentifier,
        from startDate: Date? = nil,
        limit: Int = HKObjectQueryNoLimit,
        ascending: Bool = true
    ) async throws -> [HKCategorySample] {
        guard let type = HKObjectType.categoryType(forIdentifier: typeID) else { return [] }

        let predicate: NSPredicate?
        if let start = startDate {
            predicate = HKQuery.predicateForSamples(withStart: start, end: nil)
        } else {
            predicate = nil
        }

        let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: ascending)

        return try await withCheckedThrowingContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: type,
                predicate: predicate,
                limit: limit,
                sortDescriptors: [sortDesc]
            ) { _, samples, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: (samples as? [HKCategorySample]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Workouts

    func fetchWorkouts(from startDate: Date? = nil, limit: Int = HKObjectQueryNoLimit) async throws -> [HKWorkout] {
        let predicate: NSPredicate?
        if let start = startDate {
            predicate = HKQuery.predicateForSamples(withStart: start, end: nil)
        } else {
            predicate = nil
        }

        let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)

        return try await withCheckedThrowingContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: .workoutType(),
                predicate: predicate,
                limit: limit,
                sortDescriptors: [sortDesc]
            ) { _, samples, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: (samples as? [HKWorkout]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Blood Pressure (Correlation)

    func fetchBloodPressure(from startDate: Date? = nil, until endDate: Date? = nil) async throws -> [HKCorrelation] {
        guard let type = HKObjectType.correlationType(forIdentifier: .bloodPressure) else { return [] }

        let predicate: NSPredicate?
        if startDate != nil || endDate != nil {
            predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        } else {
            predicate = nil
        }

        return try await withCheckedThrowingContinuation { continuation in
            let query = HKCorrelationQuery(
                type: type,
                predicate: predicate,
                samplePredicates: nil
            ) { _, correlations, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: correlations ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Activity Summaries

    func fetchActivitySummaries(from startDate: Date? = nil, until endDate: Date? = nil) async throws -> [HKActivitySummary] {
        let calendar = Calendar.current
        let predicate: NSPredicate?
        if let start = startDate {
            var startComponents = calendar.dateComponents([.era, .year, .month, .day], from: start)
            startComponents.calendar = calendar
            var endComponents = calendar.dateComponents([.era, .year, .month, .day], from: endDate ?? Date())
            endComponents.calendar = calendar
            predicate = HKQuery.predicate(forActivitySummariesBetweenStart: startComponents, end: endComponents)
        } else {
            predicate = nil
        }
        return try await withCheckedThrowingContinuation { continuation in
            let query = HKActivitySummaryQuery(predicate: predicate) { _, summaries, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: summaries ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Workout Routes

    func fetchWorkoutRoutes(for workout: HKWorkout) async throws -> [HKWorkoutRoute] {
        let predicate = HKQuery.predicateForObjects(from: workout)
        return try await withCheckedThrowingContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: HKSeriesType.workoutRoute(),
                predicate: predicate,
                limit: HKObjectQueryNoLimit,
                sortDescriptors: nil
            ) { _, samples, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: (samples as? [HKWorkoutRoute]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    func fetchRouteLocations(for route: HKWorkoutRoute) async throws -> [CLLocation] {
        try await withCheckedThrowingContinuation { continuation in
            var locations: [CLLocation] = []
            let query = HKWorkoutRouteQuery(route: route) { _, newLocations, done, error in
                if let error = error {
                    continuation.resume(throwing: error)
                    return
                }
                if let locs = newLocations { locations.append(contentsOf: locs) }
                if done { continuation.resume(returning: locations) }
            }
            store.execute(query)
        }
    }

    // MARK: - State of Mind (iOS 18+)

    @available(iOS 18, *)
    func fetchStateOfMind(from startDate: Date? = nil, until endDate: Date? = nil) async throws -> [HKStateOfMind] {
        let type = HKObjectType.stateOfMindType()
        let predicate: NSPredicate? = (startDate != nil || endDate != nil)
            ? HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
            : nil
        let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)

        return try await withCheckedThrowingContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: type,
                predicate: predicate,
                limit: HKObjectQueryNoLimit,
                sortDescriptors: [sortDesc]
            ) { _, samples, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else {
                    continuation.resume(returning: (samples as? [HKStateOfMind]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    // MARK: - Count queries (sync validation)

    func countSamples(type: HKSampleType) async -> Int {
        await withCheckedContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: type, predicate: nil,
                limit: HKObjectQueryNoLimit, sortDescriptors: nil
            ) { _, samples, _ in
                continuation.resume(returning: samples?.count ?? 0)
            }
            store.execute(query)
        }
    }

    func countActivitySummaries() async -> Int {
        await withCheckedContinuation { continuation in
            let query = HKActivitySummaryQuery(predicate: nil) { _, summaries, _ in
                continuation.resume(returning: summaries?.count ?? 0)
            }
            store.execute(query)
        }
    }

    func latestSampleDate(for sampleType: HKSampleType) async -> Date? {
        await withCheckedContinuation { continuation in
            let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: false)
            let query = HKSampleQuery(
                sampleType: sampleType,
                predicate: nil,
                limit: 1,
                sortDescriptors: [sortDesc]
            ) { _, samples, _ in
                continuation.resume(returning: samples?.first?.startDate)
            }
            store.execute(query)
        }
    }

    // MARK: - Streaming queries (memory-efficient paged reads)
    //
    // Uses HKSampleQuery with offset-based pagination for full syncs to guarantee
    // ALL historical data is returned. HKAnchoredObjectQuery can skip records for
    // certain data types (e.g. AppleSleepingWristTemperature) when there is no
    // prior anchor, because it was designed for change-tracking, not bulk export.

    private func pagedBatch<T: HKSample>(
        type: HKSampleType,
        predicate: NSPredicate?,
        limit: Int,
        offset: Int
    ) async throws -> [T] {
        try await withCheckedThrowingContinuation { cont in
            // HKSampleQuery does not support offset directly, so we use
            // limit + sort + date-based cursor. For simplicity and reliability,
            // fetch in pages using limit with ascending sort.
            let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)
            let query = HKSampleQuery(
                sampleType: type,
                predicate: predicate,
                limit: limit,
                sortDescriptors: [sortDesc]
            ) { _, samples, error in
                if let error = error {
                    cont.resume(throwing: error)
                } else {
                    cont.resume(returning: (samples as? [T]) ?? [])
                }
            }
            store.execute(query)
        }
    }

    func streamQuantitySamples(
        typeID: HKQuantityTypeIdentifier,
        from startDate: Date?,
        until endDate: Date? = nil,
        batchSize: Int = 5_000,
        handler: ([HKQuantitySample]) async throws -> Void
    ) async throws {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return }

        // Use cursor-based pagination: fetch a batch, then use the last sample's
        // start date as the lower bound for the next query. This guarantees we get
        // ALL historical data, unlike HKAnchoredObjectQuery.
        var cursorDate = startDate
        while true {
            let predicate: NSPredicate?
            if let start = cursorDate, let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: end, options: .strictStartDate)
            } else if let start = cursorDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: nil, options: .strictStartDate)
            } else if let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: .distantPast, end: end, options: .strictStartDate)
            } else {
                predicate = nil
            }
            let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)

            let samples: [HKQuantitySample] = try await withCheckedThrowingContinuation { cont in
                let query = HKSampleQuery(
                    sampleType: type,
                    predicate: predicate,
                    limit: batchSize,
                    sortDescriptors: [sortDesc]
                ) { _, results, error in
                    if let error = error {
                        cont.resume(throwing: error)
                    } else {
                        cont.resume(returning: (results as? [HKQuantitySample]) ?? [])
                    }
                }
                store.execute(query)
            }

            guard !samples.isEmpty else { break }
            try await handler(samples)

            if samples.count < batchSize { break }

            // Advance cursor past the last sample to avoid infinite loops.
            // Add a tiny epsilon to avoid re-fetching the same sample.
            if let lastDate = samples.last?.startDate {
                cursorDate = lastDate.addingTimeInterval(0.001)
            } else {
                break
            }
        }
    }

    /// Aggregates quantity samples into fixed-interval buckets with min/avg/max.
    /// Much faster than streaming individual samples for high-frequency types like heart rate.
    struct AggregatedBucket {
        let startDate: Date
        let min: Double
        let avg: Double
        let max: Double
    }

    func queryAggregatedStatistics(
        typeID: HKQuantityTypeIdentifier,
        unit: HKUnit,
        from startDate: Date,
        until endDate: Date,
        interval: TimeInterval
    ) async throws -> [AggregatedBucket] {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return [] }

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        var dateComponents = DateComponents()
        dateComponents.second = Int(interval)
        // Anchor to midnight so buckets align to clock boundaries
        let anchor = Calendar.current.startOfDay(for: startDate)

        return try await withCheckedThrowingContinuation { cont in
            let query = HKStatisticsCollectionQuery(
                quantityType: type,
                quantitySamplePredicate: predicate,
                options: [.discreteMin, .discreteAverage, .discreteMax],
                anchorDate: anchor,
                intervalComponents: dateComponents
            )
            query.initialResultsHandler = { _, collection, error in
                if let error = error {
                    cont.resume(throwing: error)
                    return
                }
                guard let collection = collection else {
                    cont.resume(returning: [])
                    return
                }
                var buckets: [AggregatedBucket] = []
                collection.enumerateStatistics(from: startDate, to: endDate) { stats, _ in
                    guard let min = stats.minimumQuantity()?.doubleValue(for: unit),
                          let avg = stats.averageQuantity()?.doubleValue(for: unit),
                          let max = stats.maximumQuantity()?.doubleValue(for: unit) else { return }
                    buckets.append(AggregatedBucket(startDate: stats.startDate, min: min, avg: avg, max: max))
                }
                cont.resume(returning: buckets)
            }
            store.execute(query)
        }
    }

    /// Same buckets as `queryAggregatedStatistics`, computed from the individual samples.
    /// Fallback for `HKStatisticsCollectionQuery` failing with "Unable to invalidate interval:
    /// no data source available" (HKError code 3), which it does for some ranges on iOS 27.
    /// Reads every sample in the range, so it is meant for short ranges such as one workout.
    func aggregateSamples(
        typeID: HKQuantityTypeIdentifier,
        unit: HKUnit,
        from startDate: Date,
        until endDate: Date,
        interval: TimeInterval
    ) async throws -> [AggregatedBucket] {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return [] }

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        let samples: [HKQuantitySample] = try await withCheckedThrowingContinuation { cont in
            let query = HKSampleQuery(
                sampleType: type,
                predicate: predicate,
                limit: HKObjectQueryNoLimit,
                sortDescriptors: nil
            ) { _, results, error in
                if let error = error {
                    cont.resume(throwing: error)
                } else {
                    cont.resume(returning: (results as? [HKQuantitySample]) ?? [])
                }
            }
            store.execute(query)
        }

        // Buckets are aligned to midnight, matching the anchor of queryAggregatedStatistics.
        let anchor = Calendar.current.startOfDay(for: startDate)
        var grouped: [Int: [Double]] = [:]
        for s in samples {
            let index = Int(floor(s.startDate.timeIntervalSince(anchor) / interval))
            grouped[index, default: []].append(s.quantity.doubleValue(for: unit))
        }
        return grouped.keys.sorted().compactMap { index in
            guard let values = grouped[index], let min = values.min(), let max = values.max() else { return nil }
            return AggregatedBucket(
                startDate: anchor.addingTimeInterval(Double(index) * interval),
                min: min,
                avg: values.reduce(0, +) / Double(values.count),
                max: max
            )
        }
    }

    /// Aggregates cumulative quantity samples into fixed-interval buckets with SUM.
    /// Used for step_count, energy, distance, etc. where individual samples are meaningless.
    struct CumulativeBucket {
        let startDate: Date
        let sum: Double
    }

    func queryCumulativeStatistics(
        typeID: HKQuantityTypeIdentifier,
        unit: HKUnit,
        from startDate: Date,
        until endDate: Date,
        interval: TimeInterval
    ) async throws -> [CumulativeBucket] {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return [] }

        let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
        var dateComponents = DateComponents()
        dateComponents.second = Int(interval)
        let anchor = Calendar.current.startOfDay(for: startDate)

        return try await withCheckedThrowingContinuation { cont in
            let query = HKStatisticsCollectionQuery(
                quantityType: type,
                quantitySamplePredicate: predicate,
                options: [.cumulativeSum],
                anchorDate: anchor,
                intervalComponents: dateComponents
            )
            query.initialResultsHandler = { _, collection, error in
                if let error = error {
                    cont.resume(throwing: error)
                    return
                }
                guard let collection = collection else {
                    cont.resume(returning: [])
                    return
                }
                var buckets: [CumulativeBucket] = []
                collection.enumerateStatistics(from: startDate, to: endDate) { stats, _ in
                    guard let sum = stats.sumQuantity()?.doubleValue(for: unit) else { return }
                    buckets.append(CumulativeBucket(startDate: stats.startDate, sum: sum))
                }
                cont.resume(returning: buckets)
            }
            store.execute(query)
        }
    }

    /// Same buckets as `queryCumulativeStatistics`, one `HKStatisticsQuery` per bucket.
    /// Fallback for the collection query failing with "Unable to invalidate interval:
    /// no data source available" (HKError code 3) for some ranges on iOS 27. Summing
    /// the samples instead would count steps twice where iPhone and Watch both recorded
    /// them; a statistics query merges overlapping sources the way the collection query
    /// does.
    func queryCumulativeStatisticsPerBucket(
        typeID: HKQuantityTypeIdentifier,
        unit: HKUnit,
        from startDate: Date,
        until endDate: Date,
        interval: TimeInterval
    ) async throws -> [CumulativeBucket] {
        guard let type = HKObjectType.quantityType(forIdentifier: typeID) else { return [] }
        let dayStart = Calendar.current.startOfDay(for: startDate)
        var bucketStart = dayStart.addingTimeInterval(floor(startDate.timeIntervalSince(dayStart) / interval) * interval)
        var buckets: [CumulativeBucket] = []
        while bucketStart < endDate {
            try Task.checkCancellation()
            let bucketEnd = bucketStart.addingTimeInterval(interval)
            let predicate = HKQuery.predicateForSamples(withStart: bucketStart, end: bucketEnd, options: .strictStartDate)
            let sum: Double? = try await withCheckedThrowingContinuation { cont in
                let query = HKStatisticsQuery(quantityType: type, quantitySamplePredicate: predicate, options: .cumulativeSum) { _, stats, error in
                    if let error = error as? HKError, error.code == .errorNoData {
                        cont.resume(returning: nil)
                    } else if let error {
                        cont.resume(throwing: error)
                    } else {
                        cont.resume(returning: stats?.sumQuantity()?.doubleValue(for: unit))
                    }
                }
                store.execute(query)
            }
            if let sum { buckets.append(CumulativeBucket(startDate: bucketStart, sum: sum)) }
            bucketStart = bucketEnd
        }
        return buckets
    }

    /// Lightweight existence check — returns true if at least one sample exists in the date range.
    func sampleExists(for sampleType: HKSampleType, from startDate: Date, to endDate: Date) async -> Bool {
        await withCheckedContinuation { continuation in
            let predicate = HKQuery.predicateForSamples(withStart: startDate, end: endDate, options: .strictStartDate)
            let query = HKSampleQuery(
                sampleType: sampleType,
                predicate: predicate,
                limit: 1,
                sortDescriptors: nil
            ) { _, samples, _ in
                continuation.resume(returning: (samples?.count ?? 0) > 0)
            }
            store.execute(query)
        }
    }

    func streamCategorySamples(
        typeID: HKCategoryTypeIdentifier,
        from startDate: Date?,
        until endDate: Date? = nil,
        batchSize: Int = 5_000,
        handler: ([HKCategorySample]) async throws -> Void
    ) async throws {
        guard let type = HKObjectType.categoryType(forIdentifier: typeID) else { return }

        var cursorDate = startDate
        while true {
            let predicate: NSPredicate?
            if let start = cursorDate, let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: end, options: .strictStartDate)
            } else if let start = cursorDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: nil, options: .strictStartDate)
            } else if let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: .distantPast, end: end, options: .strictStartDate)
            } else {
                predicate = nil
            }
            let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)

            let samples: [HKCategorySample] = try await withCheckedThrowingContinuation { cont in
                let query = HKSampleQuery(
                    sampleType: type,
                    predicate: predicate,
                    limit: batchSize,
                    sortDescriptors: [sortDesc]
                ) { _, results, error in
                    if let error = error {
                        cont.resume(throwing: error)
                    } else {
                        cont.resume(returning: (results as? [HKCategorySample]) ?? [])
                    }
                }
                store.execute(query)
            }

            guard !samples.isEmpty else { break }
            try await handler(samples)

            if samples.count < batchSize { break }

            if let lastDate = samples.last?.startDate {
                cursorDate = lastDate.addingTimeInterval(0.001)
            } else {
                break
            }
        }
    }

    func streamWorkouts(
        from startDate: Date?,
        until endDate: Date? = nil,
        batchSize: Int = 1_000,
        handler: ([HKWorkout]) async throws -> Void
    ) async throws {
        var cursorDate = startDate
        while true {
            let predicate: NSPredicate?
            if let start = cursorDate, let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: end, options: .strictStartDate)
            } else if let start = cursorDate {
                predicate = HKQuery.predicateForSamples(withStart: start, end: nil, options: .strictStartDate)
            } else if let end = endDate {
                predicate = HKQuery.predicateForSamples(withStart: .distantPast, end: end, options: .strictStartDate)
            } else {
                predicate = nil
            }
            let sortDesc = NSSortDescriptor(key: HKSampleSortIdentifierStartDate, ascending: true)

            let samples: [HKWorkout] = try await withCheckedThrowingContinuation { cont in
                let query = HKSampleQuery(
                    sampleType: .workoutType(),
                    predicate: predicate,
                    limit: batchSize,
                    sortDescriptors: [sortDesc]
                ) { _, results, error in
                    if let error = error {
                        cont.resume(throwing: error)
                    } else {
                        cont.resume(returning: (results as? [HKWorkout]) ?? [])
                    }
                }
                store.execute(query)
            }

            guard !samples.isEmpty else { break }
            try await handler(samples)

            if samples.count < batchSize { break }

            if let lastDate = samples.last?.startDate {
                cursorDate = lastDate.addingTimeInterval(0.001)
            } else {
                break
            }
        }
    }

    // MARK: - Anchored change detection
    //
    // The incremental sync asks HealthKit what was added since a stored anchor, not
    // which samples fall after a date: samples arrive with timestamps before the
    // previous sync (Watch transfer, Oura writing the night later), and a date filter
    // on its own would drop them. Only the time spans of the added samples are
    // returned; the caller re-sends those spans through the date-range sync functions.

    /// Time spans of the samples added to `type` since `anchor` whose time lies after
    /// `notBefore`, and the anchor to store once they have been sent. Without an anchor
    /// this is every sample after `notBefore`, which is how a type starts being tracked:
    /// an anchor taken without reading (a query that matches nothing) turned out not to
    /// mark the current position for every type on iOS 27, and a nil anchor without the
    /// bound returns the whole history. Deletions are not reported: the server keeps
    /// rows it has received, whatever happens to the sample in HealthKit afterwards.
    func addedSampleSpans(
        for type: HKSampleType, since anchor: HKQueryAnchor?, notBefore: Date
    ) async throws -> (spans: [DateInterval], newAnchor: HKQueryAnchor?) {
        let pageSize = 10_000
        let window = HKQuery.predicateForSamples(withStart: notBefore, end: nil)
        var spans: [DateInterval] = []
        var cursor = anchor
        while true {
            let (samples, next): ([HKSample], HKQueryAnchor?) = try await withCheckedThrowingContinuation { cont in
                let query = HKAnchoredObjectQuery(type: type, predicate: window, anchor: cursor, limit: pageSize) { _, added, _, newAnchor, error in
                    if let error {
                        cont.resume(throwing: error)
                    } else {
                        cont.resume(returning: (added ?? [], newAnchor))
                    }
                }
                store.execute(query)
            }
            spans.append(contentsOf: samples.map { DateInterval(start: $0.startDate, end: max($0.startDate, $0.endDate)) })
            cursor = next ?? cursor
            if samples.count < pageSize { break }
        }
        return (spans, cursor)
    }

    // MARK: - Sample counts (for status display)

    func sampleCount(for type: HKSampleType) async -> Int {
        await withCheckedContinuation { continuation in
            let query = HKSampleQuery(
                sampleType: type,
                predicate: nil,
                limit: HKObjectQueryNoLimit,
                sortDescriptors: nil
            ) { _, samples, _ in
                continuation.resume(returning: samples?.count ?? 0)
            }
            store.execute(query)
        }
    }
}

// MARK: - Helper extensions


