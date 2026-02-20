import { useMemo } from "react";
import { SleepSession, SleepStage } from "../../api";

const STAGE_COLORS: Record<string, string> = {
  Deep: "#6366f1",
  Core: "#3b82f6",
  REM: "#8b5cf6",
  Awake: "#f59e0b",
};

interface Props {
  sessions: SleepSession[];
  stages: SleepStage[];
}

/**
 * Apple Health-style sleep timeline chart.
 * Y-axis = clock time (inverted: evening at top, morning at bottom).
 * Each night is a column with stage-colored blocks positioned by time.
 */
export default function SleepHistoryChart({ sessions, stages }: Props) {
  const chartData = useMemo(() => {
    if (!sessions || sessions.length === 0) return null;

    // Sort ascending by date
    const sorted = [...sessions].sort(
      (a, b) => new Date(a.SleepStart).getTime() - new Date(b.SleepStart).getTime()
    );

    // Find the overall time window (earliest bedtime, latest wake time)
    // Normalize to "hours from 18:00" so we can handle midnight crossing
    const getOffsetHours = (date: Date): number => {
      let h = date.getHours() + date.getMinutes() / 60;
      // Shift so 18:00 = 0, 00:00 = 6, 06:00 = 12, 12:00 = 18
      if (h >= 18) return h - 18;
      return h + 6; // after midnight
    };

    let minOffset = 24;
    let maxOffset = 0;
    for (const s of sorted) {
      const startOff = getOffsetHours(new Date(s.SleepStart));
      const endOff = getOffsetHours(new Date(s.SleepEnd));
      if (startOff < minOffset) minOffset = startOff;
      if (endOff > maxOffset) maxOffset = endOff;
    }

    // Add some padding
    minOffset = Math.floor(minOffset);
    maxOffset = Math.ceil(maxOffset);
    const totalRange = maxOffset - minOffset;

    // Generate Y-axis labels (clock times)
    const yLabels: string[] = [];
    for (let off = minOffset; off <= maxOffset; off += 2) {
      let hour = off + 18;
      if (hour >= 24) hour -= 24;
      yLabels.push(`${hour.toString().padStart(2, "0")}:00`);
    }

    // Map stages to sessions
    const sessionsWithStages = sorted.map((session) => {
      const sessionStart = new Date(session.SleepStart).getTime();
      const sessionEnd = new Date(session.SleepEnd).getTime();
      const sessionStages = stages.filter((st) => {
        const t = new Date(st.StartTime).getTime();
        return t >= sessionStart && t < sessionEnd;
      });
      return { session, stages: sessionStages };
    });

    // Compute average sleep duration
    const totalSleepHours = sorted.reduce((sum, s) => sum + s.TotalSleep, 0);
    const avgSleepHours = totalSleepHours / sorted.length;
    const avgH = Math.floor(avgSleepHours);
    const avgM = Math.round((avgSleepHours - avgH) * 60);

    return {
      sessionsWithStages,
      minOffset,
      maxOffset,
      totalRange,
      yLabels,
      avgLabel: `${avgH}h ${avgM}m`,
      count: sorted.length,
    };
  }, [sessions, stages]);

  if (!chartData || chartData.count === 0) {
    return (
      <div className="bg-zinc-900 rounded-lg p-6 text-zinc-500 text-sm">
        No sleep history data.
      </div>
    );
  }

  const { sessionsWithStages, minOffset, totalRange, yLabels, avgLabel, count } =
    chartData;
  const showStages = count <= 60; // Show stage detail for up to ~2 months

  const getOffsetHours = (date: Date): number => {
    let h = date.getHours() + date.getMinutes() / 60;
    if (h >= 18) return h - 18;
    return h + 6;
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-medium text-zinc-400">Sleep History</h3>
        <span className="text-sm text-zinc-300">
          Avg: <span className="font-medium text-zinc-100">{avgLabel}</span>
        </span>
      </div>

      <div className="flex gap-2">
        {/* Chart area */}
        <div
          className="flex-1 min-w-0 relative"
          style={{ height: `${Math.max(totalRange * 20, 120)}px` }}
        >
          {/* Grid lines */}
          {yLabels.map((_, i) => {
            const off = minOffset + i * 2;
            const top = ((off - minOffset) / totalRange) * 100;
            return (
              <div
                key={i}
                className="absolute left-0 right-0 border-t border-zinc-800"
                style={{ top: `${top}%` }}
              />
            );
          })}

          {/* Session columns */}
          <div className="absolute inset-0 flex items-stretch">
            {sessionsWithStages.map(({ session, stages: sessionStages }, idx) => {
              const sleepStart = new Date(session.SleepStart);
              const sleepEnd = new Date(session.SleepEnd);
              const startOff = getOffsetHours(sleepStart);
              const endOff = getOffsetHours(sleepEnd);
              const topPct = ((startOff - minOffset) / totalRange) * 100;
              const heightPct = ((endOff - startOff) / totalRange) * 100;

              // Date label
              const dayLabel = sleepStart.toLocaleDateString(undefined, {
                weekday: count <= 14 ? "short" : undefined,
                day: "numeric",
              });

              return (
                <div
                  key={idx}
                  className="flex-1 flex flex-col items-center relative"
                  style={{ minWidth: 0 }}
                >
                  {showStages && sessionStages.length > 0 ? (
                    // Render individual stage blocks
                    sessionStages.map((stage, si) => {
                      const stStart = getOffsetHours(new Date(stage.StartTime));
                      const stEnd = getOffsetHours(new Date(stage.EndTime));
                      const stTop = ((stStart - minOffset) / totalRange) * 100;
                      const stHeight = ((stEnd - stStart) / totalRange) * 100;
                      return (
                        <div
                          key={si}
                          className="absolute rounded-[2px]"
                          style={{
                            top: `${stTop}%`,
                            height: `${Math.max(stHeight, 0.5)}%`,
                            left: "15%",
                            right: "15%",
                            backgroundColor:
                              STAGE_COLORS[stage.Stage] ?? "#71717a",
                            opacity: 0.85,
                          }}
                          title={`${stage.Stage} ${new Date(stage.StartTime).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })} - ${new Date(stage.EndTime).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })}`}
                        />
                      );
                    })
                  ) : (
                    // Simplified: single bar for the session
                    <div
                      className="absolute rounded-[2px]"
                      style={{
                        top: `${topPct}%`,
                        height: `${Math.max(heightPct, 1)}%`,
                        left: "15%",
                        right: "15%",
                        backgroundColor: "#3b82f6",
                        opacity: 0.6,
                      }}
                      title={`${sleepStart.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })} - ${sleepEnd.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false })}`}
                    />
                  )}

                  {/* Date label at bottom */}
                  <span className="absolute bottom-0 translate-y-full pt-1 text-[10px] text-zinc-500 whitespace-nowrap">
                    {dayLabel}
                  </span>
                </div>
              );
            })}
          </div>
        </div>

        {/* Y-axis labels (right side) */}
        <div
          className="shrink-0 w-12 relative"
          style={{ height: `${Math.max(totalRange * 20, 120)}px` }}
        >
          {yLabels.map((label, i) => {
            const off = minOffset + i * 2;
            const top = ((off - minOffset) / totalRange) * 100;
            return (
              <span
                key={i}
                className="absolute text-xs text-zinc-500 right-0 -translate-y-1/2"
                style={{ top: `${top}%` }}
              >
                {label}
              </span>
            );
          })}
        </div>
      </div>

      {/* Legend */}
      {showStages && (
        <div className="flex gap-4 mt-5 text-xs text-zinc-500 justify-center">
          {Object.entries(STAGE_COLORS).map(([stage, color]) => (
            <span key={stage} className="flex items-center gap-1">
              <span
                className="w-3 h-3 rounded-sm inline-block"
                style={{ backgroundColor: color, opacity: 0.85 }}
              />
              {stage}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
