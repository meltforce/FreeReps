import { useState, useEffect } from "react";
import { Workout } from "../../api";
import WorkoutCard from "./WorkoutCard";

const PAGE_SIZE = 20;

interface Props {
  workouts: Workout[];
  typeFilter: string;
  onTypeFilter: (type: string) => void;
}

export default function WorkoutList({
  workouts,
  typeFilter,
  onTypeFilter,
}: Props) {
  const [page, setPage] = useState(0);

  // Reset to page 0 when filter changes
  useEffect(() => {
    setPage(0);
  }, [typeFilter, workouts.length]);

  // Collect unique workout types
  const types = [...new Set(workouts.map((w) => w.Name))].sort();

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

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {pageWorkouts.map((w) => (
          <WorkoutCard key={w.ID} workout={w} />
        ))}
      </div>

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
