const BASE = "/api/v1";

// --- User Identity ---

export interface UserInfo {
  login: string;
  display_name: string;
  tailscale_id?: string;
  tailnet?: string;
}

export async function fetchMe(): Promise<UserInfo> {
  const res = await fetch(`${BASE}/me`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

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

export interface DailySum {
  MetricName: string;
  Units: string;
  Total: number;
}

export interface LatestMetricsResponse {
  latest: HealthMetricRow[];
  daily_sums: DailySum[] | null;
}

export async function fetchLatestMetrics(): Promise<LatestMetricsResponse> {
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

// --- Workout Sets ---

export interface WorkoutSet {
  SessionName: string;
  SessionDate: string;
  SessionDuration: string;
  ExerciseNumber: number;
  ExerciseName: string;
  Equipment: string;
  TargetReps: number;
  IsWarmup: boolean;
  SetNumber: number;
  WeightKg: number;
  IsBodyweightPlus: boolean;
  Reps: number;
  RIR: number;
}

export async function fetchWorkoutSets(id: string): Promise<WorkoutSet[]> {
  const res = await fetch(`${BASE}/workouts/${id}/sets`);
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

// --- Stats ---

export interface WorkoutTypeStat {
  name: string;
  count: number;
  total_duration_sec: number;
  total_distance?: number | null;
}

export interface DataStats {
  total_metric_rows: number;
  total_workouts: number;
  total_sleep_nights: number;
  total_sets: number;
  earliest_data: string | null;
  latest_data: string | null;
  workouts_by_type: WorkoutTypeStat[] | null;
}

export async function fetchStats(): Promise<DataStats> {
  const res = await fetch(`${BASE}/stats`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- Import Logs ---

export interface ImportLog {
  id: number;
  user_id: number;
  created_at: string;
  source: string;
  status: string;
  metrics_received: number;
  metrics_inserted: number;
  workouts_received: number;
  workouts_inserted: number;
  sleep_sessions: number;
  sets_inserted: number;
  duration_ms: number | null;
  error_message: string | null;
  metadata: Record<string, unknown> | null;
}

export async function fetchImportLogs(
  limit: number = 50
): Promise<ImportLog[]> {
  const params = new URLSearchParams({ limit: String(limit) });
  const res = await fetch(`${BASE}/import-logs?${params}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- HAE TCP Import ---

export interface HAEImportStatus {
  running: boolean;
  done?: boolean;
  step?: number;
  total?: number;
  metric?: string;
  chunk?: string;
  metrics_received?: number;
  metrics_inserted?: number;
  workouts_received?: number;
  workouts_inserted?: number;
  sleep_sessions?: number;
  bytes_fetched?: number;
  error?: string;
  log_id?: number;
}

export async function startHAEImport(params: {
  hae_host: string;
  hae_port: number;
  start: string;
  end: string;
  dry_run: boolean;
}): Promise<{ status: string; total_steps: number; log_id: number }> {
  const res = await fetch(`${BASE}/import/hae-tcp`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
  return res.json();
}

export async function cancelHAEImport(): Promise<void> {
  const res = await fetch(`${BASE}/import/hae-tcp`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function fetchHAEImportStatus(): Promise<HAEImportStatus> {
  const res = await fetch(`${BASE}/import/hae-tcp/status`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

// --- Alpha CSV Upload ---

export async function uploadAlphaCSV(
  file: File
): Promise<{ sets_received: number; sets_inserted: number }> {
  const res = await fetch(`${BASE}/ingest/alpha`, {
    method: "POST",
    headers: { "Content-Type": "text/csv" },
    body: file,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
  return res.json();
}
