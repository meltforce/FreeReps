import { useEffect, useState } from "react";
import { fetchMe, type UserInfo } from "../../api";

export default function IdentityTab() {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchMe().then(setUser).catch((e) => setError(e.message));
  }, []);

  if (error) {
    return <p className="text-red-400">Failed to load identity: {error}</p>;
  }
  if (!user) {
    return <p className="text-zinc-500">Loading...</p>;
  }

  const fields = [
    { label: "Display Name", value: user.display_name || "—" },
    { label: "Login", value: user.login },
    { label: "Hostname", value: user.tailscale_id || "—" },
    { label: "Tailnet", value: user.tailnet || "—" },
  ];

  return (
    <div>
      <div className="grid gap-4 sm:grid-cols-2 mb-4">
        {fields.map((f) => (
          <div
            key={f.label}
            className="bg-zinc-900 border border-zinc-800 rounded-lg p-4"
          >
            <p className="text-xs text-zinc-500 uppercase tracking-wide mb-1">
              {f.label}
            </p>
            <p className="text-zinc-100 font-mono text-sm break-all">
              {f.value}
            </p>
          </div>
        ))}
      </div>
      <p className="text-xs text-zinc-600">Identity is managed by your Tailscale account</p>
    </div>
  );
}
