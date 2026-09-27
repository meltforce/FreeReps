import { useQuery } from "@tanstack/react-query";
import { Link, useLocation, useParams } from "react-router-dom";
import { fetchMaxHeartRate, fetchWorkoutDetail, type Workout } from "../api";
import PageHeader from "../components/PageHeader";
import HRTimelineChart from "../components/workouts/HRTimelineChart";
import HRZoneBars from "../components/workouts/HRZoneBars";
import RouteMap from "../components/workouts/RouteMap";
import WorkoutSets from "../components/workouts/WorkoutSets";
import { getWorkoutDisplayName } from "../components/workouts/workoutNames";
import { useIsDesktop } from "../hooks/useMediaQuery";
import { distanceKm, formatDistance, formatDuration, formatNumber } from "../utils/format";

export default function WorkoutDetailPage() {
  const { id } = useParams<{ id: string }>();
  const isDesktop = useIsDesktop();
  const location = useLocation();
  const routeWorkout = (location.state as { workout?: Workout } | null)?.workout;
  // Sessions that live only in workout_sets have no row in the workouts table,
  // so their id cannot be fetched — the list already carries everything shown.
  const isSynthetic =
    routeWorkout?.Source === "Alpha Progression" || routeWorkout?.Source === "Hevy";

  const { data, isLoading, error } = useQuery({
    queryKey: ["workout", id],
    queryFn: () => fetchWorkoutDetail(id!),
    enabled: !!id && !isSynthetic,
  });
  // The same edges the workout list colours its bars with. Without them the
  // zones were fractions of this session's own peak, which puts an easy run
  // in the top zones.
  const zonesQuery = useQuery({
    queryKey: ["max-heart-rate"],
    queryFn: fetchMaxHeartRate,
    enabled: !isSynthetic,
  });
  const zoneEdges = zonesQuery.data?.zone_edges;

  const w = isSynthetic ? routeWorkout! : data;

  if (!isSynthetic && isLoading) {
    return (
      <div className="page-x" style={{ paddingTop: 22 }}>
        <span className="skel" style={{ width: 220, height: 34 }} />
      </div>
    );
  }

  if (!w || (!isSynthetic && error)) {
    return (
      <>
        <PageHeader kicker="Workout" title="Not found" />
        <p
          className="page-x"
          style={{ color: "var(--color-neutral-600)", fontSize: 13 }}
        >
          This workout is no longer in the database.
        </p>
      </>
    );
  }

  const hasHR = !isSynthetic && data?.HeartRateData && data.HeartRateData.length > 0;
  const hasRoute = !isSynthetic && data?.RouteData && data.RouteData.length > 0;

  const stats: { label: string; value: string; unit?: string }[] = [
    { label: "Duration", value: formatDuration(w.DurationSec) },
  ];
  if (w.ActiveEnergyBurned != null) {
    stats.push({
      label: "Active energy",
      value: formatNumber(w.ActiveEnergyBurned),
      unit: "kcal",
    });
  }
  if (w.AvgHeartRate != null) {
    stats.push({
      label: "Avg HR",
      value: formatNumber(w.AvgHeartRate),
      unit: "bpm",
    });
  }
  if (w.MaxHeartRate != null) {
    stats.push({
      label: "Max HR",
      value: formatNumber(w.MaxHeartRate),
      unit: "bpm",
    });
  }
  if (w.Distance != null && w.Distance > 0) {
    stats.push({
      label: "Distance",
      value: formatDistance(w.Distance, w.DistanceUnits).replace(" km", ""),
      unit: "km",
    });
  }
  if (w.ElevationUp != null && w.ElevationUp > 0) {
    stats.push({
      label: "Elevation",
      value: formatNumber(w.ElevationUp),
      unit: "m",
    });
  }

  return (
    <>
      <PageHeader
        kicker={new Date(w.StartTime).toLocaleDateString("en-GB", {
          weekday: "long",
          day: "numeric",
          month: "long",
          year: "numeric",
          hour: "2-digit",
          minute: "2-digit",
          hour12: false,
        })}
        title={getWorkoutDisplayName(w)}
        actions={
          <Link to="/workouts" className="btn btn-secondary" style={{ fontSize: 12 }}>
            ← Workouts
          </Link>
        }
      />

      <div
        style={{
          display: "grid",
          gridTemplateColumns: `repeat(${isDesktop ? Math.min(stats.length, 6) : 3}, 1fr)`,
          borderTop: "2px solid var(--color-text)",
          borderBottom: "2px solid var(--color-text)",
        }}
      >
        {stats.map((s) => (
          <div
            key={s.label}
            className="page-x"
            style={{
              paddingTop: isDesktop ? 24 : 14,
              paddingBottom: isDesktop ? 22 : 14,
              borderRight: "1px solid var(--color-divider)",
            }}
          >
            <div className="kick">{s.label}</div>
            <div
              className="num"
              style={{
                font: `800 ${isDesktop ? 44 : 26}px/1 var(--font-heading)`,
                letterSpacing: "-0.035em",
                marginTop: isDesktop ? 14 : 8,
              }}
            >
              {s.value}
              {s.unit ? (
                <span
                  style={{
                    font: `500 ${isDesktop ? 14 : 11}px var(--font-body)`,
                    color: "var(--color-neutral-600)",
                    marginLeft: 5,
                  }}
                >
                  {s.unit}
                </span>
              ) : null}
            </div>
          </div>
        ))}
      </div>

      <div className="page-x" style={{ paddingTop: 26, paddingBottom: 40 }}>
        <WorkoutSets
          workoutId={id!}
          workoutName={w.Name}
          alphaSessionName={w.alpha_session_name}
          workoutStart={isSynthetic ? w.StartTime : undefined}
          workoutEnd={isSynthetic ? w.EndTime : undefined}
        />

        {hasHR ? <HRTimelineChart hrData={data!.HeartRateData!} zoneEdges={zoneEdges} /> : null}
        {hasHR ? <HRZoneBars hrData={data!.HeartRateData!} zoneEdges={zoneEdges} /> : null}

        {/* Hidden for indoor or zero-distance workouts: there is no track. */}
        {hasRoute && !w.IsIndoor && (distanceKm(w.Distance, w.DistanceUnits) ?? 0) > 0.1 ? (
          <RouteMap route={data!.RouteData!} />
        ) : null}
      </div>
    </>
  );
}
