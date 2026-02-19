import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchSleep, SleepStage } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import SleepMetricCards from "../components/sleep/SleepMetricCards";
import Hypnogram from "../components/sleep/Hypnogram";
import SleepHistoryChart from "../components/sleep/SleepHistoryChart";

type TimeRange = "7d" | "30d" | "90d";

function daysFromRange(range_: TimeRange): number {
  switch (range_) {
    case "7d":
      return 7;
    case "30d":
      return 30;
    case "90d":
      return 90;
  }
}

export default function SleepPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>("30d");

  const end = new Date().toISOString().split("T")[0];
  const start = new Date(Date.now() - daysFromRange(timeRange) * 86400000)
    .toISOString()
    .split("T")[0];

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

  // Filter stages belonging to the most recent night
  const lastNightStages: SleepStage[] = lastSession
    ? stages.filter((s) => {
        const stageDate = new Date(s.StartTime).toISOString().split("T")[0];
        const sessionDate = lastSession.Date.split("T")[0];
        // Stages from the night before or the session date
        const dayBefore = new Date(
          new Date(sessionDate).getTime() - 86400000
        )
          .toISOString()
          .split("T")[0];
        return stageDate === sessionDate || stageDate === dayBefore;
      })
    : [];

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Sleep</h2>
        <TimeRangeSelector
          value={timeRange}
          onChange={(v) => setTimeRange(v as TimeRange)}
          options={["7d", "30d", "90d"]}
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
      {sessions.length > 1 && <SleepHistoryChart sessions={sessions} />}
    </div>
  );
}
