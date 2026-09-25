import SwiftUI
import WidgetKit

/// Shows when the last sync finished and how it ended. Tapping it opens the app
/// through `freereps://sync`, which starts a sync in the foreground.
struct LastSyncWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "LastSync", provider: LastSyncProvider()) { entry in
            LastSyncView(record: entry.record)
                .containerBackground(.fill.tertiary, for: .widget)
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

/// One entry, no refresh schedule: the app reloads the timeline after every sync,
/// and the relative time text updates on its own.
struct LastSyncProvider: TimelineProvider {
    func placeholder(in context: Context) -> LastSyncEntry {
        LastSyncEntry(date: Date(), record: LastSyncRecord(finishedAt: Date(), outcome: .succeeded, message: nil))
    }

    func getSnapshot(in context: Context, completion: @escaping (LastSyncEntry) -> Void) {
        completion(LastSyncEntry(date: Date(), record: LastSyncRecord.load() ?? placeholder(in: context).record))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<LastSyncEntry>) -> Void) {
        completion(Timeline(entries: [LastSyncEntry(date: Date(), record: LastSyncRecord.load())], policy: .never))
    }
}

struct LastSyncView: View {
    @Environment(\.widgetFamily) private var family
    let record: LastSyncRecord?

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
                    Text(record.finishedAt, style: .relative) + Text(" ago")
                }
            }
        default:
            VStack(alignment: .leading, spacing: 6) {
                Text("FreeReps")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                Spacer(minLength: 0)
                Image(systemName: symbol)
                    .font(.title2)
                    .foregroundStyle(record?.outcome == .failed ? Color.orange : Color.accentColor)
                Text(statusText)
                    .font(.headline)
                if let record {
                    (Text(record.finishedAt, style: .relative) + Text(" ago"))
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    if let message = record.message {
                        Text(message)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                            .lineLimit(2)
                    }
                }
                Text("Tap to sync")
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        }
    }
}
