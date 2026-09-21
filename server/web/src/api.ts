const BASE = "/api/v1";

// --- Version ---

export interface VersionInfo {
  version: string;
}

export async function fetchVersion(): Promise<VersionInfo> {
  const res = await fetch(`${BASE}/version`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

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

// --- Front page ---

/**
 * One metric as the front page needs it: metadata, latest reading, and
 * everything derived from the 30-day window. The series array is what lets the
 * dashboard render sparklines without a second request or a chart library.
 */
export interface FrontPageMetric {
  metric_name: string;
  label: string;
  category: string;
  unit: string;
  is_cumulative: boolean;
  multiplier: number;
  source: string;
  time: string;
  latest: number | null;
  /** Mean of the last 7 days minus the mean of the 7 before it. */
  delta_7d: number | null;
  /** The same change relative to the earlier window, as a fraction. */
  delta_7d_pct: number | null;
  /** p05 and p95 over the window — percentiles, so one bad reading does not widen it. */
  range_low: number | null;
  range_high: number | null;
  /** One slot per day, oldest first, null where the day has no samples. */
  series: (number | null)[];
}

export interface FrontPageResponse {
  metrics: FrontPageMetric[];
  heroes: string[];
  total_available: number;
  window_days: number;
  window_start: string;
  last_sync: string | null;
  last_sources: string[] | null;
}

export async function fetchFrontPage(
  range: string = "30d",
): Promise<FrontPageResponse> {
  const res = await fetch(`${BASE}/metrics/latest?range=${range}`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export interface MaxHeartRate {
  bpm: number;
  /** Where the figure came from; the screen states which. */
  origin: "configured" | "estimated" | "observed";
  /** The measured figure, kept whichever origin wins. */
  observed: number;
  /** The age-based estimate, 0 when no birth date is stored. */
  estimated: number;
  /** Completed years, 0 when no birth date is stored. */
  age: number;
}

export interface BirthDate {
  birth_date: string;
  age: number;
}

export async function fetchBirthDate(): Promise<BirthDate> {
  const res = await fetch(`${BASE}/preferences/birth-date`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

/** Send an empty string to clear it. */
export async function saveBirthDate(birthDate: string): Promise<void> {
  const res = await fetch(`${BASE}/preferences/birth-date`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ birth_date: birthDate }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function fetchMaxHeartRate(): Promise<MaxHeartRate> {
  const res = await fetch(`${BASE}/preferences/max-heart-rate`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

/** Send 0 to clear the configured value and fall back to the measured one. */
export async function saveMaxHeartRate(bpm: number): Promise<void> {
  const res = await fetch(`${BASE}/preferences/max-heart-rate`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ bpm }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function saveFrontPageHeroes(heroes: string[]): Promise<void> {
  const res = await fetch(`${BASE}/preferences/front-page-heroes`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(heroes),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
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
  Source?: string;
  /** Every source that reported this workout; more than one when a provider's
   *  session also reaches HealthKit. See recordedBy(). */
  Sources?: string[];
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
  alpha_session_name?: string;
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

/** One session's share of time per heart rate zone, five fractions summing to 1. */
export interface WorkoutZones {
  workout_id: string;
  shares: number[];
}

export interface WorkoutZonesResponse {
  /** The highest heart rate ever recorded; the zone bands derive from it. */
  max_heart_rate: number;
  zones: WorkoutZones[] | null;
}

export async function fetchWorkoutZones(
  start: string,
  end: string,
): Promise<WorkoutZonesResponse> {
  const params = new URLSearchParams({ start, end });
  const res = await fetch(`${BASE}/workouts/zones?${params}`);
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
  /** Resolved from the exercise catalog; empty when the exercise has no entry. */
  PrimaryMuscleGroup: string;
  Equipment: string;
  TargetReps: number;
  IsWarmup: boolean;
  SetNumber: number;
  WeightKg: number;
  IsBodyweightPlus: boolean;
  Reps: number;
  /** Reps in reserve. Alpha Progression records this; -1 means unrated. */
  RIR: number | null;
  /** Rating of perceived exertion. Hevy records this instead of RIR. */
  RPE: number | null;
  /** Derived by the database from whichever of the two the source supplied. */
  EffortRIR: number | null;
}

export async function fetchWorkoutSets(
  id: string,
  start?: string,
  end?: string
): Promise<WorkoutSet[]> {
  const params = new URLSearchParams();
  if (start) params.set("start", start);
  if (end) params.set("end", end);
  const qs = params.toString();
  const res = await fetch(`${BASE}/workouts/${id}/sets${qs ? "?" + qs : ""}`);
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
  sets_received: number;
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

// --- Metric Metadata ---

export interface MetricMeta {
  metric_name: string;
  category: string;
  display_label: string;
  display_unit: string;
  is_cumulative: boolean;
  display_multiplier: number;
  visible: boolean;
}

export async function fetchAvailableMetrics(): Promise<MetricMeta[]> {
  const res = await fetch(`${BASE}/metrics/available`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveMetricVisibility(visibility: Record<string, boolean>): Promise<void> {
  const res = await fetch(`${BASE}/metrics/visibility`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(visibility),
  });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

// --- Source Priority ---

export interface SourcePriorityRule {
  user_id: number;
  category: string;
  sources: string[];
}

/** When a source last wrote a metric, and how many it has written. */
export interface SourceActivity {
  source: string;
  last_seen: string;
  rows: number;
}

export interface SourcePriorityConfig {
  rules: SourcePriorityRule[];
  sources: string[];
  categories: string[];
  activity: SourceActivity[] | null;
  default: string[];
}

export async function fetchSourcePriority(): Promise<SourcePriorityConfig> {
  const res = await fetch(`${BASE}/source-priority`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveSourcePriority(category: string, sources: string[]): Promise<void> {
  const res = await fetch(`${BASE}/source-priority`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ category, sources }),
  });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

export async function deleteSourcePriority(category: string): Promise<void> {
  const res = await fetch(`${BASE}/source-priority/${encodeURIComponent(category)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

// --- Oura Integration ---

export interface OuraStatus {
  configured: boolean;
  connected: boolean;
  /** The redirect URI this instance will use; register it with the provider. */
  redirect_uri?: string;
  client_id?: string;
  expires_at?: string;
  sync_states?: Record<string, string>;
}

export async function fetchOuraStatus(): Promise<OuraStatus> {
  const res = await fetch(`${BASE}/oura/status`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveOuraCredentials(clientId: string, clientSecret: string): Promise<void> {
  const res = await fetch(`${BASE}/oura/credentials`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ client_id: clientId, client_secret: clientSecret }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function authorizeOura(): Promise<{ authorize_url: string }> {
  const res = await fetch(`${BASE}/oura/authorize`, { method: "POST" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function triggerOuraSync(): Promise<void> {
  const res = await fetch(`${BASE}/oura/sync`, { method: "POST" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

export async function disconnectOura(): Promise<void> {
  const res = await fetch(`${BASE}/oura/disconnect`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

// --- Withings Integration ---

export interface WithingsStatus {
  configured: boolean;
  connected: boolean;
  /** The redirect URI this instance will use; register it with the provider. */
  redirect_uri?: string;
  client_id?: string;
  expires_at?: string;
  /** Job name -> RFC3339 timestamp the next delta fetch resumes from. */
  sync_states?: Record<string, string>;
}

export async function fetchWithingsStatus(): Promise<WithingsStatus> {
  const res = await fetch(`${BASE}/withings/status`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveWithingsCredentials(
  clientId: string,
  clientSecret: string,
): Promise<void> {
  const res = await fetch(`${BASE}/withings/credentials`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ client_id: clientId, client_secret: clientSecret }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function authorizeWithings(): Promise<{ authorize_url: string }> {
  const res = await fetch(`${BASE}/withings/authorize`, { method: "POST" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function triggerWithingsSync(): Promise<void> {
  const res = await fetch(`${BASE}/withings/sync`, { method: "POST" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

export async function disconnectWithings(): Promise<void> {
  const res = await fetch(`${BASE}/withings/disconnect`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

// --- Hevy Integration ---

export interface HevyStatus {
  configured: boolean;
  /** YYYY-MM-DD. Workouts that started before this date are not ingested. */
  sync_from?: string;
  /** RFC3339 timestamp the next event fetch resumes from. */
  last_sync?: string;
}

export async function fetchHevyStatus(): Promise<HevyStatus> {
  const res = await fetch(`${BASE}/hevy/status`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveHevyCredentials(apiKey: string, syncFrom: string): Promise<void> {
  const res = await fetch(`${BASE}/hevy/credentials`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ api_key: apiKey, sync_from: syncFrom }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

export async function triggerHevySync(): Promise<void> {
  const res = await fetch(`${BASE}/hevy/sync`, { method: "POST" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

export async function disconnectHevy(): Promise<void> {
  const res = await fetch(`${BASE}/hevy/disconnect`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
}

/* ── Alerts ──────────────────────────────────────────────────────────────── */

/** The channel configuration as stored in the database, not in config.yaml. */
export interface AlertSettings {
  enabled: boolean;
  ntfy_url: string;
  hostname: string;
  check_interval_sec: number;
  failure_threshold: number;
  /** 0 turns the Apple Health silence rule off. */
  apple_silence_sec: number;
}

/** One alert condition and what the last check made of it. */
export interface AlertCondition {
  monitor_id: number;
  service: string;
  firing: boolean;
  since?: string;
  last_msg?: string;
  checked_at?: string;
}

export interface AlertsResponse {
  /** False while no settings row exists — the watcher then reports nothing. */
  configured: boolean;
  settings: AlertSettings;
  updated_at?: string;
  conditions: AlertCondition[];
}

export async function fetchAlerts(): Promise<AlertsResponse> {
  const res = await fetch(`${BASE}/alerts`);
  if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
  return res.json();
}

export async function saveAlertSettings(settings: AlertSettings): Promise<void> {
  const res = await fetch(`${BASE}/alerts`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `${res.status}: ${res.statusText}`);
  }
}

/** Posts one message on the channel-test id, using the stored settings. */
export async function sendTestAlert(): Promise<{ monitor_id: number; target: string }> {
  const res = await fetch(`${BASE}/alerts/test`, { method: "POST" });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `${res.status}: ${res.statusText}`);
  return body;
}
