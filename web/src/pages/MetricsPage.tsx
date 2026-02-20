import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchTimeSeries, fetchMetricStats } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import MetricStatsBar from "../components/metrics/MetricStatsBar";
import MetricTimeSeriesChart from "../components/metrics/MetricTimeSeriesChart";

const METRICS = [
  { value: "heart_rate", label: "Heart Rate", unit: "bpm" },
  { value: "resting_heart_rate", label: "Resting HR", unit: "bpm" },
  { value: "heart_rate_variability", label: "HRV", unit: "ms" },
  { value: "blood_oxygen_saturation", label: "SpO2", unit: "%" },
  { value: "respiratory_rate", label: "Resp. Rate", unit: "brpm" },
  { value: "vo2_max", label: "VO2 Max", unit: "mL/kg/min" },
  { value: "weight_body_mass", label: "Weight", unit: "kg" },
  { value: "body_fat_percentage", label: "Body Fat", unit: "%" },
  { value: "active_energy", label: "Active Energy", unit: "kcal" },
  { value: "basal_energy_burned", label: "Basal Energy", unit: "kcal" },
  { value: "apple_exercise_time", label: "Exercise Time", unit: "min" },
];

type TimeRange = "1d" | "7d" | "30d" | "90d" | "1y";

function daysFromRange(range_: TimeRange): number {
  switch (range_) {
    case "1d":
      return 1;
    case "7d":
      return 7;
    case "30d":
      return 30;
    case "90d":
      return 90;
    case "1y":
      return 365;
  }
}

export default function MetricsPage() {
  const [metric, setMetric] = useState("resting_heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");

  const end = new Date().toISOString().split("T")[0];
  const start = new Date(Date.now() - daysFromRange(timeRange) * 86400000)
    .toISOString()
    .split("T")[0];

  const selected = METRICS.find((m) => m.value === metric)!;

  const agg = timeRange === "1d" ? "hourly" : "daily";
  const { data: tsData, isLoading: tsLoading } = useQuery({
    queryKey: ["timeseries", metric, start, end, agg],
    queryFn: () => fetchTimeSeries(metric, start, end, agg),
  });

  const { data: statsData } = useQuery({
    queryKey: ["metricStats", metric, start, end],
    queryFn: () => fetchMetricStats(metric, start, end),
  });

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Metrics</h2>
        <TimeRangeSelector
          value={timeRange}
          onChange={(v) => setTimeRange(v as TimeRange)}
          options={["1d", "7d", "30d", "90d", "1y"]}
        />
      </div>

      {/* Metric selector — sidebar on desktop, horizontal scroll on mobile */}
      <div className="flex gap-6">
        <div className="hidden lg:block shrink-0 w-48 space-y-1">
          {METRICS.map((m) => (
            <button
              key={m.value}
              onClick={() => setMetric(m.value)}
              className={`w-full text-left px-3 py-2 rounded-md text-sm transition-colors ${
                metric === m.value
                  ? "bg-cyan-600/20 text-cyan-400 font-medium"
                  : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
              }`}
            >
              {m.label}
            </button>
          ))}
        </div>

        <div className="flex-1 min-w-0 space-y-4">
          {/* Mobile metric selector */}
          <div className="lg:hidden">
            <select
              value={metric}
              onChange={(e) => setMetric(e.target.value)}
              className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-3 py-1.5 text-sm w-full
                         focus:outline-none focus:ring-1 focus:ring-cyan-500"
            >
              {METRICS.map((m) => (
                <option key={m.value} value={m.value}>
                  {m.label}
                </option>
              ))}
            </select>
          </div>

          {/* Stats bar */}
          {statsData && <MetricStatsBar stats={statsData} />}

          {/* Chart */}
          {tsLoading ? (
            <div className="bg-zinc-900 rounded-lg p-6 h-[340px] animate-pulse" />
          ) : (
            <MetricTimeSeriesChart
              data={tsData ?? []}
              stats={statsData ?? null}
              label={selected.label}
              unit={selected.unit}
              start={start}
              end={end}
            />
          )}
        </div>
      </div>
    </div>
  );
}
