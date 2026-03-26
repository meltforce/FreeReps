import DailyOverview from "../components/DailyOverview";
import TimeSeriesChart from "../components/TimeSeriesChart";
import MetricSelector from "../components/MetricSelector";
import TimeRangeSelector from "../components/TimeRangeSelector";
import { useEffect, useState } from "react";
import { useAvailableMetrics } from "../hooks/useMetrics";
import { useDashboardInit } from "../hooks/useDashboardInit";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

export default function DashboardPage() {
  // Single request seeds both available-metrics and latestMetrics caches.
  useDashboardInit();

  const { visibleOptions: options, lookup, isLoading } = useAvailableMetrics();
  const [metric, setMetric] = useState("heart_rate");
  const [timeRange, setTimeRange] = useState<TimeRange>("30d");
  const [offset, setOffset] = useState(0);

  // Auto-select first metric when options load
  useEffect(() => {
    if (options.length > 0 && !options.find((m) => m.value === metric)) {
      setMetric(options[0].value);
    }
  }, [options, metric]);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  const selected = lookup.get(metric);

  // Only cumulative metrics (steps, calories) and heart rate have meaningful
  // sub-daily data. Everything else (sleep, weight, scores) is once-per-day
  // and should always use daily aggregation to avoid nonsensical hourly x-axis.
  const supportsHourly = selected?.isCumulative || metric === "heart_rate";
  const agg = timeRange === "1d" && supportsHourly ? "hourly" : "daily";

  return (
    <>
      <DailyOverview />

      <div className="mt-8">
        <div className="flex flex-wrap items-center gap-4 mb-4">
          {!isLoading && (
            <MetricSelector
              options={options}
              value={metric}
              onChange={setMetric}
            />
          )}
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
          agg={agg}
          multiplier={selected?.multiplier ?? 1}
        />
      </div>
    </>
  );
}
