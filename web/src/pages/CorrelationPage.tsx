import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchCorrelation } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import ScatterChart from "../components/correlation/ScatterChart";
import OverlayChart from "../components/correlation/OverlayChart";
import SavedViews, {
  CorrelationView,
} from "../components/correlation/SavedViews";

const METRIC_GROUPS = [
  {
    label: "Cardiovascular",
    metrics: [
      { value: "heart_rate", label: "Heart Rate" },
      { value: "resting_heart_rate", label: "Resting HR" },
      { value: "heart_rate_variability", label: "HRV" },
      { value: "blood_oxygen_saturation", label: "SpO2" },
      { value: "respiratory_rate", label: "Resp. Rate" },
      { value: "vo2_max", label: "VO2 Max" },
    ],
  },
  {
    label: "Sleep",
    metrics: [{ value: "sleep_analysis", label: "Sleep Duration" }],
  },
  {
    label: "Body",
    metrics: [
      { value: "weight_body_mass", label: "Weight" },
      { value: "body_fat_percentage", label: "Body Fat" },
    ],
  },
  {
    label: "Activity",
    metrics: [
      { value: "active_energy", label: "Active Energy" },
      { value: "basal_energy_burned", label: "Basal Energy" },
      { value: "apple_exercise_time", label: "Exercise Time" },
    ],
  },
];

// Flat list for lookup
const ALL_METRICS = METRIC_GROUPS.flatMap((g) => g.metrics);

const PRESETS = [
  { label: "Sleep vs HRV", x: "sleep_analysis", y: "heart_rate_variability" },
  { label: "HRV vs RHR", x: "heart_rate_variability", y: "resting_heart_rate" },
  { label: "Sleep vs RHR", x: "sleep_analysis", y: "resting_heart_rate" },
  {
    label: "Exercise vs HRV",
    x: "apple_exercise_time",
    y: "heart_rate_variability",
  },
];

import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

type Mode = "scatter" | "overlay";

function getLabel(value: string): string {
  return ALL_METRICS.find((m) => m.value === value)?.label ?? value;
}

function MetricSelect({
  value,
  onChange,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  label: string;
}) {
  return (
    <div>
      <label className="block text-xs text-zinc-500 mb-1">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-3 py-1.5 text-sm
                   focus:outline-none focus:ring-1 focus:ring-cyan-500"
      >
        {METRIC_GROUPS.map((group) => (
          <optgroup key={group.label} label={group.label}>
            {group.metrics.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  );
}

export default function CorrelationPage() {
  const [xMetric, setXMetric] = useState("heart_rate_variability");
  const [yMetric, setYMetric] = useState("resting_heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [mode, setMode] = useState<Mode>("scatter");
  const [offset, setOffset] = useState(0);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  const { data, isLoading } = useQuery({
    queryKey: ["correlation", xMetric, yMetric, start, end],
    queryFn: () => fetchCorrelation(xMetric, yMetric, start, end),
    enabled: xMetric !== yMetric,
  });

  const handleLoadView = (view: CorrelationView) => {
    setXMetric(view.xMetric);
    setYMetric(view.yMetric);
    setTimeRange(view.timeRange as TimeRange);
    setMode(view.mode);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Correlations</h2>
        <TimeRangeSelector
          value={timeRange}
          onChange={(v) => { setTimeRange(v as TimeRange); setOffset(0); }}
          options={["1d", "7d", "30d", "90d", "1y"]}
          onPrev={() => setOffset((o) => o + 1)}
          onNext={() => setOffset((o) => Math.max(0, o - 1))}
          canGoNext={offset > 0}
          dateLabel={formatDateLabel(start, end)}
        />
      </div>

      {/* Presets */}
      <div className="flex flex-wrap gap-2">
        {PRESETS.map((p) => (
          <button
            key={p.label}
            onClick={() => {
              setXMetric(p.x);
              setYMetric(p.y);
            }}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
              xMetric === p.x && yMetric === p.y
                ? "bg-cyan-600 text-white"
                : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200"
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>

      {/* Metric selectors */}
      <div className="flex flex-wrap items-end gap-4">
        <MetricSelect value={xMetric} onChange={setXMetric} label="X Axis" />
        <span className="text-zinc-500 text-sm pb-1">vs</span>
        <MetricSelect value={yMetric} onChange={setYMetric} label="Y Axis" />

        {/* Mode toggle */}
        <div className="flex gap-1">
          <button
            onClick={() => setMode("scatter")}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
              mode === "scatter"
                ? "bg-cyan-600 text-white"
                : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
            }`}
          >
            Scatter
          </button>
          <button
            onClick={() => setMode("overlay")}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
              mode === "overlay"
                ? "bg-cyan-600 text-white"
                : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
            }`}
          >
            Overlay
          </button>
        </div>
      </div>

      {/* Saved views */}
      <SavedViews
        current={{ xMetric, yMetric, timeRange, mode }}
        onLoad={handleLoadView}
      />

      {/* Same metric warning */}
      {xMetric === yMetric && (
        <div className="text-amber-500 text-sm p-4 bg-zinc-900 rounded-lg">
          Select two different metrics to see their correlation.
        </div>
      )}

      {/* Chart */}
      {xMetric !== yMetric && (
        <>
          {isLoading ? (
            <div className="bg-zinc-900 rounded-lg p-6 h-[390px] animate-pulse" />
          ) : data?.points && data.points.length > 0 ? (
            <>
              {/* Pearson R badge */}
              {data.pearson_r != null && (
                <div className="flex items-center gap-3 text-sm">
                  <span className="text-zinc-500">Pearson r =</span>
                  <span
                    className={`font-mono font-medium ${
                      Math.abs(data.pearson_r) > 0.5
                        ? "text-cyan-400"
                        : "text-zinc-400"
                    }`}
                  >
                    {data.pearson_r.toFixed(3)}
                  </span>
                  <span className="text-zinc-600">
                    ({data.count} data points)
                  </span>
                </div>
              )}

              {/* Low data warning */}
              {data.count < 7 && (
                <div className="text-amber-500 text-sm p-3 bg-amber-500/5 border border-amber-500/20 rounded-lg">
                  Low data overlap — correlation may not be reliable.
                </div>
              )}

              {mode === "scatter" ? (
                <ScatterChart
                  points={data.points}
                  xLabel={getLabel(xMetric)}
                  yLabel={getLabel(yMetric)}
                />
              ) : (
                <OverlayChart
                  points={data.points}
                  xLabel={getLabel(xMetric)}
                  yLabel={getLabel(yMetric)}
                />
              )}
            </>
          ) : (
            <div className="text-zinc-500 text-sm p-4 bg-zinc-900 rounded-lg">
              No overlapping data found for these metrics in the selected time
              range.
            </div>
          )}
        </>
      )}
    </div>
  );
}
