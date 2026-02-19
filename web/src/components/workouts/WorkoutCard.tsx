import { Link } from "react-router-dom";
import { Workout } from "../../api";

function formatDuration(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function WorkoutCard({ workout }: { workout: Workout }) {
  return (
    <Link
      to={`/workouts/${workout.ID}`}
      className="block bg-zinc-900 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition-colors"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="font-medium text-zinc-100 truncate">
            {workout.Name}
          </div>
          <div className="text-xs text-zinc-500 mt-1">
            {formatDate(workout.StartTime)}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-4 mt-3 text-sm">
        <Stat label="Duration" value={formatDuration(workout.DurationSec)} />
        {workout.AvgHeartRate != null && (
          <Stat
            label="Avg HR"
            value={`${Math.round(workout.AvgHeartRate)} bpm`}
          />
        )}
        {workout.ActiveEnergyBurned != null && (
          <Stat
            label="Calories"
            value={`${Math.round(workout.ActiveEnergyBurned)} kcal`}
          />
        )}
        {workout.Distance != null && workout.Distance > 0 && (
          <Stat
            label="Distance"
            value={`${workout.Distance.toFixed(2)} ${workout.DistanceUnits}`}
          />
        )}
      </div>
    </Link>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span className="text-zinc-500">{label}</span>{" "}
      <span className="text-zinc-200 font-medium tabular-nums">{value}</span>
    </div>
  );
}
