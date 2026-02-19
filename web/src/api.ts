const BASE = "/api/v1";

// --- Health Metrics ---

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

export interface MetricStats {
  metric: string;
  avg: number | null;
  min: number | null;
  max: number | null;
  stddev: number | null;
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

export async function fetchMetricStats(
  metric: string,
  start: string,
  end: string
): Promise<MetricStats> {
  const params = new URLSearchParams({ metric, start, end });
  const res = await fetch(`${BASE}/metrics/stats?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- Sleep ---

export interface SleepSession {
  ID: number;
  UserID: number;
  Date: string;
  TotalSleep: number;
  Asleep: number;
  Core: number;
  Deep: number;
  REM: number;
  InBed: number;
  SleepStart: string;
  SleepEnd: string;
  InBedStart: string;
  InBedEnd: string;
}

export interface SleepStage {
  StartTime: string;
  EndTime: string;
  Stage: string;
  DurationHr: number;
  Source: string;
}

export interface SleepResponse {
  sessions: SleepSession[];
  stages: SleepStage[];
}

export async function fetchSleep(
  start: string,
  end: string
): Promise<SleepResponse> {
  const params = new URLSearchParams({ start, end });
  const res = await fetch(`${BASE}/sleep?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- Workouts ---

export interface Workout {
  ID: string;
  UserID: number;
  Name: string;
  StartTime: string;
  EndTime: string;
  DurationSec: number;
  Location: string;
  IsIndoor: boolean | null;
  ActiveEnergyBurned: number | null;
  ActiveEnergyUnits: string;
  TotalEnergy: number | null;
  TotalEnergyUnits: string;
  Distance: number | null;
  DistanceUnits: string;
  AvgHeartRate: number | null;
  MaxHeartRate: number | null;
  MinHeartRate: number | null;
  ElevationUp: number | null;
  ElevationDown: number | null;
}

export interface WorkoutHR {
  Time: string;
  MinBPM: number | null;
  AvgBPM: number | null;
  MaxBPM: number | null;
}

export interface WorkoutRoute {
  Time: string;
  Latitude: number;
  Longitude: number;
  Altitude: number | null;
  Speed: number | null;
}

export interface WorkoutDetail extends Workout {
  HeartRateData: WorkoutHR[] | null;
  RouteData: WorkoutRoute[] | null;
}

export async function fetchWorkouts(
  start: string,
  end: string,
  type?: string
): Promise<Workout[]> {
  const params = new URLSearchParams({ start, end });
  if (type) params.set("type", type);
  const res = await fetch(`${BASE}/workouts?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function fetchWorkoutDetail(id: string): Promise<WorkoutDetail> {
  const res = await fetch(`${BASE}/workouts/${id}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- Correlation ---

export interface CorrelationPoint {
  time: string;
  x: number | null;
  y: number | null;
}

export interface CorrelationResponse {
  points: CorrelationPoint[];
  pearson_r: number | null;
  count: number;
}

export async function fetchCorrelation(
  xMetric: string,
  yMetric: string,
  start: string,
  end: string,
  bucket: string = "1 day"
): Promise<CorrelationResponse> {
  const params = new URLSearchParams({
    x: xMetric,
    y: yMetric,
    start,
    end,
    bucket,
  });
  const res = await fetch(`${BASE}/correlation?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}
