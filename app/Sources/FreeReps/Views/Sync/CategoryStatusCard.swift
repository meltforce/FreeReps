import SwiftUI

struct CategoryStatusCard: View {
    let state: CategorySyncState
    var onReset: (() -> Void)? = nil
    var onSync: (() -> Void)? = nil
    var isSyncRunning: Bool = false

    @State private var showResetConfirm = false

    var body: some View {
        HStack(spacing: 12) {
            // Icon
            ZStack {
                RoundedRectangle(cornerRadius: 10)
                    .fill(iconColor.opacity(0.15))
                    .frame(width: 44, height: 44)
                Image(systemName: state.systemImage)
                    .font(.system(size: 20))
                    .foregroundStyle(iconColor)
            }

            // Info
            VStack(alignment: .leading, spacing: 3) {
                Text(state.displayName)
                    .font(.subheadline.weight(.semibold))
                    .foregroundStyle(.primary)
                HStack(spacing: 6) {
                    statusBadge
                    if state.recordCount > 0 || state.status == .syncing {
                        Text("\(state.recordCount.formatted()) records")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                            .monospacedDigit()
                            .lineLimit(1)
                            .contentTransition(.numericText())
                            .animation(.default, value: state.recordCount)
                    }
                }
                if case .syncing = state.status {
                    if state.currentProgress > 0 {
                        Text("Window \(state.currentProgress)/\(state.totalEstimated)")
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .monospacedDigit()
                    }
                    ProgressView(value: state.progressFraction)
                        .tint(.blue)
                        .frame(maxWidth: 180)
                }
            }

            Spacer()

            // Last sync time + staleness indicator
            VStack(alignment: .trailing, spacing: 2) {
                if let date = state.lastSyncDate {
                    Text(date, style: .relative)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.trailing)
                }
                if let days = state.daysBehind {
                    Text(days == 1 ? "1 day behind" : "\(days)d behind")
                        .font(.caption2)
                        .foregroundStyle(.orange)
                        .multilineTextAlignment(.trailing)
                }
            }
        }
        .padding(.vertical, 6)
        .swipeActions(edge: .trailing, allowsFullSwipe: true) {
            if !isSyncRunning {
                Button {
                    onSync?()
                } label: {
                    Label("Sync", systemImage: "arrow.clockwise")
                }
                .tint(.blue)
            }
        }
        .swipeActions(edge: .leading, allowsFullSwipe: false) {
            if !isSyncRunning {
                Button(role: .destructive) {
                    showResetConfirm = true
                } label: {
                    Label("Reset", systemImage: "trash.fill")
                }
            }
        }
        .confirmationDialog(
            "Reset \(state.displayName)?",
            isPresented: $showResetConfirm,
            titleVisibility: .visible
        ) {
            Button("Delete All Records", role: .destructive) {
                onReset?()
            }
        } message: {
            Text("This permanently deletes all \(state.displayName) records from MySQL. This cannot be undone.")
        }
    }

    private var statusBadge: some View {
        HStack(spacing: 4) {
            Circle()
                .fill(statusColor)
                .frame(width: 7, height: 7)
            Text(state.status.label)
                .font(.caption)
                .foregroundStyle(statusColor)
        }
    }

    private var statusColor: Color {
        switch state.status {
        case .idle:       return .secondary
        case .syncing:    return .blue
        case .completed:  return .green
        case .failed:     return .red
        }
    }

    private var iconColor: Color {
        switch state.status {
        case .failed:    return .red
        case .syncing:   return .blue
        case .completed: return .green
        default:         return state.daysBehind != nil ? .orange : .blue
        }
    }

}
