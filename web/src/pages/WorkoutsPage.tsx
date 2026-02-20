import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchWorkouts } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import WorkoutList from "../components/workouts/WorkoutList";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

export default function WorkoutsPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [typeFilter, setTypeFilter] = useState("");
  const [offset, setOffset] = useState(0);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  const { data, isLoading, error } = useQuery({
    queryKey: ["workouts", start, end],
    queryFn: () => fetchWorkouts(start, end),
  });

  const filtered =
    typeFilter && data ? data.filter((w) => w.Name === typeFilter) : (data ?? []);

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 bg-zinc-900 rounded animate-pulse" />
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="bg-zinc-900 rounded-lg p-4 animate-pulse h-28"
            />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
        Failed to load workouts.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Workouts</h2>
        <TimeRangeSelector
          value={timeRange}
          onChange={(v) => { setTimeRange(v as TimeRange); setOffset(0); }}
          options={["1d", "7d", "30d", "90d", "1y"]}
          onPrev={() => setOffset((o) => o + 1)}
          onNext={() => setOffset((o) => Math.max(0, o - 1))}
          canGoNext={offset > 0}
          dateLabel={formatDateLabel(start, end)}
        />
      </div>

      <WorkoutList
        workouts={filtered}
        allWorkouts={data ?? []}
        typeFilter={typeFilter}
        onTypeFilter={setTypeFilter}
      />
    </div>
  );
}
