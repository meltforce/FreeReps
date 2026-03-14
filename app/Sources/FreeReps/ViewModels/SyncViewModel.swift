import Combine
import Foundation
import HealthKit
import SwiftUI

@MainActor
final class SyncViewModel: ObservableObject {

    private(set) var syncState: SyncState
    private let syncService: SyncService
    private var cancellables = Set<AnyCancellable>()
    private var syncTask: Task<Void, Never>?

    @Published var prerequisiteIssues: [SyncPrerequisiteIssue] = []
    @Published var showPrerequisiteAlert = false

    init() {
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
            await syncService.runFullSync(config: config)
            refreshLatestHealthKitDates()
        }
        syncTask = task
        syncService.taskForCancellation = task
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
        case "cat_ecg":
            return await HealthKitService.shared.latestSampleDate(for: .electrocardiogramType())
        case "cat_audiogram":
            return await HealthKitService.shared.latestSampleDate(for: .audiogramSampleType())
        case "cat_workout_routes":
            return await HealthKitService.shared.latestSampleDate(for: HKSeriesType.workoutRoute())
        case "cat_vision":
            return await HealthKitService.shared.latestSampleDate(for: HKObjectType.visionPrescriptionType())
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
