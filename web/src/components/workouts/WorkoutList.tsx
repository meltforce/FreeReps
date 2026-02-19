import { Workout } from "../../api";
import WorkoutCard from "./WorkoutCard";

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
  // Collect unique workout types
  const types = [...new Set(workouts.map((w) => w.Name))].sort();

  return (
    <div className="space-y-4">
      {types.length > 1 && (
        <div className="flex gap-1 overflow-x-auto scrollbar-none pb-1">
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
        {workouts.map((w) => (
          <WorkoutCard key={w.ID} workout={w} />
        ))}
      </div>

      {workouts.length === 0 && (
        <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
          No workouts found in this time range.
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
