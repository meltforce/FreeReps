import Combine
import Foundation
import HealthKit
import SwiftUI
import WidgetKit

@MainActor
final class SyncViewModel: ObservableObject {

    private(set) var syncState: SyncState
    private let syncService: SyncService
    private var cancellables = Set<AnyCancellable>()
    private var syncTask: Task<Void, Never>?

    @Published var prerequisiteIssues: [SyncPrerequisiteIssue] = []
    @Published var showPrerequisiteAlert = false

    /// One instance for the UI, the widget's deep link and the Shortcuts intent, so a
    /// sync started from any of them is the one the dashboard shows and a second start
    /// is refused while it runs.
    static let shared = SyncViewModel()

    private init() {
        let state = SyncState()
        self.syncState = state
        self.syncService = SyncService(syncState: state)
        // Forward SyncState changes so SwiftUI views subscribed to this
        // view model re-render whenever any SyncState @Published property changes.
        state.objectWillChange
            .sink { [weak self] in self?.objectWillChange.send() }
            .store(in: &cancellables)
    }

    var categories: [CategorySyncState] { syncState.categories }
    var isFullSyncRunning: Bool { syncState.isFullSyncRunning }
    var isAnySyncRunning: Bool { syncState.isAnySyncRunning }
    var totalRecords: Int { syncState.totalRecords }
    var lastSyncDate: Date? { syncState.lastSyncDate }
    var overallProgress: Double { syncState.overallProgress }
    var currentOperation: String { syncState.currentOperation }
    var errorMessage: String? { syncState.errorMessage }
    var hasCompletedFullSync: Bool { syncState.hasCompletedFullSync }

    var lastSyncLabel: String {
        guard let date = lastSyncDate else { return "Never synced" }
        let rel = RelativeDateTimeFormatter()
        rel.unitsStyle = .full
        return "Last synced \(rel.localizedString(for: date, relativeTo: Date()))"
    }

    func checkPrerequisites() {
        let config = FreeRepsConfig.load()
        Task {
            let issues = await syncService.validatePrerequisites(config: config)
            self.prerequisiteIssues = issues
        }
    }

    func startFullSync() {
        let config = FreeRepsConfig.load()
        // Fire off prerequisite validation without blocking the sync
        Task {
            let issues = await syncService.validatePrerequisites(config: config)
            self.prerequisiteIssues = issues
            if !issues.isEmpty {
                self.showPrerequisiteAlert = true
            }
        }
        let task = Task {
            _ = await runSyncBody(config: config)
        }
        syncTask = task
        syncService.taskForCancellation = task
    }

    /// Runs a sync to completion and returns its outcome; used by the Shortcuts intent,
    /// which has to report a result. The caller checks `isAnySyncRunning` first.
    func runSync() async -> LastSyncRecord {
        let config = FreeRepsConfig.load()
        let task = Task { _ = await runSyncBody(config: config) }
        syncTask = task
        syncService.taskForCancellation = task
        await task.value
        return lastRecord ?? LastSyncRecord(finishedAt: Date(), outcome: .cancelled, message: nil)
    }

    /// Outcome of the most recent run started from this instance.
    private(set) var lastRecord: LastSyncRecord?

    private func runSyncBody(config: FreeRepsConfig) async -> LastSyncRecord {
        await syncService.runFullSync(config: config)
        refreshLatestHealthKitDates()
        let outcome: LastSyncRecord.Outcome
        if syncState.errorMessage != nil {
            outcome = .failed
        } else if syncState.currentOperation == "Sync cancelled" {
            outcome = .cancelled
        } else {
            outcome = .succeeded
        }
        let record = LastSyncRecord(
            finishedAt: Date(),
            outcome: outcome,
            message: outcome == .failed ? syncState.errorMessage : nil
        )
        record.save()
        lastRecord = record
        WidgetCenter.shared.reloadAllTimelines()
        return record
    }

    func cancelSync() {
        syncTask?.cancel()
        syncTask = nil
    }

    func startCategorySync(categoryID: String) {
        let config = FreeRepsConfig.load()
        syncTask = Task {
            await syncService.runSingleCategorySync(categoryID: categoryID, config: config)
            refreshLatestHealthKitDates()
        }
    }

    /// Re-syncs a list of categories sequentially (used by data validation repair).
    func repairCategories(categoryIDs: [String]) {
        let config = FreeRepsConfig.load()
        let task = Task {
            for catID in categoryIDs {
                await syncService.runSingleCategorySync(categoryID: catID, config: config)
            }
            refreshLatestHealthKitDates()
        }
        syncTask = task
        syncService.taskForCancellation = task
    }

    func resetCategory(categoryID: String) {
        guard !isAnySyncRunning else { return }
        // With FreeReps, server-side data management replaces client-side DB resets.
        // Reset local sync state so the next sync re-sends all data for this category.
        syncState.resetCategoryLocalState(categoryID)
    }

    func resetAllSyncState() {
        guard !isAnySyncRunning else { return }
        syncState.resetAllLocalState()
    }

    func refreshLatestHealthKitDates() {
        Task {
            for i in syncState.categories.indices {
                let date = await latestHKDate(for: syncState.categories[i].id)
                syncState.categories[i].latestHealthKitDate = date
            }
        }
    }

    private func latestHKDate(for catID: String) async -> Date? {
        if catID.hasPrefix("qty_") {
            guard let cat = HealthCategory.allCases.first(where: { "qty_\($0.rawValue)" == catID })
            else { return nil }
            let types = HealthDataTypes.allQuantityTypes.filter { $0.category == cat }
            return await withTaskGroup(of: Date?.self) { group in
                for td in types {
                    guard let hkType = td.hkType else { continue }
                    group.addTask { await HealthKitService.shared.latestSampleDate(for: hkType) }
                }
                var latest: Date? = nil
                for await date in group {
                    if let d = date, latest == nil || d > latest! { latest = d }
                }
                return latest
            }
        }
        switch catID {
        case "cat_category":
            return await withTaskGroup(of: Date?.self) { group in
                for td in HealthDataTypes.allCategoryTypes {
                    guard let hkType = td.hkType else { continue }
                    group.addTask { await HealthKitService.shared.latestSampleDate(for: hkType) }
                }
                var latest: Date? = nil
                for await date in group {
                    if let d = date, latest == nil || d > latest! { latest = d }
                }
                return latest
            }
        case "cat_workouts":
            return await HealthKitService.shared.latestSampleDate(for: .workoutType())
        case "cat_bp":
            guard let t = HKObjectType.correlationType(forIdentifier: .bloodPressure) else { return nil }
            return await HealthKitService.shared.latestSampleDate(for: t)
        case "cat_workout_routes":
            return await HealthKitService.shared.latestSampleDate(for: HKSeriesType.workoutRoute())
        case "cat_state_of_mind":
            if #available(iOS 18, *) {
                return await HealthKitService.shared.latestSampleDate(for: HKObjectType.stateOfMindType())
            }
            return nil
        case "cat_strength":
            return nil
        default:
            return nil
        }
    }

    func refreshRecordCounts() {
        // Record counts are tracked locally in SyncState from ingest results.
        // No direct DB query needed — counts accumulate from successful sync operations.
        syncState.persist()
    }
}
