import SwiftUI
import UIKit

struct SyncDashboardView: View {
    @ObservedObject var vm: SyncViewModel
    @State private var navigateToHealthPermissions = false
    @AppStorage("keepScreenOnDuringSync") private var keepScreenOnDuringSync = true

    var body: some View {
        NavigationStack {
            List {
                // Header section
                Section {
                    VStack(spacing: 16) {
                        statusHeader
                        syncButtons
                        if vm.isAnySyncRunning {
                            overallProgress
                        }
                    }
                    .padding(.vertical, 4)
                }

                // No-full-sync warning banner
                if !vm.hasCompletedFullSync && !vm.isAnySyncRunning {
                    Section {
                        noticeBanner(
                            icon: "exclamationmark.triangle.fill",
                            color: .yellow,
                            title: "No Complete Baseline",
                            message: "A full sync has never completed. Historical data may be missing from FreeReps. Run Full Sync to establish a complete baseline."
                        )
                    }
                }

                // Full sync screen-on reminder
                if vm.isFullSyncRunning {
                    Section {
                        noticeBanner(
                            icon: "lock.open.display",
                            color: .blue,
                            title: "Keep Screen On",
                            message: "Apple HealthKit is not accessible when the device is locked. Keep the screen on until the full sync completes."
                        )
                    }
                }

                // Error banner
                if let err = vm.errorMessage {
                    Section {
                        noticeBanner(
                            icon: "exclamationmark.triangle.fill",
                            color: .red,
                            title: nil,
                            message: err
                        )
                    }
                }

                // Prerequisite issues banner
                if !vm.prerequisiteIssues.isEmpty && !vm.isAnySyncRunning {
                    Section("Action Required") {
                        ForEach(vm.prerequisiteIssues) { issue in
                            VStack(alignment: .leading, spacing: 6) {
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
                                }
                            }
                            .padding(.vertical, 4)
                        }
                    }
                }

                // Category cards
                Section("Categories") {
                    ForEach(vm.categories) { cat in
                        CategoryStatusCard(
                            state: cat,
                            onReset: { vm.resetCategory(categoryID: cat.id) },
                            onSync: { vm.startCategorySync(categoryID: cat.id) },
                            isSyncRunning: vm.isAnySyncRunning
                        )
                    }
                }

                BrandFooter()
            }
            .navigationTitle("FreeReps")
            .navigationBarTitleDisplayMode(.large)
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

    private var statusHeader: some View {
        VStack(spacing: 6) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(vm.lastSyncLabel)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    if vm.totalRecords > 0 {
                        Text("\(vm.totalRecords.formatted()) total records in DB")
                            .font(.headline)
                            .foregroundStyle(.primary)
                    }
                }
                Spacer()
            }
            if vm.isAnySyncRunning, !vm.currentOperation.isEmpty {
                Text(vm.currentOperation)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }

    private var syncButtons: some View {
        HStack(spacing: 12) {
            Button {
                vm.startFullSync()
            } label: {
                Label("Full Sync", systemImage: "arrow.clockwise.icloud.fill")
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 10)
                    .background(Color.blue, in: RoundedRectangle(cornerRadius: 10))
                    .foregroundStyle(.white)
                    .font(.subheadline.weight(.semibold))
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .disabled(vm.isAnySyncRunning)
            .opacity(vm.isAnySyncRunning ? 0.5 : 1)

            if vm.isAnySyncRunning {
                Button {
                    vm.cancelSync()
                } label: {
                    Label("Cancel", systemImage: "xmark.circle.fill")
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                        .background(Color.red.opacity(0.12), in: RoundedRectangle(cornerRadius: 10))
                        .foregroundStyle(.red)
                        .font(.subheadline.weight(.semibold))
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
        }
    }

    private var overallProgress: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("Overall Progress")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Spacer()
                Text("\(Int(vm.overallProgress * 100))%")
                    .font(.caption.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            ProgressView(value: vm.overallProgress)
                .tint(.blue)
        }
    }

    private func noticeBanner(icon: String, color: Color, title: String?, message: String) -> some View {
        HStack(spacing: 10) {
            Image(systemName: icon)
                .foregroundStyle(color)
            VStack(alignment: .leading, spacing: 2) {
                if let title {
                    Text(title)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(.primary)
                }
                Text(message)
                    .font(.caption)
                    .foregroundStyle(title != nil ? .secondary : color)
                    .lineLimit(3)
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(color.opacity(0.10), in: RoundedRectangle(cornerRadius: 10))
        .listRowBackground(Color.clear)
        .listRowInsets(EdgeInsets(top: 4, leading: 16, bottom: 4, trailing: 16))
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
