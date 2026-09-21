import { useCallback, useEffect, useState } from "react";
import {
  fetchAlerts,
  saveAlertSettings,
  sendTestAlert,
  type AlertCondition,
  type AlertSettings,
} from "../../api";
import { MONO, Row, TabHeader } from "./parts";

/** Seconds as minutes and hours, for the two interval fields. */
const MIN = 60;
const HOUR = 3600;

function describeAge(iso?: string): string {
  if (!iso) return "not checked yet";
  const then = new Date(iso).getTime();
  const mins = Math.round((Date.now() - then) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.round(mins / 60);
  if (hours < 48) return `${hours} h ago`;
  return `${Math.round(hours / 24)} d ago`;
}

function ConditionRow({ c }: { c: AlertCondition }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "baseline",
        gap: 12,
        padding: "10px 0",
        borderBottom: "1px solid var(--color-neutral-300)",
      }}
    >
      <span
        style={{
          width: 10,
          height: 10,
          flex: "none",
          alignSelf: "center",
          background: c.firing
            ? "var(--color-accent)"
            : "var(--color-neutral-400)",
        }}
      />
      <span style={{ font: "600 13px var(--font-body)", minWidth: 220 }}>
        {c.service}
      </span>
      <span style={{ ...MONO, fontSize: 12, color: "var(--color-neutral-600)" }}>
        {c.monitor_id}
      </span>
      <span
        style={{
          font: "400 12px/1.5 var(--font-body)",
          color: "var(--color-neutral-600)",
          flex: 1,
          minWidth: 0,
        }}
      >
        {c.firing ? "reported as a problem" : "quiet"} · checked{" "}
        {describeAge(c.checked_at)}
        {c.last_msg ? ` · ${c.last_msg}` : ""}
      </span>
    </div>
  );
}

