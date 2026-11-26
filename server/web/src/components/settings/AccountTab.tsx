import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createToken,
  fetchMe,
  fetchSessions,
  fetchTokens,
  revokeSession,
  revokeToken,
  type CreatedToken,
  type TokenInfo,
  type TokenScope,
} from "../../api";
import { canSignOut, signOut } from "../../auth";
import { MONO, Row, SquareCheckbox, TabHeader } from "./parts";

const NOTE: React.CSSProperties = {
  font: "400 12px/1.5 var(--font-body)",
  color: "var(--color-neutral-600)",
};

const ERROR: React.CSSProperties = {
  color: "var(--color-accent-700)",
  fontSize: 13,
  paddingTop: 12,
};

/** One entry of a list: text left, action right; the action wraps below on a phone. */
export const ITEM: React.CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  alignItems: "center",
  justifyContent: "space-between",
  gap: "8px 16px",
  padding: "13px 0",
  borderBottom: "1px solid var(--color-neutral-300)",
};

export function formatWhen(ts: string | undefined): string {
  return ts ? new Date(ts).toLocaleString("en-GB") : "never";
}

const METHOD_LABEL: Record<string, string> = {
  session: "Application login",
  token: "Access token",
  tailscale: "Tailscale",
  dev: "Local development user",
};

export default function AccountTab() {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const [signOutFailed, setSignOutFailed] = useState(false);

  return (
    <>
      <TabHeader title="Account">
        The browser sessions signed in to this account and the access tokens
        that let apps and scripts act as it.
      </TabHeader>

      <Row label="Signed in as">
        {me.data?.display_name || me.data?.login || "—"}
        {me.data?.is_admin ? (
          <span className="tag tag-accent" style={{ marginLeft: 10 }}>
            Admin
          </span>
        ) : null}
      </Row>
      <Row label="Through">
        {me.data ? (METHOD_LABEL[me.data.auth_method] ?? me.data.auth_method) : "—"}
      </Row>
      {canSignOut(me.data?.auth_method) ? (
        <Row label="This browser">
          <button
            type="button"
            className="btn btn-secondary"
            style={{ fontSize: 12 }}
            onClick={async () => setSignOutFailed(!(await signOut()))}
          >
            Sign out
          </button>
          {signOutFailed ? <p style={ERROR}>Sign-out failed.</p> : null}
        </Row>
      ) : null}

      <Sessions />
      <Tokens />
    </>
  );
}

