import SwiftUI
import WidgetKit

/// Shows when the last sync finished and how it ended. Tapping it opens the app
/// through `freereps://sync`, which starts a sync in the foreground.
struct LastSyncWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "LastSync", provider: LastSyncProvider()) { entry in
            LastSyncView(record: entry.record, now: entry.date)
                .containerBackground(for: .widget) { Color(.systemBackground) }
                .widgetURL(SyncLink.url)
        }
        .configurationDisplayName("FreeReps Sync")
        .description("The last sync to your FreeReps server. Tap to sync now.")
        .supportedFamilies([.systemSmall, .accessoryRectangular, .accessoryCircular])
    }
}

struct LastSyncEntry: TimelineEntry {
    let date: Date
    let record: LastSyncRecord?
}

/// The small widget shows the age of the last sync as a number and a unit, which
/// a live `Text(_:style:)` cannot split. The timeline therefore holds one entry per
/// point at which that text changes: every minute for the first hour, every hour
/// for the first day, every day for the first week, then one reload per day. The app
/// also reloads it after every sync.
struct LastSyncProvider: TimelineProvider {
    func placeholder(in context: Context) -> LastSyncEntry {
        LastSyncEntry(date: Date(), record: LastSyncRecord(finishedAt: Date().addingTimeInterval(-12 * 60), outcome: .succeeded, message: nil))
    }

    func getSnapshot(in context: Context, completion: @escaping (LastSyncEntry) -> Void) {
        completion(LastSyncEntry(date: Date(), record: LastSyncRecord.load() ?? placeholder(in: context).record))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<LastSyncEntry>) -> Void) {
        let now = Date()
        guard let record = LastSyncRecord.load() else {
            completion(Timeline(entries: [LastSyncEntry(date: now, record: nil)], policy: .never))
            return
        }
        let finished = record.finishedAt
        var dates: [Date] = [now]
        dates += (1..<60).map { finished.addingTimeInterval(Double($0) * 60) }
        dates += (1..<24).map { finished.addingTimeInterval(Double($0) * 3600) }
        dates += (1...7).map { finished.addingTimeInterval(Double($0) * 86400) }
        let entries = dates.filter { $0 >= now }.sorted().map { LastSyncEntry(date: $0, record: record) }
        // Past the first week the day count advances through one reload per day.
        let reload = max(entries.last?.date ?? now, now).addingTimeInterval(86400)
        completion(Timeline(entries: entries, policy: .after(reload)))
    }
}

/// Age of the last sync as a number and a unit, e.g. ("12", "min ago").
struct SyncAge {
    let value: String
    let unit: String

    init(from finished: Date, to now: Date) {
        let seconds = max(0, now.timeIntervalSince(finished))
        switch seconds {
        case ..<60:
            value = "<1"; unit = "min ago"
        case ..<3600:
            value = "\(Int(seconds / 60))"; unit = "min ago"
        case ..<86400:
            value = "\(Int(seconds / 3600))"; unit = "h ago"
        default:
            let days = Int(seconds / 86400)
            value = "\(days)"; unit = days == 1 ? "day ago" : "days ago"
        }
    }
}

struct LastSyncView: View {
    @Environment(\.widgetFamily) private var family
    let record: LastSyncRecord?
    var now: Date = Date()

    private var symbol: String {
        switch record?.outcome {
        case .succeeded: return "checkmark.circle.fill"
        case .failed: return "exclamationmark.triangle.fill"
        case .cancelled: return "xmark.circle.fill"
        case nil: return "arrow.triangle.2.circlepath"
        }
    }

    private var statusText: String {
        switch record?.outcome {
        case .succeeded: return "Synced"
        case .failed: return "Sync failed"
        case .cancelled: return "Cancelled"
        case nil: return "Never synced"
        }
    }

    private var statusColor: Color {
        switch record?.outcome {
        case .succeeded: return Color("Brand")
        case .failed: return Color("StatusWarning")
        case .cancelled, nil: return .secondary
        }
    }

    var body: some View {
        switch family {
        case .accessoryCircular:
            ZStack {
                AccessoryWidgetBackground()
                Image(systemName: symbol)
            }
        case .accessoryRectangular:
            VStack(alignment: .leading, spacing: 2) {
                Label(statusText, systemImage: symbol)
                    .font(.headline)
                if let record {
                    Text("\(record.finishedAt, style: .relative) ago")
                }
            }
        default:
            VStack(alignment: .leading, spacing: 0) {
                Text("FreeReps")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color("Brand"))
                Spacer(minLength: 0)
                Text("Last sync")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
                HStack(alignment: .firstTextBaseline, spacing: 4) {
                    if let record {
                        let age = SyncAge(from: record.finishedAt, to: now)
                        Text(age.value)
                            .font(.system(size: 34, weight: .bold, design: .rounded))
                            .tracking(-0.68)
                        Text(age.unit)
                            .font(.system(size: 15, weight: .semibold))
                            .foregroundStyle(.secondary)
                    } else {
                        Text("–")
                            .font(.system(size: 34, weight: .bold, design: .rounded))
                    }
                }
                .lineLimit(1)
                .minimumScaleFactor(0.6)
                HStack(spacing: 6) {
                    Circle()
                        .fill(statusColor)
                        .frame(width: 8, height: 8)
                    Text(statusText)
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(statusColor)
                        .lineLimit(1)
                }
                .padding(.top, 8)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        }
    }
}
