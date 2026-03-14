import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { fetchWorkouts } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import WorkoutList from "../components/workouts/WorkoutList";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

const STORAGE_KEY = "workouts-filters";

export default function WorkoutsPage() {
  const [searchParams, setSearchParams] = useSearchParams();

  // Restore filters from sessionStorage when navigating to bare /workouts
  useEffect(() => {
    if (!searchParams.has("range") && !searchParams.has("type") && !searchParams.has("offset")) {
      const saved = sessionStorage.getItem(STORAGE_KEY);
      if (saved) {
        setSearchParams(new URLSearchParams(saved), { replace: true });
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // run once on mount

  // Save filters to sessionStorage whenever they change
  useEffect(() => {
    if (searchParams.has("range") || searchParams.has("type") || searchParams.has("offset")) {
      sessionStorage.setItem(STORAGE_KEY, searchParams.toString());
    }
  }, [searchParams]);

  const timeRange = (searchParams.get("range") || "90d") as TimeRange;
  const typeFilter = searchParams.get("type") || "";
  const offset = parseInt(searchParams.get("offset") || "0", 10);

  const setTimeRange = (v: TimeRange) => {
    setSearchParams((prev) => {
      prev.set("range", v);
      prev.set("offset", "0");
      return prev;
    }, { replace: true });
  };

  const setTypeFilter = (v: string) => {
    setSearchParams((prev) => {
      if (v) {
        prev.set("type", v);
      } else {
        prev.delete("type");
      }
      return prev;
    }, { replace: true });
  };

  const setOffset = (updater: (prev: number) => number) => {
    setSearchParams((prev) => {
      const next = updater(parseInt(prev.get("offset") || "0", 10));
      prev.set("offset", String(next));
      return prev;
    }, { replace: true });
  };

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
          onChange={(v) => setTimeRange(v as TimeRange)}
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
