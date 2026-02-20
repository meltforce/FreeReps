import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchWorkouts } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import WorkoutList from "../components/workouts/WorkoutList";

type TimeRange = "1d" | "30d" | "90d" | "1y";

function daysFromRange(range_: TimeRange): number {
  switch (range_) {
    case "1d":
      return 1;
    case "30d":
      return 30;
    case "90d":
      return 90;
    case "1y":
      return 365;
  }
}

export default function WorkoutsPage() {
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [typeFilter, setTypeFilter] = useState("");

  const end = new Date().toISOString().split("T")[0];
  const start = new Date(Date.now() - daysFromRange(timeRange) * 86400000)
    .toISOString()
    .split("T")[0];

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
          onChange={(v) => setTimeRange(v as TimeRange)}
          options={["1d", "30d", "90d", "1y"]}
        />
      </div>

      <WorkoutList
        workouts={filtered}
        typeFilter={typeFilter}
        onTypeFilter={setTypeFilter}
      />
    </div>
  );
}
