const BASE = "/api/v1";

export interface HealthMetricRow {
  Time: string;
  MetricName: string;
  Units: string;
  Qty: number | null;
  MinVal: number | null;
  AvgVal: number | null;
  MaxVal: number | null;
}

export interface TimeSeriesPoint {
  time: string;
  avg: number | null;
  min: number | null;
  max: number | null;
  count: number;
}

export async function fetchLatestMetrics(): Promise<HealthMetricRow[]> {
  const res = await fetch(`${BASE}/metrics/latest`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function fetchTimeSeries(
  metric: string,
  start: string,
  end: string,
  agg: string = "daily"
): Promise<TimeSeriesPoint[]> {
  const params = new URLSearchParams({ metric, start, end, agg });
  const res = await fetch(`${BASE}/timeseries?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}
