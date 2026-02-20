import { useState, useEffect } from "react";
import { Link } from "react-router-dom";
import { Workout } from "../../api";

const PAGE_SIZE = 10;

interface Props {
  workouts: Workout[];
  allWorkouts: Workout[];
  typeFilter: string;
  onTypeFilter: (type: string) => void;
}

export default function WorkoutList({
  workouts,
  allWorkouts,
  typeFilter,
  onTypeFilter,
}: Props) {
  const [page, setPage] = useState(0);

  // Reset to page 0 when filter changes
  useEffect(() => {
    setPage(0);
  }, [typeFilter, workouts.length]);

  // Collect unique workout types from the full dataset so pills remain visible when filtered
  const types = [...new Set(allWorkouts.map((w) => w.Name))].sort();

  const totalPages = Math.ceil(workouts.length / PAGE_SIZE);
  const pageStart = page * PAGE_SIZE;
  const pageEnd = pageStart + PAGE_SIZE;
  const pageWorkouts = workouts.slice(pageStart, pageEnd);

  return (
    <div className="space-y-4">
      {types.length > 1 && (
        <div className="flex flex-wrap gap-1 pb-1">
          <FilterPill
            label="All"
            active={typeFilter === ""}
            onClick={() => onTypeFilter("")}
          />
          {types.map((t) => (
            <FilterPill
              key={t}
              label={t}
              active={typeFilter === t}
              onClick={() => onTypeFilter(t)}
            />
          ))}
        </div>
      )}

      <WorkoutListView workouts={pageWorkouts} />

      {workouts.length === 0 && (
        <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
          No workouts found in this time range.
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between pt-2">
          <span className="text-sm text-zinc-500">
            Showing {pageStart + 1}-{Math.min(pageEnd, workouts.length)} of{" "}
            {workouts.length}
          </span>
          <div className="flex gap-2">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className="px-3 py-1.5 rounded-md text-sm font-medium transition-colors
                         bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200
                         disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Prev
            </button>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className="px-3 py-1.5 rounded-md text-sm font-medium transition-colors
                         bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200
                         disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function formatDuration(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function WorkoutListView({ workouts }: { workouts: Workout[] }) {
  // Group by date
  const groups: { date: string; workouts: Workout[] }[] = [];
  let currentDate = "";
  for (const w of workouts) {
    const d = new Date(w.StartTime).toLocaleDateString("de-DE", {
      weekday: "short",
      day: "numeric",
      month: "short",
    });
    if (d !== currentDate) {
      groups.push({ date: d, workouts: [w] });
      currentDate = d;
    } else {
      groups[groups.length - 1].workouts.push(w);
    }
  }

  return (
    <div className="space-y-1">
      {groups.map((group) => (
        <div key={group.date}>
          <div className="text-xs text-zinc-500 font-medium px-3 py-1.5 bg-zinc-900/50">
            {group.date}
          </div>
          {group.workouts.map((w) => (
            <Link
              key={w.ID}
              to={`/workouts/${w.ID}`}
              className="flex items-center gap-4 px-3 py-2.5 bg-zinc-900 border border-zinc-800 hover:border-zinc-700 transition-colors text-sm"
            >
              <span className="text-zinc-500 text-xs w-12 shrink-0">
                {new Date(w.StartTime).toLocaleTimeString("de-DE", {
                  hour: "2-digit",
                  minute: "2-digit",
                  hour12: false,
                })}
              </span>
              <span className="text-zinc-100 font-medium min-w-0 truncate flex-1">
                {w.Name}
              </span>
              <span className="text-zinc-400 tabular-nums shrink-0">
                {formatDuration(w.DurationSec)}
              </span>
              {w.AvgHeartRate != null && (
                <span className="text-zinc-400 tabular-nums shrink-0 hidden sm:inline">
                  {Math.round(w.AvgHeartRate)}
                  {w.MaxHeartRate != null && `/${Math.round(w.MaxHeartRate)}`} bpm
                </span>
              )}
              {w.ActiveEnergyBurned != null && (
                <span className="text-zinc-400 tabular-nums shrink-0 hidden sm:inline">
                  {Math.round(w.ActiveEnergyBurned)} kcal
                </span>
              )}
              {w.Distance != null && w.Distance > 0 && (
                <span className="text-zinc-400 tabular-nums shrink-0 hidden md:inline">
                  {w.Distance.toFixed(2)} {w.DistanceUnits}
                </span>
              )}
              {w.ElevationUp != null && w.ElevationUp > 0 && (
                <span className="text-zinc-400 tabular-nums shrink-0 hidden md:inline">
                  ↑{Math.round(w.ElevationUp)}m
                </span>
              )}
            </Link>
          ))}
        </div>
      ))}
    </div>
  );
}

function FilterPill({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors ${
        active
          ? "bg-cyan-600 text-white"
          : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200"
      }`}
    >
      {label}
    </button>
  );
}