function Sessions() {
  const qc = useQueryClient();
  const sessions = useQuery({ queryKey: ["sessions"], queryFn: fetchSessions });
  const revoke = useMutation({
    mutationFn: revokeSession,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["sessions"] }),
  });

  return (
    <div style={{ paddingTop: 30 }}>
      <h3 style={{ fontSize: 15, fontWeight: 700 }}>Sessions</h3>
      <div style={{ borderTop: "2px solid var(--color-text)", marginTop: 12 }}>
        {sessions.data?.length === 0 ? (
          <p style={{ ...NOTE, fontSize: 13, paddingTop: 14 }}>
            No browser session; this request was not signed in through one.
          </p>
        ) : null}
        {sessions.data?.map((s) => (
          <div key={s.id} style={ITEM}>
            <div style={{ minWidth: 0, flex: "1 1 240px" }}>
              <div
                style={{
                  fontSize: 14,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={s.user_agent}
              >
                {s.user_agent || s.method}
              </div>
              <div style={{ ...NOTE, marginTop: 3 }}>
                <span style={MONO}>{s.client_ip}</span> · last used{" "}
                {formatWhen(s.last_used_at)} · expires{" "}
                {formatWhen(s.expires_at)}
              </div>
            </div>
            {s.current ? (
              <span className="tag tag-accent">This browser</span>
            ) : (
              <button
                type="button"
                className="btn btn-ghost"
                style={{ fontSize: 12 }}
                disabled={revoke.isPending}
                onClick={() => revoke.mutate(s.id)}
              >
                End session
              </button>
            )}
          </div>
        ))}
      </div>
      {sessions.error ? <p style={ERROR}>{sessions.error.message}</p> : null}
      {revoke.error ? <p style={ERROR}>{revoke.error.message}</p> : null}
    </div>
  );
}

const SCOPES: { value: TokenScope; label: string }[] = [
  { value: "api", label: "api" },
  { value: "mcp", label: "mcp" },
];

/** Durations the server parses; empty means no expiry. */
const EXPIRY: { value: string; label: string }[] = [
  { value: "30d", label: "30 days" },
  { value: "90d", label: "90 days" },
  { value: "365d", label: "1 year" },
  { value: "", label: "Never" },
];

function Tokens() {
  const qc = useQueryClient();
  const tokens = useQuery({ queryKey: ["tokens"], queryFn: fetchTokens });
  const [name, setName] = useState("");
  const [scopes, setScopes] = useState<TokenScope[]>(["api"]);
  const [expires, setExpires] = useState("90d");
  const [created, setCreated] = useState<CreatedToken | null>(null);
  const [copied, setCopied] = useState(false);

  const create = useMutation({
    mutationFn: () => createToken(name.trim(), scopes, expires),
    onSuccess: (res) => {
      setCreated(res);
      setCopied(false);
      setName("");
      void qc.invalidateQueries({ queryKey: ["tokens"] });
    },
  });
  const revoke = useMutation({
    mutationFn: revokeToken,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tokens"] }),
  });

  function toggleScope(s: TokenScope) {
    setScopes((cur) =>
      cur.includes(s) ? cur.filter((x) => x !== s) : [...cur, s],
    );
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }

  function handleRevoke(tk: TokenInfo) {
    if (
      !confirm(
        `Revoke the token "${tk.name}"? Every client that uses it loses access at once.`,
      )
    ) {
      return;
    }
    if (created?.detail.id === tk.id) setCreated(null);
    revoke.mutate(tk.id);
  }

  async function copy() {
    if (!created) return;
    try {
      // Absent outside a secure context; the field stays selectable.
      await navigator.clipboard.writeText(created.token);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div style={{ paddingTop: 30 }}>
      <h3 style={{ fontSize: 15, fontWeight: 700 }}>Access tokens</h3>
      <p
        style={{
          font: "400 13px/1.55 var(--font-body)",
          color: "var(--color-neutral-700)",
          maxWidth: "62ch",
          margin: "8px 0 0",
        }}
      >
        A token signs a client in as this account without a browser session.
        Scope <code>api</code> is for the FreeReps iOS app, Health Auto Export
        and scripts, which send <code>Authorization: Bearer &lt;token&gt;</code>;
        scope <code>mcp</code> is for MCP clients at <code>/mcp</code>.
      </p>

      {created ? (
        <div
          style={{
            border: "2px solid var(--color-text)",
            padding: "18px 20px",
            marginTop: 18,
          }}
        >
          <label className="kick" htmlFor="token-secret">
            {created.detail.name} · shown once
          </label>
          <div
            style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 8 }}
          >
            <input
              id="token-secret"
              className="input"
              style={{ ...MONO, flex: "1 1 260px", width: "auto" }}
              readOnly
              value={created.token}
              onFocus={(e) => e.currentTarget.select()}
            />
            <button type="button" className="btn btn-primary" onClick={copy}>
              {copied ? "Copied" : "Copy"}
            </button>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => setCreated(null)}
            >
              Dismiss
            </button>
          </div>
          <p style={{ ...NOTE, margin: "8px 0 0" }}>
            The server stores only a hash of the token, so it cannot be
            displayed again. Copy it now.
          </p>
        </div>
      ) : null}

      <div style={{ borderTop: "2px solid var(--color-text)", marginTop: 18 }}>
        {tokens.data?.length === 0 ? (
          <p style={{ ...NOTE, fontSize: 13, paddingTop: 14 }}>
            No access tokens.
          </p>
        ) : null}
        {tokens.data?.map((tk) => (
          <div key={tk.id} style={ITEM}>
            <div style={{ minWidth: 0, flex: "1 1 240px" }}>
              <div
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  alignItems: "center",
                  gap: 8,
                  fontSize: 14,
                }}
              >
                <span style={{ overflowWrap: "anywhere" }}>{tk.name}</span>
                {tk.scopes.map((s) => (
                  <span key={s} className="tag tag-neutral" style={MONO}>
                    {s}
                  </span>
                ))}
                {tk.expired ? (
                  <span className="tag tag-outline">Expired</span>
                ) : null}
              </div>
              <div style={{ ...NOTE, marginTop: 3 }}>
                created {formatWhen(tk.created_at)} · last used{" "}
                {formatWhen(tk.last_used_at)} ·{" "}
                {tk.expires_at
                  ? `${tk.expired ? "expired" : "expires"} ${formatWhen(tk.expires_at)}`
                  : "no expiry"}
              </div>
            </div>
            <button
              type="button"
              className="btn btn-ghost"
              style={{ fontSize: 12 }}
              disabled={revoke.isPending}
              onClick={() => handleRevoke(tk)}
            >
              Revoke
            </button>
          </div>
        ))}
      </div>
      {tokens.error ? <p style={ERROR}>{tokens.error.message}</p> : null}
      {revoke.error ? <p style={ERROR}>{revoke.error.message}</p> : null}

      <form onSubmit={submit} style={{ paddingTop: 24, maxWidth: 520 }}>
        <div className="field" style={{ marginBottom: 14 }}>
          <label htmlFor="token-name">New token — name</label>
          <input
            id="token-name"
            className="input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="iPhone, Claude Desktop, backup script"
          />
        </div>

        <div className="field" style={{ marginBottom: 14 }}>
          <label>Scopes</label>
          <div style={{ display: "flex", gap: 22 }}>
            {SCOPES.map((s) => (
              <span
                key={s.value}
                style={{ display: "inline-flex", alignItems: "center", gap: 8 }}
              >
                <SquareCheckbox
                  checked={scopes.includes(s.value)}
                  onChange={() => toggleScope(s.value)}
                  label={`Scope ${s.label}`}
                />
                <span style={MONO}>{s.label}</span>
              </span>
            ))}
          </div>
        </div>

        <div className="field" style={{ marginBottom: 16 }}>
          <label>Expires after</label>
          <span className="seg">
            {EXPIRY.map((o) => (
              <label key={o.value} className="seg-opt">
                <input
                  type="radio"
                  name="token-expires"
                  value={o.value}
                  checked={expires === o.value}
                  onChange={() => setExpires(o.value)}
                />
                {o.label}
              </label>
            ))}
          </span>
        </div>

        <button
          type="submit"
          className="btn btn-primary"
          disabled={!name.trim() || scopes.length === 0 || create.isPending}
        >
          {create.isPending ? "Creating…" : "Create token"}
        </button>
        {create.error ? <p style={ERROR}>{create.error.message}</p> : null}
      </form>
    </div>
  );
}
