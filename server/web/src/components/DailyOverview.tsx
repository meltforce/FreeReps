import { useQuery } from "@tanstack/react-query";
import { fetchLatestMetrics, HealthMetricRow, DailySum } from "../api";

function getValue(row: HealthMetricRow): string {
  if (row.AvgVal !== null) return row.AvgVal.toFixed(0);
  if (row.Qty !== null) return row.Qty.toFixed(1);
  return "—";
}

// Map raw Apple Health units to human-friendly display units
const UNIT_MAP: Record<string, Record<string, string>> = {
  heart_rate: { "count/min": "bpm" },
  resting_heart_rate: { "count/min": "bpm" },
  respiratory_rate: { "count/min": "br/min" },
};

function displayUnit(metricName: string, rawUnit: string): string {
  return UNIT_MAP[metricName]?.[rawUnit] ?? rawUnit;
}

// Cumulative metrics that should show daily totals
const CUMULATIVE_METRICS = new Set([
  "active_energy",
  "basal_energy_burned",
  "apple_exercise_time",
]);

const DISPLAY_ORDER = [
  "resting_heart_rate",
  "heart_rate_variability",
  "heart_rate",
  "blood_oxygen_saturation",
  "respiratory_rate",
  "vo2_max",
  "weight_body_mass",
  "body_fat_percentage",
  "active_energy",
  "basal_energy_burned",
  "apple_exercise_time",
];

const LABELS: Record<string, string> = {
  heart_rate: "Heart Rate",
  resting_heart_rate: "Resting HR",
  heart_rate_variability: "HRV",
  blood_oxygen_saturation: "SpO2",
  respiratory_rate: "Resp. Rate",
  vo2_max: "VO2 Max",
  weight_body_mass: "Weight",
  body_fat_percentage: "Body Fat",
  active_energy: "Active Cal",
  basal_energy_burned: "Basal Cal",
  apple_exercise_time: "Exercise",
};

export default function DailyOverview() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["latestMetrics"],
    queryFn: fetchLatestMetrics,
  });

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="bg-zinc-900 rounded-lg p-4 animate-pulse h-20"
          />
        ))}
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
        No data yet. Send health data to the ingest API to get started.
      </div>
    );
  }

  const latestRows = data.latest ?? [];
  const dailySums = data.daily_sums ?? [];
  const sumMap = new Map(dailySums.map((s) => [s.MetricName, s]));

  const metricMap = new Map(latestRows.map((m) => [m.MetricName, m]));
  const ordered = DISPLAY_ORDER.filter((name) => metricMap.has(name)).map(
    (name) => metricMap.get(name)!
  );

  if (ordered.length === 0) {
    return (
      <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
        No data yet. Send health data to the ingest API to get started.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
      {ordered.map((m) => (
        <MetricCard
          key={m.MetricName}
          row={m}
          dailySum={sumMap.get(m.MetricName) ?? null}
        />
      ))}
    </div>
  );
}

function MetricCard({
  row,
  dailySum,
}: {
  row: HealthMetricRow;
  dailySum: DailySum | null;
}) {
  const label = LABELS[row.MetricName] ?? row.MetricName;
  const isCumulative = CUMULATIVE_METRICS.has(row.MetricName);

  // For cumulative metrics, show daily total; otherwise show latest value
  let value: string;
  let unit: string;
  let subtitle: string;

  if (isCumulative && dailySum) {
    let total = dailySum.Total;
    let displayUnits = dailySum.Units;
    // Convert kJ to kcal for display consistency
    if (row.MetricName === "basal_energy_burned" && displayUnits === "kJ") {
      total = total / 4.184;
      displayUnits = "kcal";
    }
    value = Math.round(total).toString();
    unit = displayUnit(row.MetricName, displayUnits);
    subtitle = "today";
  } else {
    value = getValue(row);
    unit = displayUnit(row.MetricName, row.Units);
    subtitle = formatTimeAgo(row.Time);
  }

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 hover:border-zinc-700 transition-colors">
      <div className="text-xs text-zinc-500 mb-1">{label}</div>
      <div className="text-2xl font-semibold text-zinc-100 tabular-nums">
        {value}
        <span className="text-sm text-zinc-500 ml-1">{unit}</span>
      </div>
      <div className="text-xs text-zinc-600 mt-1">{subtitle}</div>
    </div>
  );
}

function formatTimeAgo(iso: string): string {
  const d = new Date(iso);
  const now = Date.now();
  const diffMin = Math.floor((now - d.getTime()) / 60000);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay}d ago`;
}
