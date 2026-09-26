import SwiftUI
import UIKit

struct SyncDashboardView: View {
    @ObservedObject var vm: SyncViewModel
    @State private var navigateToHealthPermissions = false
    @AppStorage("keepScreenOnDuringSync") private var keepScreenOnDuringSync = true

    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        NavigationStack {
            TimelineView(.periodic(from: .now, by: 60)) { context in
                ScrollView {
                    VStack(spacing: 0) {
                        statusCard(now: context.date)
                        banners
                        categoryHeader
                        LazyVGrid(columns: [GridItem(.flexible(), spacing: 10), GridItem(.flexible(), spacing: 10)], spacing: 10) {
                            ForEach(vm.categories) { cat in
                                CategoryStatusCard(
                                    state: cat,
                                    now: context.date,
                                    onReset: { vm.resetCategory(categoryID: cat.id) },
                                    onSync: { vm.startCategorySync(categoryID: cat.id) },
                                    isSyncRunning: vm.isAnySyncRunning
                                )
                            }
                        }
                    }
                    .padding(.horizontal, 16)
                    .padding(.top, 12)
                    .padding(.bottom, 24)
                }
            }
            .background(Color(.systemGroupedBackground))
            // No visible title: the status card starts right below the safe area.
            // The title stays for VoiceOver; Import File lives in Settings › Data.
            .navigationTitle("FreeReps")
            .toolbar(.hidden, for: .navigationBar)
            .navigationDestination(isPresented: $navigateToHealthPermissions) {
                HealthPermissionsView(vm: SettingsViewModel())
            }
            .onAppear {
                vm.refreshRecordCounts()
                vm.checkPrerequisites()
                vm.refreshLatestHealthKitDates()
            }
            .onChange(of: vm.isFullSyncRunning) { _, isRunning in
                UIApplication.shared.isIdleTimerDisabled = isRunning && keepScreenOnDuringSync
            }
            .onDisappear {
                UIApplication.shared.isIdleTimerDisabled = false
            }
            .alert("Sync Prerequisites", isPresented: $vm.showPrerequisiteAlert) {
                Button("Continue Anyway") { }
                Button("Cancel Sync", role: .cancel) {
                    vm.cancelSync()
                }
            } message: {
                let titles = vm.prerequisiteIssues.map { $0.title }
                Text("Issues found:\n\(titles.joined(separator: "\n"))\n\nThe sync will continue but some data may be missing. Fix these issues in Settings for a complete sync.")
            }
        }
    }

    // MARK: - Status card

    /// The last finished run; a device that synced before the record existed
    /// falls back to the stored last sync date.
    private var lastRecord: LastSyncRecord? {
        if let record = vm.lastRecord { return record }
        guard let date = vm.lastSyncDate else { return nil }
        return LastSyncRecord(finishedAt: date, outcome: .succeeded, message: nil)
    }

    // While a sync runs the card shows that run, not the outcome of the previous one.

    private var statusColor: Color {
        if vm.isAnySyncRunning { return Color("Brand") }
        switch lastRecord?.outcome {
        case .succeeded: return Color("Brand")
        case .failed: return Color("StatusWarning")
        case .cancelled, nil: return .secondary
        }
    }

    private var statusSymbol: String {
        if vm.isAnySyncRunning { return "arrow.triangle.2.circlepath" }
        switch lastRecord?.outcome {
        case .succeeded: return "checkmark.circle.fill"
        case .failed: return "exclamationmark.circle.fill"
        case .cancelled: return "xmark.circle.fill"
        case nil: return "arrow.triangle.2.circlepath"
        }
    }

    private var statusTitle: String {
        if vm.isAnySyncRunning { return "Syncing\u{2026}" }
        switch lastRecord?.outcome {
        case .succeeded: return "Synced"
        case .failed: return "Sync failed"
        case .cancelled: return "Sync cancelled"
        case nil: return "Never synced"
        }
    }

    private func statusSubtitle(now: Date) -> String {
        if vm.isAnySyncRunning {
            if let wait = vm.healthKitWait { return wait }
            return vm.currentOperation.isEmpty ? "Starting\u{2026}" : vm.currentOperation
        }
        guard let record = lastRecord else { return "Run Full Sync to start" }
        let when = RelativeTime.describe(record.finishedAt, now: now, style: .full)
        return record.outcome == .succeeded ? when.prefix(1).uppercased() + when.dropFirst() : "Attempted \(when)"
    }

    /// Error text of the current state, or of the last failed run.
    private var errorText: String? {
        vm.errorMessage ?? (lastRecord?.outcome == .failed ? lastRecord?.message : nil)
    }

    private func statusCard(now: Date) -> some View {
        VStack(spacing: 16) {
            HStack(spacing: 14) {
                ZStack {
                    Circle()
                        .fill(statusColor.opacity(colorScheme == .dark ? 0.16 : 0.12))
                    Image(systemName: statusSymbol)
                        .font(.system(size: 32))
                        .foregroundStyle(statusColor)
                        .symbolEffect(.rotate, options: .repeat(.continuous), isActive: vm.isAnySyncRunning)
                        // A new view per state: otherwise the rotation finishes its turn on
                        // the outcome symbol after the sync has ended.
                        .id(vm.isAnySyncRunning)
                }
                .frame(width: 52, height: 52)

                VStack(alignment: .leading, spacing: 2) {
                    Text(statusTitle)
                        .font(.system(size: 24, weight: .bold))
                        .tracking(-0.24)
                        .lineLimit(1)
                    Text(statusSubtitle(now: now))
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }

                Spacer(minLength: 0)

                VStack(alignment: .trailing, spacing: 2) {
                    Text(vm.totalRecords.formatted(.number.notation(.compactName).precision(.significantDigits(1...3))))
                        .font(.system(size: 20, weight: .bold, design: .rounded))
                        .monospacedDigit()
                        .contentTransition(.numericText())
                        .animation(.default, value: vm.totalRecords)
                    Text("records")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            if let errorText {
                Text(errorText)
                    .font(.footnote)
                    .lineSpacing(3)
                    .foregroundStyle(Color("WarningText"))
                    .padding(.vertical, 10)
                    .padding(.horizontal, 12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.orange.opacity(colorScheme == .dark ? 0.14 : 0.10), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
            }

            syncControls
        }
        .padding(18)
        .background(Color(.secondarySystemGroupedBackground), in: RoundedRectangle(cornerRadius: 22, style: .continuous))
    }

    @ViewBuilder
    private var syncControls: some View {
        if vm.isAnySyncRunning {
            VStack(spacing: 12) {
                Button(role: .cancel) {
                    vm.cancelSync()
                } label: {
                    Label("Cancel", systemImage: "xmark")
                        .font(.headline)
                        .frame(maxWidth: .infinity, minHeight: 36)
                }
                .buttonStyle(.glass)
                .buttonBorderShape(.capsule)
                .controlSize(.large)
                .tint(.red)

                VStack(alignment: .leading, spacing: 4) {
                    HStack {
                        Text("Overall Progress")
                        Spacer()
                        Text("\(Int(vm.overallProgress * 100))%")
                            .monospacedDigit()
                    }
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    ProgressView(value: vm.overallProgress)
                        .tint(Color("Brand"))
                }
            }
        } else {
            Button {
                vm.startFullSync()
            } label: {
                Label("Full Sync", systemImage: "arrow.triangle.2.circlepath")
                    .font(.headline)
                    .foregroundStyle(Color("BrandOnFill"))
                    .frame(maxWidth: .infinity, minHeight: 36)
            }
            .buttonStyle(.glassProminent)
            .buttonBorderShape(.capsule)
            .controlSize(.large)
            .tint(Color("BrandFill"))
        }
    }

    // MARK: - Banners

    @ViewBuilder
    private var banners: some View {
        VStack(spacing: 10) {
            if !vm.hasCompletedFullSync && !vm.isAnySyncRunning {
                noticeBanner(
                    icon: "exclamationmark.triangle.fill",
                    color: .yellow,
                    title: "No Complete Baseline",
                    message: "A full sync has never completed. Historical data may be missing from FreeReps. Run Full Sync to establish a complete baseline."
                )
            }

            // With "Keep Screen On" enabled (the default) the app keeps the display awake
            // itself; the notice is only needed when that is switched off.
            if vm.isFullSyncRunning && !keepScreenOnDuringSync {
                noticeBanner(
                    icon: "lock.open.display",
                    color: .blue,
                    title: "Keep Screen On",
                    message: "Apple HealthKit is not accessible when the device is locked. Keep the screen on until the sync completes, or enable Keep Screen On in Settings."
                )
            }

            if !vm.prerequisiteIssues.isEmpty && !vm.isAnySyncRunning {
                VStack(alignment: .leading, spacing: 10) {
                    Text("Action Required")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.secondary)
                    ForEach(vm.prerequisiteIssues) { issue in
                        VStack(alignment: .leading, spacing: 4) {
                            HStack(spacing: 8) {
                                Image(systemName: "exclamationmark.circle.fill")
                                    .foregroundStyle(.orange)
                                Text(issue.title)
                                    .font(.subheadline.weight(.semibold))
                            }
                            Text(issue.message)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                            if !issue.actionLabel.isEmpty {
                                Button(issue.actionLabel) {
                                    handlePrerequisiteAction(issue)
                                }
                                .font(.caption.weight(.semibold))
                                .tint(Color("Brand"))
                            }
                        }
                    }
                }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color.orange.opacity(colorScheme == .dark ? 0.14 : 0.10), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
            }
        }
        .padding(.top, 10)
    }

    private func noticeBanner(icon: String, color: Color, title: String, message: String) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Image(systemName: icon)
                .foregroundStyle(color)
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                    .font(.caption.weight(.semibold))
                Text(message)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(color.opacity(colorScheme == .dark ? 0.14 : 0.10), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
    }

    // MARK: - Categories

    /// Categories that completed and hold everything HealthKit has.
    private var upToDateCount: Int {
        vm.categories.filter { $0.status == .completed && $0.daysBehind == nil }.count
    }

    private var categoryHeader: some View {
        HStack(alignment: .firstTextBaseline) {
            Text("Categories")
                .font(.title2.bold())
            Spacer()
            Text("\(upToDateCount) of \(vm.categories.count) up to date")
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .lineLimit(1)
        }
        .padding(.top, 24)
        .padding(.bottom, 10)
        .padding(.horizontal, 4)
    }

    private func handlePrerequisiteAction(_ issue: SyncPrerequisiteIssue) {
        switch issue {
        case .healthPermissionsNotRequested, .somePermissionsDenied:
            navigateToHealthPermissions = true
        case .connectionFailed:
            break
        case .healthDataUnavailable:
            break
        }
    }
}
