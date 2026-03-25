import { useCallback, useEffect, useState } from "react";
import {
  fetchOuraStatus,
  authorizeOura,
  triggerOuraSync,
  disconnectOura,
  type OuraStatus,
} from "../../api";

export default function OuraTab() {
  const [status, setStatus] = useState<OuraStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);

  const load = useCallback(() => {
    fetchOuraStatus().then(setStatus).catch((e) => setError(e.message));
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
    return <p className="text-red-400">{error}</p>;
  }
  if (!status) {
    return <p className="text-zinc-500">Loading...</p>;
  }
  if (!status.enabled) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <p className="text-zinc-400">
          Oura integration is not configured. Add <code className="text-cyan-400">oura.enabled: true</code> with
          your client credentials in <code className="text-cyan-400">config.yaml</code> to enable it.
        </p>
      </div>
    );
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
      // Reload status after a short delay to show updated sync times.
      setTimeout(load, 3000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Sync failed");
    } finally {
      setSyncing(false);
    }
  }

  async function handleDisconnect() {
    if (!confirm("Disconnect Oura Ring? This removes stored tokens and stops syncing.")) {
      return;
    }
    try {
      await disconnectOura();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Disconnect failed");
    }
  }

  if (!status.connected) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6 text-center">
        <p className="text-zinc-300 mb-4">Connect your Oura Ring to sync health data.</p>
        <button
          onClick={handleConnect}
          className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg font-medium transition-colors"
        >
          Connect Oura Ring
        </button>
      </div>
    );
  }

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
