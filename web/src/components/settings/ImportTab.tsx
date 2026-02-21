import { useCallback, useEffect, useRef, useState } from "react";
import { DayPicker } from "react-day-picker";
import { format, parse } from "date-fns";
import {
  startHAEImport,
  cancelHAEImport,
  fetchHAEImportStatus,
  checkHAEConnection,
  uploadAlphaCSV,
  type HAEImportStatus,
} from "../../api";

const BASE = "/api/v1";

// ---------------------------------------------------------------------------
// DatePickerInput — dark-themed calendar dropdown
// ---------------------------------------------------------------------------

function DatePickerInput({
  value,
  onChange,
  disabled,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const selected = value ? parse(value, "yyyy-MM-dd", new Date()) : undefined;

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center gap-2 bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-left text-zinc-100 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
      >
        <svg className="w-4 h-4 text-zinc-500 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 012.25-2.25h13.5A2.25 2.25 0 0121 7.5v11.25m-18 0A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75m-18 0v-7.5A2.25 2.25 0 015.25 9h13.5A2.25 2.25 0 0121 11.25v7.5" />
        </svg>
        <span className={value ? "text-zinc-100" : "text-zinc-600"}>
          {value || placeholder || "Select date"}
        </span>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 rdp-wrapper">
          <DayPicker
            mode="single"
            selected={selected}
            onSelect={(day) => {
              if (day) {
                onChange(format(day, "yyyy-MM-dd"));
                setOpen(false);
              }
            }}
            defaultMonth={selected}
          />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toggle Switch
// ---------------------------------------------------------------------------

function Toggle({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-5 w-9 shrink-0 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-cyan-600 focus:ring-offset-2 focus:ring-offset-zinc-950 disabled:opacity-50 ${
        checked ? "bg-cyan-600" : "bg-zinc-700"
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow transform transition-transform mt-0.5 ${
          checked ? "translate-x-4 ml-0.5" : "translate-x-0 ml-0.5"
        }`}
      />
    </button>
  );
}

// ---------------------------------------------------------------------------
// HAE Import Section
// ---------------------------------------------------------------------------

function HAEImportSection() {
  const [host, setHost] = useState(() => localStorage.getItem("hae_host") || "");
  const [port, setPort] = useState(() => localStorage.getItem("hae_port") || "9000");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState(
    new Date().toISOString().split("T")[0]
  );
  const [dryRun, setDryRun] = useState(false);

  const [status, setStatus] = useState<HAEImportStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const [checking, setChecking] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Persist host/port to localStorage
  const saveHost = useCallback((v: string) => {
    setHost(v);
    localStorage.setItem("hae_host", v);
  }, []);

  const savePort = useCallback((v: string) => {
    setPort(v);
    localStorage.setItem("hae_port", v);
  }, []);

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
      es.close();
      fetchHAEImportStatus()
        .then(setStatus)
        .catch(() => {});
    });
  }

  async function handleStart() {
    setError(null);
    setChecking(true);

    try {
      // Connection check first
      const check = await checkHAEConnection({
        hae_host: host,
        hae_port: parseInt(port, 10),
      });

      if (!check.reachable) {
        setError(
          `Could not reach HAE server at ${host}:${port}. Make sure the Health Auto Export app is open on your iPhone.`
        );
        setChecking(false);
        return;
      }
    } catch {
      setError(
        `Could not reach HAE server at ${host}:${port}. Make sure the Health Auto Export app is open on your iPhone.`
      );
      setChecking(false);
      return;
    }

    setChecking(false);
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
      setError(msg);
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
  const isBusy = starting || checking;
  const progress =
    status?.step && status?.total
      ? Math.round((status.step / status.total) * 100)
      : 0;

  return (
    <div className="space-y-5">
      <p className="text-sm text-zinc-400">
        Import health data from the Health Auto Export TCP server running on your
        iPhone. The app must be open and on the same network.
      </p>

      {/* Host + Port row */}
      <div className="flex gap-3">
        <div className="flex-[2]">
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            HAE Host
          </label>
          <input
            type="text"
            value={host}
            onChange={(e) => saveHost(e.target.value)}
            placeholder="linus-iphone"
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
          <p className="text-xs text-zinc-600 mt-1">
            Tailscale hostname (preferred) or local IP address
          </p>
        </div>
        <div className="flex-[1]">
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            Port
          </label>
          <input
            type="number"
            value={port}
            onChange={(e) => savePort(e.target.value)}
            disabled={isRunning}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 focus:border-cyan-600 focus:outline-none disabled:opacity-50"
          />
        </div>
      </div>

      {/* Date pickers row */}
      <div className="grid gap-3 grid-cols-2">
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            Start Date
          </label>
          <DatePickerInput
            value={startDate}
            onChange={setStartDate}
            disabled={isRunning}
            placeholder="Select start date"
          />
        </div>
        <div>
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            End Date
          </label>
          <DatePickerInput
            value={endDate}
            onChange={setEndDate}
            disabled={isRunning}
            placeholder="Select end date"
          />
        </div>
      </div>

      {/* Dry Run toggle */}
      <div className="flex items-center gap-3">
        <Toggle checked={dryRun} onChange={setDryRun} disabled={isRunning} />
        <div>
          <span className="text-sm text-zinc-300">Dry Run</span>
          <p className="text-xs text-zinc-600">Preview only, no data will be saved</p>
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex gap-3">
        {!isRunning ? (
          <button
            onClick={handleStart}
            disabled={isBusy || !host || !startDate || !endDate}
            className="flex items-center gap-2 px-5 py-2.5 bg-cyan-600 hover:bg-cyan-500 disabled:bg-zinc-700 disabled:text-zinc-500 text-white text-sm font-medium rounded-md transition-colors"
          >
            {checking ? (
              <>
                <Spinner />
                Connecting...
              </>
            ) : starting ? (
              <>
                <Spinner />
                Starting...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z" />
                </svg>
                Start Import
              </>
            )}
          </button>
        ) : (
          <button
            onClick={handleCancel}
            className="flex items-center gap-2 px-5 py-2.5 bg-red-600 hover:bg-red-500 text-white text-sm font-medium rounded-md transition-colors"
          >
            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M2 10a8 8 0 1116 0 8 8 0 01-16 0zm5-2.25A.75.75 0 017.75 7h4.5a.75.75 0 01.75.75v4.5a.75.75 0 01-.75.75h-4.5A.75.75 0 017 12.25v-4.5z" clipRule="evenodd" />
            </svg>
            Cancel Import
          </button>
        )}
      </div>

      {/* Error display */}
      {error && (
        <div className="bg-red-900/20 border border-red-800 rounded-lg p-4">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}

      {/* Progress display */}
      {status && (isRunning || status.done) && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 space-y-3">
          <div className="flex items-center justify-between text-sm">
            <span className="text-zinc-300">
              {isRunning ? (
                <span className="flex items-center gap-2">
                  <Spinner />
                  Importing:{" "}
                  <span className="text-cyan-400">{status.metric}</span>
                  {status.chunk && (
                    <span className="text-zinc-500">{status.chunk}</span>
                  )}
                </span>
              ) : (
                <span className="text-green-400">Import complete</span>
              )}
            </span>
            <span className="px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-400 font-mono text-xs">
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

// ---------------------------------------------------------------------------
// Alpha Import Section
// ---------------------------------------------------------------------------

function AlphaImportSection() {
  const fileRef = useRef<HTMLInputElement>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [result, setResult] = useState<{
    sets_received: number;
    sets_inserted: number;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState(false);

  function handleFileSelect(file: File | null) {
    setSelectedFile(file);
    setResult(null);
    setError(null);
  }

  async function handleUpload() {
    if (!selectedFile) return;

    setUploading(true);
    setError(null);
    setResult(null);

    try {
      const res = await uploadAlphaCSV(selectedFile);
      setResult(res);
      setSelectedFile(null);
      if (fileRef.current) fileRef.current.value = "";
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setUploading(false);
    }
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file && (file.name.endsWith(".csv") || file.type === "text/csv")) {
      handleFileSelect(file);
    }
  }

  return (
    <div className="space-y-5">
      <p className="text-sm text-zinc-400">
        Upload an Alpha Progression CSV export to import detailed set/rep/weight
        data for your strength training workouts.
      </p>

      {/* Drop zone */}
      <div
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
        onClick={() => fileRef.current?.click()}
        className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ${
          dragOver
            ? "border-cyan-500 bg-cyan-900/10"
            : selectedFile
              ? "border-cyan-700 bg-zinc-900"
              : "border-zinc-700 bg-zinc-900 hover:border-zinc-600"
        }`}
      >
        <input
          ref={fileRef}
          type="file"
          accept=".csv,text/csv"
          className="hidden"
          onChange={(e) => handleFileSelect(e.target.files?.[0] || null)}
        />

        {selectedFile ? (
          <div className="flex items-center justify-center gap-3">
            <svg className="w-5 h-5 text-cyan-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
            </svg>
            <span className="text-sm text-zinc-200">{selectedFile.name}</span>
            <button
              onClick={(e) => {
                e.stopPropagation();
                handleFileSelect(null);
                if (fileRef.current) fileRef.current.value = "";
              }}
              className="text-zinc-500 hover:text-zinc-300 transition-colors"
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
              </svg>
            </button>
          </div>
        ) : (
          <div>
            <svg className="w-8 h-8 text-zinc-600 mx-auto mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
            </svg>
            <p className="text-sm text-zinc-400">
              Drop CSV file here or click to browse
            </p>
          </div>
        )}
      </div>

      {/* Upload button */}
      {selectedFile && (
        <button
          onClick={handleUpload}
          disabled={uploading}
          className="flex items-center gap-2 px-5 py-2.5 bg-cyan-600 hover:bg-cyan-500 disabled:bg-zinc-700 disabled:text-zinc-500 text-white text-sm font-medium rounded-md transition-colors"
        >
          {uploading ? (
            <>
              <Spinner />
              Uploading...
            </>
          ) : (
            <>
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
              </svg>
              Upload
            </>
          )}
        </button>
      )}

      {/* Error */}
      {error && (
        <div className="bg-red-900/20 border border-red-800 rounded-lg p-4">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}

      {/* Result */}
      {result && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
          <p className="text-sm text-green-400 mb-2">Upload successful</p>
          <div className="flex gap-4 text-xs text-zinc-400">
            <span>Parsed: {result.sets_received} sets</span>
            <span>Imported: {result.sets_inserted} new</span>
            <span>
              Skipped: {result.sets_received - result.sets_inserted} duplicates
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Spinner
// ---------------------------------------------------------------------------

function Spinner() {
  return (
    <svg
      className="w-4 h-4 animate-spin text-current"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Main ImportTab
// ---------------------------------------------------------------------------

export default function ImportTab() {
  return (
    <div className="space-y-8">
      {/* HAE Section */}
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-5">
        <h3 className="text-lg font-semibold text-zinc-100 mb-4">
          Health Auto Export
        </h3>
        <HAEImportSection />
      </div>

      {/* Alpha Section */}
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-5">
        <h3 className="text-lg font-semibold text-zinc-100 mb-4">
          Strength Training (Alpha Progression)
        </h3>
        <AlphaImportSection />
      </div>
    </div>
  );
}
