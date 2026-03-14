import DailyOverview from "../components/DailyOverview";
import TimeSeriesChart from "../components/TimeSeriesChart";
import MetricSelector from "../components/MetricSelector";
import TimeRangeSelector from "../components/TimeRangeSelector";
import { useState } from "react";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

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

export default function DashboardPage() {
  const [metric, setMetric] = useState("heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("30d");
  const [offset, setOffset] = useState(0);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

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
            onChange={(v) => { setTimeRange(v as TimeRange); setOffset(0); }}
            options={["1d", "7d", "30d", "90d", "1y"]}
            onPrev={() => setOffset((o) => o + 1)}
            onNext={() => setOffset((o) => Math.max(0, o - 1))}
            canGoNext={offset > 0}
            dateLabel={formatDateLabel(start, end)}
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
