import { useEffect, useRef, useState } from "react";
import {
  startHAEImport,
  cancelHAEImport,
  fetchHAEImportStatus,
  type HAEImportStatus,
} from "../../api";

const BASE = "/api/v1";

export default function HAEImportTab() {
  const [host, setHost] = useState("");
  const [port, setPort] = useState("9000");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState(
    new Date().toISOString().split("T")[0]
  );
  const [dryRun, setDryRun] = useState(false);

  const [status, setStatus] = useState<HAEImportStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Poll for status on mount to catch already-running imports
  useEffect(() => {
    fetchHAEImportStatus()
      .then((s) => {
        setStatus(s);
        if (s.running) connectSSE();
      })
      .catch(() => {});
    return () => eventSourceRef.current?.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function connectSSE() {
    eventSourceRef.current?.close();
    const es = new EventSource(`${BASE}/import/hae-tcp/events`);
    eventSourceRef.current = es;

    es.addEventListener("progress", (e) => {
      const data = JSON.parse(e.data);
      setStatus((prev) => ({
        ...prev,
        running: true,
        step: data.step,
        total: data.total,
        metric: data.metric,
        chunk: data.chunk,
      }));
    });

    es.addEventListener("status", (e) => {
      const data = JSON.parse(e.data);
      setStatus((prev) => ({ ...prev, running: true, ...data }));
    });

    es.addEventListener("complete", (e) => {
      const data = JSON.parse(e.data);
      setStatus((prev) => ({
        ...prev,
        running: false,
        done: true,
        metrics_received: data.metrics_received,
        metrics_inserted: data.metrics_inserted,
        workouts_received: data.workouts_received,
        workouts_inserted: data.workouts_inserted,
        sleep_sessions: data.sleep_sessions,
        bytes_fetched: data.bytes_fetched,
      }));
      es.close();
    });

    es.addEventListener("error", () => {
      // SSE connection error or server closed it
      es.close();
      // Refresh status via polling
      fetchHAEImportStatus()
        .then(setStatus)
        .catch(() => {});
    });
  }

  async function handleStart() {
    setError(null);
    setStarting(true);
    try {
      const resp = await startHAEImport({
        hae_host: host,
        hae_port: parseInt(port, 10),
        start: startDate,
        end: endDate,
        dry_run: dryRun,
      });
      setStatus({
        running: true,
        step: 0,
        total: resp.total_steps,
        log_id: resp.log_id,
      });
      connectSSE();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg === "Failed to fetch" || msg.includes("NetworkError")) {
        setError("Could not start import. Is the HAE app open on your iPhone?");
      } else {
        setError(msg);
      }
    } finally {
      setStarting(false);
    }
  }

  async function handleCancel() {
    try {
      await cancelHAEImport();
      setStatus((prev) => (prev ? { ...prev, running: false } : prev));
      eventSourceRef.current?.close();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  const isRunning = status?.running ?? false;
  const progress =
    status?.step && status?.total
      ? Math.round((status.step / status.total) * 100)
      : 0;

  return (
    <div className="space-y-6">
      <p className="text-sm text-zinc-400">
        Import health data from the Health Auto Export TCP server running on your
        iPhone. The app must be open and on the same network.
      </p>

      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            HAE Host
          </label>
          <input
            type="text"
            value={host}
            onChange={(e) => setHost(e.target.value)}
            placeholder="linus-iphone"
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
          <p className="text-xs text-zinc-600 mt-1">Tailscale hostname or local IP</p>
        </div>
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            Port
          </label>
          <input
            type="number"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
        </div>
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            Start Date
          </label>
          <input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
        </div>
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            End Date
          </label>
          <input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
        </div>
        <div className="flex items-end">
          <label className="flex items-center gap-2 text-sm text-zinc-300 cursor-pointer">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
              disabled={isRunning}
              className="accent-cyan-600"
            />
            Dry Run
          </label>
        </div>
      </div>

      <div className="flex gap-3">
        {!isRunning ? (
          <button
            onClick={handleStart}
            disabled={starting || !host || !startDate || !endDate}
            className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 disabled:bg-zinc-700 disabled:text-zinc-500 text-white text-sm font-medium rounded-md transition-colors"
          >
            {starting ? "Starting..." : "Start Import"}
          </button>
        ) : (
          <button
            onClick={handleCancel}
            className="px-4 py-2 bg-red-600 hover:bg-red-500 text-white text-sm font-medium rounded-md transition-colors"
          >
            Cancel Import
          </button>
        )}
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {status && (isRunning || status.done) && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 space-y-3">
          <div className="flex items-center justify-between text-sm">
            <span className="text-zinc-300">
              {isRunning ? (
                <>
                  Importing:{" "}
                  <span className="text-cyan-400">{status.metric}</span>
                  {status.chunk && (
                    <span className="text-zinc-500 ml-2">{status.chunk}</span>
                  )}
                </>
              ) : (
                <span className="text-green-400">Import complete</span>
              )}
            </span>
            <span className="text-zinc-500 font-mono text-xs">
              {status.step}/{status.total}
            </span>
          </div>

          <div className="w-full bg-zinc-800 rounded-full h-2">
            <div
              className="bg-cyan-500 h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
          </div>

          {status.done && (
            <div className="flex gap-4 text-xs text-zinc-400 flex-wrap">
              {(status.metrics_received ?? 0) > 0 && (
                <span>
                  Metrics: {status.metrics_inserted} imported / {status.metrics_received} received
                </span>
              )}
              {(status.workouts_received ?? 0) > 0 && (
                <span>
                  Workouts: {status.workouts_inserted} imported / {status.workouts_received} received
                </span>
              )}
              {(status.sleep_sessions ?? 0) > 0 && (
                <span>Sleep: {status.sleep_sessions} nights</span>
              )}
              {(status.bytes_fetched ?? 0) > 0 && (
                <span>
                  Data: {((status.bytes_fetched ?? 0) / 1024 / 1024).toFixed(1)} MB
                </span>
              )}
            </div>
          )}

          {status.error && (
            <p className="text-xs text-red-400">{status.error}</p>
          )}
        </div>
      )}
    </div>
  );
}
