import { useQueries } from "@tanstack/react-query";
import { useState } from "react";
import { fetchTimeSeries, fetchMetricStats } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import TrendsChart, { type SeriesData } from "../components/trends/TrendsChart";
import TrendSummaryCards from "../components/trends/TrendSummaryCards";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

const COLORS = ["#22d3ee", "#a78bfa", "#fb923c", "#4ade80", "#f472b6"];

const METRIC_GROUPS = [
  {
    label: "Cardiovascular",
    metrics: [
      { value: "heart_rate", label: "Heart Rate", unit: "bpm" },
      { value: "resting_heart_rate", label: "Resting HR", unit: "bpm" },
      { value: "heart_rate_variability", label: "HRV", unit: "ms" },
      { value: "vo2_max", label: "VO2 Max", unit: "mL/kg/min" },
    ],
  },
  {
    label: "Sleep",
    metrics: [
      { value: "sleep_analysis", label: "Sleep Duration", unit: "hr" },
      { value: "blood_oxygen_saturation", label: "SpO2", unit: "%" },
      { value: "respiratory_rate", label: "Resp. Rate", unit: "brpm" },
      { value: "apple_sleeping_wrist_temperature", label: "Wrist Temp", unit: "°C" },
    ],
  },
  {
    label: "Body",
    metrics: [
      { value: "weight_body_mass", label: "Weight", unit: "kg" },
      { value: "body_fat_percentage", label: "Body Fat", unit: "%" },
    ],
  },
  {
    label: "Activity",
    metrics: [
      { value: "active_energy", label: "Active Energy", unit: "kcal" },
      { value: "apple_exercise_time", label: "Exercise Time", unit: "min" },
    ],
  },
];

const ALL_METRICS = METRIC_GROUPS.flatMap((g) => g.metrics);
const MAX_SELECTED = 5;
const RANGE_OPTIONS: TimeRange[] = ["30d", "90d", "1y"];

export default function TrendsPage() {
  const [selectedMetrics, setSelectedMetrics] = useState<string[]>([
    "heart_rate_variability",
    "resting_heart_rate",
  ]);
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [offset, setOffset] = useState(0);
  const [aggregation, setAggregation] = useState<"daily" | "weekly">("weekly");
  const [mobileOpen, setMobileOpen] = useState(false);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  // Previous period for % change comparison
  const prevEndDate = new Date(startDate.getTime());
  const prevStartDate = new Date(prevEndDate.getTime() - days * 86400000);
  const prevEnd = prevEndDate.toISOString().split("T")[0];
  const prevStart = prevStartDate.toISOString().split("T")[0];

  // Parallel data fetches
  const tsQueries = useQueries({
    queries: selectedMetrics.map((metric) => ({
      queryKey: ["timeseries", metric, start, end, aggregation],
      queryFn: () => fetchTimeSeries(metric, start, end, aggregation),
    })),
  });

  const statsQueries = useQueries({
    queries: selectedMetrics.map((metric) => ({
      queryKey: ["metricStats", metric, start, end],
      queryFn: () => fetchMetricStats(metric, start, end),
    })),
  });

  const prevStatsQueries = useQueries({
    queries: selectedMetrics.map((metric) => ({
      queryKey: ["metricStats", metric, prevStart, prevEnd],
      queryFn: () => fetchMetricStats(metric, prevStart, prevEnd),
    })),
  });

  const isLoading = tsQueries.some((q) => q.isLoading);

  function toggleMetric(value: string) {
    setSelectedMetrics((prev) => {
      if (prev.includes(value)) return prev.filter((m) => m !== value);
      if (prev.length >= MAX_SELECTED) return prev;
      return [...prev, value];
    });
  }

  function handleRangeChange(v: string) {
    const range = v as TimeRange;
    setTimeRange(range);
    setOffset(0);
    setAggregation(range === "30d" ? "daily" : "weekly");
  }

  // Build per-metric data used by both chart and summary cards
  const metricData = selectedMetrics.map((metric, i) => {
    const meta = ALL_METRICS.find((m) => m.value === metric);
    const label = meta?.label ?? metric;
    const unit = meta?.unit ?? "";
    const color = COLORS[i % COLORS.length];
    const points = tsQueries[i]?.data ?? [];
    return { metric, label, unit, color, points };
  });

  const seriesData: SeriesData[] = metricData;

  const summaryMetrics = metricData.map((d, i) => ({
    ...d,
    stats: statsQueries[i]?.data ?? null,
    prevStats: prevStatsQueries[i]?.data ?? null,
  }));

  const checkboxes = (
    <>
      {METRIC_GROUPS.map((group) => (
        <div key={group.label} className="space-y-1">
          <div className="text-xs font-medium text-zinc-500 uppercase tracking-wider px-1">
            {group.label}
          </div>
          {group.metrics.map((m) => {
            const checked = selectedMetrics.includes(m.value);
            const disabled = !checked && selectedMetrics.length >= MAX_SELECTED;
            return (
              <label
                key={m.value}
                className={`flex items-center gap-2 px-2 py-1.5 rounded-md text-sm cursor-pointer transition-colors ${
                  checked
                    ? "text-zinc-100"
                    : disabled
                      ? "text-zinc-600 cursor-not-allowed"
                      : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
                }`}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={disabled}
                  onChange={() => toggleMetric(m.value)}
                  className="accent-cyan-500 rounded"
                />
                {m.label}
              </label>
            );
          })}
        </div>
      ))}
    </>
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Trends</h2>
        <div className="flex items-center gap-2">
          <TimeRangeSelector
            value={timeRange}
            onChange={handleRangeChange}
            options={RANGE_OPTIONS}
            onPrev={() => setOffset((o) => o + 1)}
            onNext={() => setOffset((o) => Math.max(0, o - 1))}
            canGoNext={offset > 0}
            dateLabel={formatDateLabel(start, end)}
          />
          <div className="flex bg-zinc-800 rounded-md text-sm">
            {(["daily", "weekly"] as const).map((agg) => (
              <button
                key={agg}
                onClick={() => setAggregation(agg)}
                className={`px-3 py-1.5 rounded-md capitalize transition-colors ${
                  aggregation === agg
                    ? "bg-cyan-600 text-white"
                    : "text-zinc-400 hover:text-zinc-200"
                }`}
              >
                {agg}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="flex gap-6">
        {/* Desktop sidebar */}
        <div className="hidden lg:block shrink-0 w-48 space-y-3">
          {checkboxes}
        </div>

        <div className="flex-1 min-w-0 space-y-4">
          {/* Mobile metric selector */}
          <div className="lg:hidden">
            <button
              onClick={() => setMobileOpen(!mobileOpen)}
              className="w-full flex items-center justify-between bg-zinc-800 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100"
            >
              <span>
                {selectedMetrics.length} metric{selectedMetrics.length !== 1 ? "s" : ""} selected
              </span>
              <svg
                className={`w-4 h-4 transition-transform ${mobileOpen ? "rotate-180" : ""}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            {mobileOpen && (
              <div className="mt-2 bg-zinc-900 border border-zinc-800 rounded-lg p-3 space-y-3">
                {checkboxes}
              </div>
            )}
          </div>

          {/* Chart */}
          {isLoading ? (
            <div className="bg-zinc-900 rounded-lg p-6 h-[350px] animate-pulse" />
          ) : (
            <TrendsChart seriesData={seriesData} />
          )}

          {/* Summary cards */}
          <TrendSummaryCards metrics={summaryMetrics} />
        </div>
      </div>
    </div>
  );
}
