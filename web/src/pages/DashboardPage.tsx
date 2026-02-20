import DailyOverview from "../components/DailyOverview";
import TimeSeriesChart from "../components/TimeSeriesChart";
import MetricSelector from "../components/MetricSelector";
import TimeRangeSelector from "../components/TimeRangeSelector";
import { useState } from "react";

const METRIC_OPTIONS = [
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

export default function DashboardPage() {
  const [metric, setMetric] = useState("heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("30d");

  const end = new Date().toISOString().split("T")[0];
  const start = new Date(Date.now() - daysFromRange(timeRange) * 86400000)
    .toISOString()
    .split("T")[0];

  const selected = METRIC_OPTIONS.find((m) => m.value === metric);

  return (
    <>
      <DailyOverview />

      <div className="mt-8">
        <div className="flex flex-wrap items-center gap-4 mb-4">
          <MetricSelector
            options={METRIC_OPTIONS}
            value={metric}
            onChange={setMetric}
          />
          <TimeRangeSelector
            value={timeRange}
            onChange={(v) => setTimeRange(v as TimeRange)}
            options={["1d", "7d", "30d", "90d", "1y"]}
          />
        </div>

        <TimeSeriesChart
          metric={metric}
          start={start}
          end={end}
          label={selected?.label ?? metric}
          unit={selected?.unit ?? ""}
          agg={timeRange === "1d" ? "hourly" : "daily"}
        />
      </div>
    </>
  );
}
