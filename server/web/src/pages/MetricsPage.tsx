import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { fetchTimeSeries, fetchMetricStats } from "../api";
import TimeRangeSelector from "../components/TimeRangeSelector";
import MetricStatsBar from "../components/metrics/MetricStatsBar";
import MetricTimeSeriesChart from "../components/metrics/MetricTimeSeriesChart";
import { useAvailableMetrics } from "../hooks/useMetrics";
import { daysFromRange, formatDateLabel, type TimeRange } from "../utils/timeRange";

export default function MetricsPage() {
  const { visibleOptions: options, lookup, isLoading: metricsLoading } = useAvailableMetrics();
  const [metric, setMetric] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRange>("90d");
  const [offset, setOffset] = useState(0);

  // Auto-select first metric when options load
  useEffect(() => {
    if (options.length > 0 && !metric) {
      const rhr = options.find((m) => m.value === "resting_heart_rate");
      setMetric(rhr?.value ?? options[0].value);
    }
  }, [options, metric]);

  const days = daysFromRange(timeRange);
  const endDate = new Date(Date.now() - offset * days * 86400000);
  const startDate = new Date(endDate.getTime() - days * 86400000);
  const end = endDate.toISOString().split("T")[0];
  const start = startDate.toISOString().split("T")[0];

  const selected = lookup.get(metric);
  const multiplier = selected?.multiplier ?? 1;

  const agg = timeRange === "1d" ? "hourly" : "daily";
  const { data: tsData, isLoading: tsLoading } = useQuery({
    queryKey: ["timeseries", metric, start, end, agg],
    queryFn: () => fetchTimeSeries(metric, start, end, agg),
    enabled: !!metric,
  });

  const { data: statsData } = useQuery({
    queryKey: ["metricStats", metric, start, end],
    queryFn: () => fetchMetricStats(metric, start, end),
    enabled: !!metric,
  });

  // Apply display multiplier to stats and time series
  const scaledStats = statsData && multiplier !== 1
    ? { ...statsData, avg: (statsData.avg ?? 0) * multiplier, min: (statsData.min ?? 0) * multiplier, max: (statsData.max ?? 0) * multiplier, stddev: (statsData.stddev ?? 0) * multiplier }
    : statsData;

  const scaledTs = tsData && multiplier !== 1
    ? tsData.map((p: any) => ({ ...p, avg: p.avg != null ? p.avg * multiplier : null, min: p.min != null ? p.min * multiplier : null, max: p.max != null ? p.max * multiplier : null }))
    : tsData;

  if (metricsLoading) {
    return <p className="text-zinc-500">Loading metrics...</p>;
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 className="text-xl font-semibold text-zinc-100">Metrics</h2>
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

      <div className="flex gap-6">
        <div className="hidden lg:block shrink-0 w-48 space-y-1">
          {options.map((m) => (
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
          <div className="lg:hidden">
            <select
              value={metric}
              onChange={(e) => setMetric(e.target.value)}
              className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-3 py-1.5 text-sm w-full
                         focus:outline-none focus:ring-1 focus:ring-cyan-500"
            >
              {options.map((m) => (
                <option key={m.value} value={m.value}>
                  {m.label}
                </option>
              ))}
            </select>
          </div>

          {scaledStats && <MetricStatsBar stats={scaledStats} />}

          {tsLoading ? (
            <div className="bg-zinc-900 rounded-lg p-6 h-[340px] animate-pulse" />
          ) : (
            <MetricTimeSeriesChart
              data={scaledTs ?? []}
              stats={scaledStats ?? null}
              label={selected?.label ?? metric}
              unit={selected?.unit ?? ""}
            />
          )}
        </div>
      </div>
    </div>
  );
}
