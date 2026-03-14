import ActivityKit
import WidgetKit
import SwiftUI

struct SyncLiveActivityWidget: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: SyncActivityAttributes.self) { context in
            SyncLockScreenView(context: context)
                .activityBackgroundTint(Color(.systemBackground))
        } dynamicIsland: { context in
            DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    HStack(spacing: 5) {
                        Image("HeartRate")
                            .resizable()
                            .scaledToFit()
                            .frame(width: 16, height: 16)
                            .foregroundStyle(Color(red: 0.93, green: 0.18, blue: 0.28))
                        Text("FreeReps")
                            .font(.caption.weight(.semibold))
                    }
                }
                DynamicIslandExpandedRegion(.trailing) {
                    Text("\(context.state.recordsInserted.formatted())")
                        .font(.caption.monospacedDigit().weight(.semibold))
                        .foregroundStyle(.primary)
                }
                DynamicIslandExpandedRegion(.center) {
                    Text(context.state.phase)
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(.secondary)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    Text(context.state.operation)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            } compactLeading: {
                Image("HeartRate")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 14, height: 14)
                    .foregroundStyle(Color(red: 0.93, green: 0.18, blue: 0.28))
            } compactTrailing: {
                Text("\(context.state.recordsInserted.formatted())")
                    .font(.caption2.monospacedDigit())
                    .minimumScaleFactor(0.6)
            } minimal: {
                Image("HeartRate")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 14, height: 14)
                    .foregroundStyle(Color(red: 0.93, green: 0.18, blue: 0.28))
            }
            .keylineTint(.red)
        }
    }
}

private struct SyncLockScreenView: View {
    let context: ActivityViewContext<SyncActivityAttributes>

    var body: some View {
        HStack(spacing: 14) {
            ZStack {
                Circle()
                    .fill(Color(red: 0.93, green: 0.18, blue: 0.28).opacity(0.12))
                    .frame(width: 46, height: 46)
                Image("HeartRate")
                    .resizable()
                    .scaledToFit()
                    .frame(width: 26, height: 26)
                    .foregroundStyle(Color(red: 0.93, green: 0.18, blue: 0.28))
            }

            VStack(alignment: .leading, spacing: 4) {
                HStack {
                    Text("FreeReps")
                        .font(.subheadline.weight(.semibold))
                    Spacer()
                    Text("\(context.state.recordsInserted.formatted()) records")
                        .font(.subheadline.monospacedDigit().weight(.semibold))
                        .foregroundStyle(.red)
                }
                Text(context.state.operation)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
                ProgressView()
                    .progressViewStyle(.linear)
                    .tint(.red)
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
    }
}
