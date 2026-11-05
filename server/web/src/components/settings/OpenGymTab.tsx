import { useCallback, useEffect, useState } from "react";
import {
  disconnectOpenGym,
  fetchOpenGymStatus,
  pairOpenGym,
  triggerOpenGymSync,
  type OpenGymStatus,
} from "../../api";
import { MONO, TabHeader } from "./parts";

const HINT = {
  font: "400 12px/1.5 var(--font-body)",
  color: "var(--color-neutral-600)",
  margin: "6px 0 0",
} as const;

export default function OpenGymTab() {
  const [status, setStatus] = useState<OpenGymStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [pairing, setPairing] = useState(false);
  const [baseUrl, setBaseUrl] = useState("");
  const [code, setCode] = useState("");
  const [syncFrom, setSyncFrom] = useState("");

  const load = useCallback(() => {
    setError(null);
    fetchOpenGymStatus()
      .then((s) => {
        setStatus(s);
        if (s.base_url) setBaseUrl(s.base_url);
        setSyncFrom(s.sync_from ?? s.suggested_sync_from ?? "");
      })
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handlePair() {
    setPairing(true);
    setError(null);
    try {
      await pairOpenGym(baseUrl, code, syncFrom);
      setCode("");
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Pairing failed");
    } finally {
      setPairing(false);
    }
  }

  async function handleSync() {
    setSyncing(true);
    setError(null);
    try {
      await triggerOpenGymSync();
      setTimeout(load, 3000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Sync failed");
    } finally {
      setSyncing(false);
    }
  }

  async function handleDisconnect() {
    if (
      !confirm(
        "Disconnect openGym? The token is removed from FreeReps; it stays valid in openGym until it expires or you use \"Sign out everywhere\" there. Sets already imported stay in the database.",
      )
    ) {
      return;
    }
    try {
      await disconnectOpenGym();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Disconnect failed");
    }
  }

  const pairForm = (
    <div style={{ paddingTop: 20, maxWidth: 520 }}>
      <div className="field" style={{ marginBottom: 14 }}>
        <label htmlFor="opengym-url">Instance</label>
        <input
          id="opengym-url"
          className="input"
          style={MONO}
          type="url"
          value={baseUrl}
          onChange={(e) => setBaseUrl(e.target.value)}
          placeholder="https://gym.example.net"
        />
      </div>
      <div className="field" style={{ marginBottom: 14 }}>
        <label htmlFor="opengym-code">Pairing code</label>
        <input
          id="opengym-code"
          className="input"
          style={MONO}
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="8 characters"
          autoComplete="off"
        />
        <p style={HINT}>
          In openGym: Settings → Account → "Pair the mobile app". The code has 8
          characters, is valid for 5 minutes and works once. The "Add a device"
          code (XXXX-XXXX-XXXX) on the same page is a different thing.
        </p>
      </div>
      <div className="field" style={{ marginBottom: 16 }}>
        <label htmlFor="opengym-from">Import from</label>
        <input
          id="opengym-from"
          className="input"
          style={MONO}
          type="date"
          value={syncFrom}
          min={status?.suggested_sync_from}
          onChange={(e) => setSyncFrom(e.target.value)}
        />
        <p style={HINT}>
          Workouts that started before this day are ignored. openGym's history
          includes the workouts imported from Hevy, which FreeReps already holds,
          so the day has to lie after the last Hevy session
          {status?.last_hevy_session ? ` (${status.last_hevy_session})` : ""}.
        </p>
      </div>
      <button
        type="button"
        className="btn btn-primary"
        onClick={handlePair}
        disabled={pairing || !baseUrl || !code || !syncFrom}
      >
        {pairing ? "Pairing…" : "Pair"}
      </button>
    </div>
  );

  return (
    <>
      <TabHeader title="openGym">
        The openGym state is checked for changes on a schedule. FreeReps pairs
        like the mobile app does; the token never reaches the browser.
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

      {!status ? (
        <p
          style={{
            color: "var(--color-neutral-600)",
            fontSize: 13,
            paddingTop: 16,
          }}
        >
          Loading…
        </p>
      ) : !status.configured ? (
        pairForm
      ) : (
        <>
          <div
            style={{
              border: "2px solid var(--color-text)",
              padding: "22px 24px",
              marginTop: 20,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span
                style={{
                  width: 11,
                  height: 11,
                  background: "var(--color-accent)",
                }}
              />
              <span style={{ font: "600 14px var(--font-body)" }}>Connected</span>
              <span style={{ marginLeft: "auto", display: "flex", gap: 8 }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ fontSize: 12 }}
                  onClick={handleSync}
                  disabled={syncing}
                >
                  {syncing ? "Syncing…" : "Sync now"}
                </button>
                <button
                  type="button"
                  className="btn btn-ghost"
                  style={{ fontSize: 12 }}
                  onClick={handleDisconnect}
                >
                  Disconnect
                </button>
              </span>
            </div>
            <div
              style={{
                font: "400 12px var(--font-body)",
                color: "var(--color-neutral-600)",
                marginTop: 8,
              }}
            >
              <span style={MONO}>{status.base_url}</span> · importing workouts from{" "}
              {status.sync_from}
              {status.last_change_read
                ? ` · last change read ${new Date(status.last_change_read).toLocaleString("en-GB")}`
                : ""}
            </div>
          </div>
          <p style={{ ...HINT, marginTop: 16, maxWidth: 520 }}>
            To pair again — after "Sign out everywhere" in openGym, or for a
            different instance — disconnect first. Settings → Import Logs carries
            the outcome of every run.
          </p>
        </>
      )}
    </>
  );
}
