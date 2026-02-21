import { useEffect, useState } from "react";
import { fetchImportLogs, type ImportLog } from "../../api";

const SOURCE_LABELS: Record<string, string> = {
  hae_rest: "HAE REST",
  hae_tcp: "HAE TCP",
  alpha: "Alpha",
};

const STATUS_STYLES: Record<string, string> = {
  success: "bg-green-900/50 text-green-400 border-green-800",
  error: "bg-red-900/50 text-red-400 border-red-800",
  running: "bg-cyan-900/50 text-cyan-400 border-cyan-800",
  cancelled: "bg-yellow-900/50 text-yellow-400 border-yellow-800",
};

function formatDuration(ms: number | null): string {
  if (ms === null) return "—";
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export default function ImportLogsTab() {
  const [logs, setLogs] = useState<ImportLog[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchImportLogs(100).then(setLogs).catch((e) => setError(e.message));
  }, []);

  if (error) {
    return <p className="text-red-400">Failed to load import logs: {error}</p>;
  }
  if (!logs) {
    return <p className="text-zinc-500">Loading...</p>;
  }
  if (logs.length === 0) {
    return <p className="text-zinc-500">No imports recorded yet.</p>;
  }

  return (
    <div className="space-y-3">
      {logs.map((log) => (
        <div
          key={log.id}
          className="bg-zinc-900 border border-zinc-800 rounded-lg p-4"
        >
          <div className="flex items-center gap-3 mb-2 flex-wrap">
            <span className="px-2 py-0.5 rounded text-xs font-medium bg-zinc-800 text-zinc-300 border border-zinc-700">
              {SOURCE_LABELS[log.source] || log.source}
            </span>
            <span
              className={`px-2 py-0.5 rounded text-xs font-medium border ${STATUS_STYLES[log.status] || "bg-zinc-800 text-zinc-400 border-zinc-700"}`}
            >
              {log.status}
            </span>
            <span className="text-xs text-zinc-500">
              {new Date(log.created_at).toLocaleString()}
            </span>
            <span className="text-xs text-zinc-600">
              {formatDuration(log.duration_ms)}
            </span>
          </div>

          <div className="flex gap-4 text-xs text-zinc-400 flex-wrap">
            {log.metrics_received > 0 && (
              <span>
                Metrics: {log.metrics_inserted} imported / {log.metrics_received} received
              </span>
            )}
            {log.workouts_received > 0 && (
              <span>
                Workouts: {log.workouts_inserted} imported / {log.workouts_received} received
              </span>
            )}
            {log.sleep_sessions > 0 && (
              <span>Sleep: {log.sleep_sessions}</span>
            )}
            {log.sets_inserted > 0 && (
              <span>Sets: {log.sets_inserted}</span>
            )}
          </div>

          {log.error_message && (
            <p className="mt-2 text-xs text-red-400 font-mono break-all">
              {log.error_message}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}
