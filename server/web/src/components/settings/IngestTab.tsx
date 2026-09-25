import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  fetchAllowlist,
  fetchImportLogs,
  saveMetricEnabled,
  type ImportLog,
} from "../../api";
import { useIsDesktop } from "../../hooks/useMediaQuery";
import { formatNumber } from "../../utils/format";
import { MONO, SquareCheckbox, SquareSwitch, TabHeader } from "./parts";

export default function IngestTab() {
  const logs = useQuery({
    queryKey: ["import-logs", 25],
    queryFn: () => fetchImportLogs(25),
  });

  const serverURL =
    typeof window === "undefined" ? "" : `${window.location.origin}/api/v1/ingest`;

  return (
    <>
      <TabHeader title="Ingest">
        Point Health Auto Export at this URL. Requests arrive over the tailnet,
        so the reverse proxy is what authenticates them.
      </TabHeader>

      <div style={{ paddingTop: 20 }}>
        <label className="kick" htmlFor="ingest-url">
          Server URL
        </label>
        <input
          id="ingest-url"
          className="input"
          style={{ ...MONO, maxWidth: 620, marginTop: 8 }}
          value={serverURL}
          readOnly
          onFocus={(e) => e.currentTarget.select()}
        />
      </div>

      <AcceptedMetrics />

      <div style={{ paddingTop: 30 }}>
        <h3 style={{ fontSize: 15, fontWeight: 700 }}>Recent ingests</h3>
        <div
          style={{
            borderTop: "2px solid var(--color-text)",
            marginTop: 12,
          }}
        >
          {logs.data && logs.data.length > 0 ? (
            logs.data.map((log) => <IngestRow key={log.id} log={log} />)
          ) : (
            <p
              style={{
                color: "var(--color-neutral-600)",
                fontSize: 13,
                paddingTop: 14,
              }}
            >
              No ingests recorded yet.
            </p>
          )}
        </div>
      </div>
    </>
  );
}

/**
 * Which metrics the ingest stores for the signed-in user. A disabled metric is
 * rejected on arrival from every client, and the iOS app reads the same list to
 * skip it before reading HealthKit. Data already stored is kept.
 */
function AcceptedMetrics() {
  const isDesktop = useIsDesktop();
  const queryClient = useQueryClient();
  const allowlist = useQuery({ queryKey: ["allowlist"], queryFn: fetchAllowlist });
  const [enabled, setEnabled] = useState<Record<string, boolean>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!allowlist.data) return;
    const init: Record<string, boolean> = {};
    for (const m of allowlist.data) init[m.metric_name] = m.enabled;
    setEnabled(init);
  }, [allowlist.data]);

  const entries = allowlist.data ?? [];
  const enabledCount = entries.filter((m) => enabled[m.metric_name]).length;

  async function save() {
    setSaving(true);
    setError(null);
    try {
      await saveMetricEnabled(enabled);
      queryClient.invalidateQueries({ queryKey: ["allowlist"] });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ paddingTop: 30 }}>
      <h3 style={{ fontSize: 15, fontWeight: 700 }}>Accepted metrics</h3>
      <p
        style={{
          font: "400 12px var(--font-body)",
          color: "var(--color-neutral-600)",
          margin: "6px 0 4px",
        }}
      >
        {enabledCount} of {entries.length} accepted. A disabled metric is
        rejected from every client; stored data is kept.
      </p>

      <div style={{ borderTop: "2px solid var(--color-text)" }}>
        {entries.map((m) => {
          const on = enabled[m.metric_name] ?? false;
          const label = m.display_label || m.metric_name;
          const toggle = () =>
            setEnabled((prev) => ({ ...prev, [m.metric_name]: !on }));
          return (
            <div
              key={m.metric_name}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 14,
                padding: "11px 0",
                borderBottom: "1px solid var(--color-neutral-300)",
              }}
            >
              {isDesktop ? (
                <SquareCheckbox checked={on} onChange={toggle} label={`Accept ${label}`} />
              ) : (
                <SquareSwitch checked={on} onChange={toggle} label={`Accept ${label}`} />
              )}
              <span
                style={{
                  font: "500 13.5px var(--font-body)",
                  color: on ? "var(--color-text)" : "var(--color-neutral-600)",
                  flex: 1,
                  minWidth: 0,
                }}
              >
                {label}
              </span>
              <span className="kick" style={{ width: 120, flex: "none" }}>
                {m.category}
              </span>
            </div>
          );
        })}
      </div>

      {error ? (
        <p style={{ color: "var(--color-accent-700)", fontSize: 13, marginTop: 14 }}>
          {error}
        </p>
      ) : null}

      <div style={{ paddingTop: 20 }}>
        <button
          type="button"
          className="btn btn-primary"
          onClick={save}
          disabled={saving || !allowlist.data}
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}

function IngestRow({ log }: { log: ImportLog }) {
  const summary = [
    log.metrics_inserted
      ? `${formatNumber(log.metrics_inserted)} metrics`
      : null,
    log.workouts_inserted
      ? `${formatNumber(log.workouts_inserted)} workouts`
      : null,
    log.sleep_sessions ? `${formatNumber(log.sleep_sessions)} nights` : null,
    log.sets_inserted ? `${formatNumber(log.sets_inserted)} sets` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div
      style={{
        display: "flex",
        alignItems: "baseline",
        gap: 16,
        padding: "12px 0",
        borderBottom: "1px solid var(--color-neutral-300)",
      }}
    >
      <span
        className="num"
        style={{
          width: 150,
          flex: "none",
          font: "400 12px var(--font-body)",
          color: "var(--color-neutral-600)",
        }}
      >
        {new Date(log.created_at).toLocaleString("en-GB", {
          day: "numeric",
          month: "short",
          hour: "2-digit",
          minute: "2-digit",
        })}
      </span>
      <span
        style={{ width: 180, flex: "none", font: "500 13px var(--font-body)" }}
      >
        {log.source || "—"}
      </span>
      <span
        style={{
          flex: 1,
          minWidth: 0,
          font: "400 12.5px var(--font-body)",
          color: "var(--color-neutral-700)",
        }}
      >
        {log.error_message || summary || "nothing new"}
      </span>
      <span
        className={`tag ${log.status === "success" ? "tag-accent" : "tag-neutral"}`}
        style={{ flex: "none" }}
      >
        {log.status === "success" ? "OK" : log.status}
      </span>
    </div>
  );
}
