import { useEffect, useState } from "react";
import { fetchStats, type DataStats } from "../../api";

function formatDuration(sec: number): string {
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function formatDistance(km: number): string {
  return `${km.toFixed(1)} km`;
}

export default function StatsTab() {
  const [stats, setStats] = useState<DataStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchStats().then(setStats).catch((e) => setError(e.message));
  }, []);

  if (error) {
    return <p className="text-red-400">Failed to load stats: {error}</p>;
  }
  if (!stats) {
    return <p className="text-zinc-500">Loading...</p>;
  }

  const summaryCards = [
    { label: "Metric Rows", value: stats.total_metric_rows.toLocaleString() },
    { label: "Workouts", value: stats.total_workouts.toLocaleString() },
    {
      label: "Sleep Nights",
      value: stats.total_sleep_nights.toLocaleString(),
    },
    { label: "Exercise Sets", value: stats.total_sets.toLocaleString() },
    {
      label: "Data Range",
      value:
        stats.earliest_data && stats.latest_data
          ? `${new Date(stats.earliest_data).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" })} — ${new Date(stats.latest_data).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" })}`
          : "—",
    },
  ];

  return (
    <div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 mb-8">
        {summaryCards.map((c) => (
          <div
            key={c.label}
            className="bg-zinc-900 border border-zinc-800 rounded-lg p-4"
          >
            <p className="text-xs text-zinc-500 uppercase tracking-wide mb-1">
              {c.label}
            </p>
            <p className="text-xl font-semibold text-zinc-100">{c.value}</p>
          </div>
        ))}
      </div>

      {stats.workouts_by_type && stats.workouts_by_type.length > 0 && (
        <div>
          <h3 className="text-lg font-semibold text-zinc-100 mb-3">
            Workouts by Type
          </h3>
          <div className="overflow-x-auto">
            <table className="w-full text-sm text-left">
              <thead>
                <tr className="border-b border-zinc-800 text-zinc-500 text-xs uppercase">
                  <th className="pb-2 pr-4">Type</th>
                  <th className="pb-2 pr-4 text-right">Count</th>
                  <th className="pb-2 pr-4 text-right">Total Duration</th>
                  <th className="pb-2 text-right">Total Distance</th>
                </tr>
              </thead>
              <tbody>
                {stats.workouts_by_type.map((w) => (
                  <tr
                    key={w.name}
                    className="border-b border-zinc-800/50 text-zinc-300"
                  >
                    <td className="py-2 pr-4">{w.name}</td>
                    <td className="py-2 pr-4 text-right">{w.count}</td>
                    <td className="py-2 pr-4 text-right">
                      {formatDuration(w.total_duration_sec)}
                    </td>
                    <td className="py-2 text-right">
                      {w.total_distance
                        ? formatDistance(w.total_distance)
                        : "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
