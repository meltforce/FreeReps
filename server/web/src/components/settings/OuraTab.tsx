import { useCallback, useEffect, useState } from "react";
import {
  fetchOuraStatus,
  saveOuraCredentials,
  authorizeOura,
  triggerOuraSync,
  disconnectOura,
  type OuraStatus,
} from "../../api";

export default function OuraTab() {
  const [status, setStatus] = useState<OuraStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(() => {
    setError(null);
    fetchOuraStatus()
      .then((s) => {
        setStatus(s);
        if (s.client_id) setClientId(s.client_id);
      })
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Check for error from OAuth callback redirect.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const ouraError = params.get("error");
    if (ouraError) {
      setError(`Oura authorization failed: ${ouraError}`);
    }
  }, []);

  if (error) {
    return (
      <div>
        <p className="text-red-400 mb-4">{error}</p>
        <button onClick={load} className="text-sm text-cyan-400 hover:underline">
          Retry
        </button>
      </div>
    );
  }
  if (!status) {
    return <p className="text-zinc-500">Loading...</p>;
  }

  async function handleSaveCredentials() {
    setSaving(true);
    setError(null);
    try {
      await saveOuraCredentials(clientId, clientSecret);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save credentials");
    } finally {
      setSaving(false);
    }
  }

  async function handleConnect() {
    try {
      const { authorize_url } = await authorizeOura();
      window.location.href = authorize_url;
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to start authorization");
    }
  }

  async function handleSync() {
    setSyncing(true);
    try {
      await triggerOuraSync();
      setTimeout(load, 3000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Sync failed");
    } finally {
      setSyncing(false);
    }
  }

  async function handleDisconnect() {
    if (!confirm("Disconnect Oura Ring? This removes stored tokens and credentials.")) {
      return;
    }
    try {
      await disconnectOura();
      setClientId("");
      setClientSecret("");
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Disconnect failed");
    }
  }

  // Step 1: No credentials saved yet — show credential form.
  if (!status.configured) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <h3 className="text-zinc-200 font-medium mb-2">Connect Oura Ring</h3>
        <p className="text-zinc-400 text-sm mb-4">
          Register an app at{" "}
          <a
            href="https://cloud.ouraring.com/oauth/applications"
            target="_blank"
            rel="noopener noreferrer"
            className="text-cyan-400 hover:underline"
          >
            cloud.ouraring.com
          </a>{" "}
          and enter your credentials below.
        </p>
        <div className="space-y-3 mb-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wide mb-1">
              Client ID
            </label>
            <input
              type="text"
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
              className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-sm text-zinc-200 font-mono focus:outline-none focus:border-cyan-600"
              placeholder="Oura client ID"
            />
          </div>
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wide mb-1">
              Client Secret
            </label>
            <input
              type="password"
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-2 text-sm text-zinc-200 font-mono focus:outline-none focus:border-cyan-600"
              placeholder="Oura client secret"
            />
          </div>
        </div>
        <button
          onClick={handleSaveCredentials}
          disabled={saving || !clientId || !clientSecret}
          className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white rounded-lg font-medium transition-colors"
        >
          {saving ? "Saving..." : "Save Credentials"}
        </button>
      </div>
    );
  }

  // Step 2: Credentials saved but not yet authorized.
  if (!status.connected) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <div className="flex items-center gap-2 mb-3">
          <span className="w-2 h-2 bg-amber-400 rounded-full" />
          <span className="text-zinc-200 font-medium">Credentials Saved</span>
        </div>
        <p className="text-zinc-400 text-sm mb-4">
          Client ID: <code className="text-zinc-300">{status.client_id}</code>
        </p>
        <div className="flex gap-2">
          <button
            onClick={handleConnect}
            className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg font-medium transition-colors"
          >
            Authorize with Oura
          </button>
          <button
            onClick={handleDisconnect}
            className="px-3 py-2 text-sm bg-zinc-700 hover:bg-zinc-600 text-zinc-300 rounded-lg transition-colors"
          >
            Remove
          </button>
        </div>
      </div>
    );
  }

  // Step 3: Fully connected — show status and controls.
  return (
    <div>
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 bg-emerald-400 rounded-full" />
            <span className="text-zinc-200 font-medium">Connected</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleSync}
              disabled={syncing}
              className="px-3 py-1.5 text-sm bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white rounded-md transition-colors"
            >
              {syncing ? "Syncing..." : "Sync Now"}
            </button>
            <button
              onClick={handleDisconnect}
              className="px-3 py-1.5 text-sm bg-zinc-700 hover:bg-zinc-600 text-zinc-300 rounded-md transition-colors"
            >
              Disconnect
            </button>
          </div>
        </div>
        <p className="text-xs text-zinc-500">
          Client ID: {status.client_id}
        </p>
        {status.expires_at && (
          <p className="text-xs text-zinc-500">
            Token expires: {new Date(status.expires_at).toLocaleString()}
          </p>
        )}
      </div>

      {status.sync_states && Object.keys(status.sync_states).length > 0 && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
          <h3 className="text-sm font-medium text-zinc-300 mb-3">Sync Status</h3>
          <div className="grid gap-2">
            {Object.entries(status.sync_states)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([dataType, lastSync]) => (
                <div key={dataType} className="flex justify-between text-sm">
                  <span className="text-zinc-400">{dataType.replace(/_/g, " ")}</span>
                  <span className="text-zinc-500 font-mono">{lastSync}</span>
                </div>
              ))}
          </div>
        </div>
      )}
    </div>
  );
}
