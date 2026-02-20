import { useQuery } from "@tanstack/react-query";
import { useParams, Link } from "react-router-dom";
import { fetchWorkoutDetail } from "../api";
import HRTimelineChart from "../components/workouts/HRTimelineChart";
import HRZoneBars from "../components/workouts/HRZoneBars";
import RouteMap from "../components/workouts/RouteMap";
import WorkoutSets from "../components/workouts/WorkoutSets";

function formatDuration(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function WorkoutDetailPage() {
  const { id } = useParams<{ id: string }>();

  const { data, isLoading, error } = useQuery({
    queryKey: ["workout", id],
    queryFn: () => fetchWorkoutDetail(id!),
    enabled: !!id,
  });

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-64 bg-zinc-900 rounded animate-pulse" />
        <div className="bg-zinc-900 rounded-lg p-4 animate-pulse h-72" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
        Workout not found.
      </div>
    );
  }

  const w = data;
  const hasHR = w.HeartRateData && w.HeartRateData.length > 0;
  const hasRoute = w.RouteData && w.RouteData.length > 0;

  return (
    <div className="space-y-6">
      <div>
        <Link
          to="/workouts"
          className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors"
        >
          &larr; Back to Workouts
        </Link>
      </div>

      <div>
        <h2 className="text-xl font-semibold text-zinc-100">{w.Name}</h2>
        <div className="text-sm text-zinc-500 mt-1">
          {new Date(w.StartTime).toLocaleDateString("de-DE", {
            weekday: "long",
            year: "numeric",
            month: "long",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
            hour12: false,
          })}
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
        <StatCard label="Duration" value={formatDuration(w.DurationSec)} />
        {w.ActiveEnergyBurned != null && (
          <StatCard
            label="Active Cal"
            value={`${Math.round(w.ActiveEnergyBurned)}`}
            unit="kcal"
          />
        )}
        {w.AvgHeartRate != null && (
          <StatCard
            label="Avg HR"
            value={`${Math.round(w.AvgHeartRate)}`}
            unit="bpm"
          />
        )}
        {w.MaxHeartRate != null && (
          <StatCard
            label="Max HR"
            value={`${Math.round(w.MaxHeartRate)}`}
            unit="bpm"
          />
        )}
        {w.Distance != null && w.Distance > 0 && (
          <StatCard
            label="Distance"
            value={w.Distance.toFixed(2)}
            unit={w.DistanceUnits}
          />
        )}
        {w.ElevationUp != null && w.ElevationUp > 0 && (
          <StatCard
            label="Elev. Gain"
            value={`${Math.round(w.ElevationUp)}`}
            unit="m"
          />
        )}
      </div>

      {/* Workout Sets (Alpha Progression data) */}
      <WorkoutSets workoutId={id!} workoutName={w.Name} />

      {/* HR Timeline */}
      {hasHR && <HRTimelineChart hrData={w.HeartRateData!} />}

      {/* HR Zones */}
      {hasHR && <HRZoneBars hrData={w.HeartRateData!} />}

      {/* Route Map — hidden for indoor or zero-distance workouts */}
      {hasRoute && !w.IsIndoor && (w.Distance ?? 0) > 0.1 && (
        <RouteMap route={w.RouteData!} />
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  unit,
}: {
  label: string;
  value: string;
  unit?: string;
}) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <div className="text-xs text-zinc-500 mb-1">{label}</div>
      <div className="text-xl font-semibold text-zinc-100 tabular-nums">
        {value}
        {unit && (
          <span className="text-sm text-zinc-500 ml-1">{unit}</span>
        )}
      </div>
    </div>
  );
}
