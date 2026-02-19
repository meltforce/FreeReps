import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { fetchCorrelation } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import ScatterChart from "../components/correlation/ScatterChart";
import OverlayChart from "../components/correlation/OverlayChart";
import SavedViews, {
  CorrelationView,
} from "../components/correlation/SavedViews";

const METRICS = [
  { value: "heart_rate", label: "Heart Rate" },
  { value: "resting_heart_rate", label: "Resting HR" },
  { value: "heart_rate_variability", label: "HRV" },
  { value: "blood_oxygen_saturation", label: "SpO2" },
  { value: "respiratory_rate", label: "Resp. Rate" },
  { value: "vo2_max", label: "VO2 Max" },
  { value: "weight_body_mass", label: "Weight" },
  { value: "body_fat_percentage", label: "Body Fat" },
  { value: "active_energy", label: "Active Energy" },
  { value: "basal_energy_burned", label: "Basal Energy" },
  { value: "apple_exercise_time", label: "Exercise Time" },
];

type TimeRange = "30d" | "90d" | "1y";
type Mode = "scatter" | "overlay";

function daysFromRange(range_: TimeRange): number {
  switch (range_) {
    case "30d":
      return 30;
    case "90d":
      return 90;
    case "1y":
      return 365;
  }
}

function getLabel(value: string): string {
  return METRICS.find((m) => m.value === value)?.label ?? value;
}

export default function CorrelationPage() {
  const [xMetric, setXMetric] = useState("heart_rate_variability");
  const [yMetric, setYMetric] = useState("resting_heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [mode, setMode] = useState<Mode>("scatter");

  const end = new Date().toISOString().split("T")[0];
  const start = new Date(Date.now() - daysFromRange(timeRange) * 86400000)
    .toISOString()
    .split("T")[0];

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
          onChange={(v) => setTimeRange(v as TimeRange)}
          options={["30d", "90d", "1y"]}
        />
      </div>

      {/* Metric selectors */}
      <div className="flex flex-wrap items-end gap-4">
        <div>
          <label className="block text-xs text-zinc-500 mb-1">X Axis</label>
          <select
            value={xMetric}
            onChange={(e) => setXMetric(e.target.value)}
            className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-3 py-1.5 text-sm
                       focus:outline-none focus:ring-1 focus:ring-cyan-500"
          >
            {METRICS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
        </div>

        <span className="text-zinc-500 text-sm pb-1">vs</span>

        <div>
          <label className="block text-xs text-zinc-500 mb-1">Y Axis</label>
          <select
            value={yMetric}
            onChange={(e) => setYMetric(e.target.value)}
            className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-3 py-1.5 text-sm
                       focus:outline-none focus:ring-1 focus:ring-cyan-500"
          >
            {METRICS.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </select>
        </div>

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
