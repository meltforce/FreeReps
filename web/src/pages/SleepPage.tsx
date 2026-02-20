import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchSleep, SleepStage } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import SleepMetricCards from "../components/sleep/SleepMetricCards";
import Hypnogram from "../components/sleep/Hypnogram";
import SleepHistoryChart from "../components/sleep/SleepHistoryChart";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

export default function SleepPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>("30d");
  const [offset, setOffset] = useState(0);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  const { data, isLoading, error } = useQuery({
    queryKey: ["sleep", start, end],
    queryFn: () => fetchSleep(start, end),
  });

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 bg-zinc-900 rounded animate-pulse" />
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="bg-zinc-900 rounded-lg p-4 animate-pulse h-20"
            />
          ))}
        </div>
        <div className="bg-zinc-900 rounded-lg p-4 animate-pulse h-48" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
        Failed to load sleep data.
      </div>
    );
  }

  const sessions = data.sessions ?? [];
  const stages = data.stages ?? [];

  // Most recent session for the summary / hypnogram
  const lastSession = sessions.length > 0 ? sessions[0] : null;

  // Filter stages belonging to the most recent night using session timestamps
  const lastNightStages: SleepStage[] = lastSession
    ? stages.filter((s) => {
        const st = new Date(s.StartTime).getTime();
        return (
          st >= new Date(lastSession.SleepStart).getTime() &&
          st < new Date(lastSession.SleepEnd).getTime()
        );
      })
    : [];

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Sleep</h2>
        <TimeRangeSelector
          value={timeRange}
          onChange={(v) => { setTimeRange(v as TimeRange); setOffset(0); }}
          options={["7d", "30d", "90d"]}
          onPrev={() => setOffset((o) => o + 1)}
          onNext={() => setOffset((o) => Math.max(0, o - 1))}
          canGoNext={offset > 0}
          dateLabel={formatDateLabel(start, end)}
        />
      </div>

      {/* Last Night Summary */}
      {lastSession ? (
        <div className="space-y-4">
          <h3 className="text-sm font-medium text-zinc-400">
            Last Night &mdash;{" "}
            {new Date(lastSession.Date).toLocaleDateString(undefined, {
              weekday: "short",
              month: "short",
              day: "numeric",
            })}
          </h3>
          <SleepMetricCards session={lastSession} />
        </div>
      ) : (
        <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
          No sleep sessions found in this time range.
        </div>
      )}

      {/* Hypnogram */}
      {lastNightStages.length > 0 && <Hypnogram stages={lastNightStages} />}

      {/* Sleep History */}
      {sessions.length > 0 && (
        <SleepHistoryChart sessions={sessions} stages={stages} start={start} end={end} />
      )}
    </div>
  );
}