export default function AlertsTab() {
  const [settings, setSettings] = useState<AlertSettings | null>(null);
  const [conditions, setConditions] = useState<AlertCondition[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  const load = useCallback(() => {
    setError(null);
    fetchAlerts()
      .then((r) => {
        setSettings(r.settings);
        setConditions(r.conditions);
      })
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  function patch(next: Partial<AlertSettings>) {
    setSettings((s) => (s ? { ...s, ...next } : s));
  }

  async function handleSave() {
    if (!settings) return;
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      await saveAlertSettings(settings);
      setNotice("Saved. The next check cycle uses these values.");
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    setTesting(true);
    setError(null);
    setNotice(null);
    try {
      const r = await sendTestAlert();
      setNotice(`Test message sent to ${r.target} on monitor_id ${r.monitor_id}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Test failed");
    } finally {
      setTesting(false);
    }
  }

  return (
    <>
      <TabHeader title="Alerts">
        A data source that stops delivering is reported to an{" "}
        <a href="https://ntfy.sh" target="_blank" rel="noreferrer">
          ntfy
        </a>{" "}
        topic, so a silent integration surfaces without anyone reading the import
        log. The payload follows Uptime Kuma's webhook shape, and each condition
        carries its own monitor id — a receiver can group by it. Stored in the
        database, not in the config file.
      </TabHeader>

      {error ? (
        <p
          style={{
            color: "var(--color-accent-700)",
            fontSize: 13,
            paddingTop: 16,
          }}
        >
          {error}{" "}
          <button type="button" className="btn btn-ghost" onClick={load}>
            Retry
          </button>
        </p>
      ) : null}

      {notice ? (
        <p
          style={{
            font: "400 13px/1.5 var(--font-body)",
            color: "var(--color-neutral-700)",
            paddingTop: 16,
          }}
        >
          {notice}
        </p>
      ) : null}

      {!settings ? (
        <p
          style={{
            color: "var(--color-neutral-600)",
            fontSize: 13,
            paddingTop: 16,
          }}
        >
          Loading…
        </p>
      ) : (
        <>
          <div style={{ paddingTop: 12, maxWidth: 640 }}>
            <Row label="Reporting">
              <label style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <input
                  type="checkbox"
                  checked={settings.enabled}
                  onChange={(e) => patch({ enabled: e.target.checked })}
                />
                <span style={{ font: "400 13px var(--font-body)" }}>
                  {settings.enabled
                    ? "Conditions are reported"
                    : "Nothing is reported"}
                </span>
              </label>
            </Row>

            <Row label="ntfy topic URL">
              <input
                className="input"
                style={{ ...MONO, width: "100%" }}
                type="url"
                value={settings.ntfy_url}
                onChange={(e) => patch({ ntfy_url: e.target.value })}
                placeholder="https://ntfy.example.com/freereps-alerts"
              />
              <p
                style={{
                  font: "400 12px/1.5 var(--font-body)",
                  color: "var(--color-neutral-600)",
                  margin: "6px 0 0",
                }}
              >
                The full topic URL, including the topic name — that name is what
                a subscriber listens on. Any endpoint accepting an ntfy-style
                POST works; the body is JSON either way.
              </p>
            </Row>

            <Row label="Reported as">
              <input
                className="input"
                style={{ ...MONO, width: 260 }}
                value={settings.hostname}
                onChange={(e) => patch({ hostname: e.target.value })}
                placeholder="freereps"
              />
              <p
                style={{
                  font: "400 12px/1.5 var(--font-body)",
                  color: "var(--color-neutral-600)",
                  margin: "6px 0 0",
                }}
              >
                Fills the payload's <code>hostname</code>, so a receiver can
                tell a test instance from the production one.
              </p>
            </Row>

            <Row label="Check every">
              <span style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                <input
                  className="input"
                  style={{ ...MONO, width: 90 }}
                  type="number"
                  min={1}
                  value={Math.round(settings.check_interval_sec / MIN)}
                  onChange={(e) =>
                    patch({
                      check_interval_sec: Math.max(1, Number(e.target.value)) * MIN,
                    })
                  }
                />
                <span style={{ font: "400 13px var(--font-body)" }}>minutes</span>
              </span>
            </Row>

            <Row label="Failures before alert">
              <span style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                <input
                  className="input"
                  style={{ ...MONO, width: 90 }}
                  type="number"
                  min={1}
                  value={settings.failure_threshold}
                  onChange={(e) =>
                    patch({ failure_threshold: Math.max(1, Number(e.target.value)) })
                  }
                />
                <span
                  style={{
                    font: "400 12px/1.5 var(--font-body)",
                    color: "var(--color-neutral-600)",
                  }}
                >
                  consecutive failed runs of one source for one user. At a
                  30-minute sync interval, 3 means a defect is reported within
                  two hours while a single DNS timeout is not.
                </span>
              </span>
            </Row>

            <Row label="Apple Health silence">
              <span style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                <input
                  className="input"
                  style={{ ...MONO, width: 90 }}
                  type="number"
                  min={0}
                  value={Math.round(settings.apple_silence_sec / HOUR)}
                  onChange={(e) =>
                    patch({
                      apple_silence_sec: Math.max(0, Number(e.target.value)) * HOUR,
                    })
                  }
                />
                <span
                  style={{
                    font: "400 12px/1.5 var(--font-body)",
                    color: "var(--color-neutral-600)",
                  }}
                >
                  hours without a Health Auto Export delivery that stored a row
                  before the ingress counts as down. An export window that
                  repeats the same days keeps delivering without storing
                  anything, which is why the rule counts stored rows. 0 turns
                  the rule off.
                </span>
              </span>
            </Row>
          </div>

          <div style={{ display: "flex", gap: 8, paddingTop: 20 }}>
            <button
              type="button"
              className="btn btn-primary"
              onClick={handleSave}
              disabled={saving}
            >
              {saving ? "Saving…" : "Save"}
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={handleTest}
              disabled={testing || !settings.ntfy_url}
            >
              {testing ? "Sending…" : "Send test message"}
            </button>
          </div>

          <div style={{ paddingTop: 30, maxWidth: 640 }}>
            <span className="kick">Conditions</span>
            <div style={{ marginTop: 10 }}>
              {conditions.map((c) => (
                <ConditionRow key={c.monitor_id} c={c} />
              ))}
            </div>
          </div>
        </>
      )}
    </>
  );
}
