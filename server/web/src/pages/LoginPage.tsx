import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Navigate, useLocation } from "react-router-dom";
import { fetchAuthMethods, passwordLogin } from "../api";
import { autoStartURL, loginErrorText, safeNext } from "../auth";

const NOTE: React.CSSProperties = {
  font: "400 13px/1.5 var(--font-body)",
  margin: "0 0 16px",
};

// /login lists the methods of GET /auth/methods. With OIDC as the only method
// it hands over to the provider at once (autoStartURL).
export default function LoginPage() {
  const { search } = useLocation();
  const methods = useQuery({
    queryKey: ["auth-methods"],
    queryFn: fetchAuthMethods,
    retry: 0,
  });
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const start = methods.data ? autoStartURL(methods.data, search) : null;
  useEffect(() => {
    if (start) window.location.replace(start);
  }, [start]);

  // 404 from /auth/methods: the instance has no application login.
  if (methods.data === null) return <Navigate to="/" replace />;
  if (methods.isLoading || start) {
    return (
      <Shell>
        <p style={{ ...NOTE, color: "var(--color-neutral-600)" }}>Loading…</p>
      </Shell>
    );
  }

  const params = new URLSearchParams(search);
  const errorCode = params.get("error");
  const next = safeNext(search);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setFormError(null);
    try {
      await passwordLogin(username, password);
      // A full load, so every cached query starts over with the session.
      window.location.assign(next);
    } catch (err) {
      const status = err instanceof Error ? err.message : "";
      setFormError(
        loginErrorText(
          status === "429"
            ? "rate_limited"
            : status === "401"
              ? "invalid_credentials"
              : status === "403"
                ? "no_account"
                : "login_failed",
        ),
      );
      setBusy(false);
    }
  }

  const oidc = (methods.data ?? []).filter(
    (m) => m.type === "oidc" && m.start_url,
  );
  const hasPassword = (methods.data ?? []).some((m) => m.type === "password");

  return (
    <Shell>
      {params.has("signed_out") ? (
        <p style={{ ...NOTE, color: "var(--color-neutral-700)" }}>
          You are signed out.
        </p>
      ) : null}
      {errorCode ? (
        <p role="alert" style={{ ...NOTE, color: "var(--color-accent-700)" }}>
          {loginErrorText(errorCode)}
        </p>
      ) : null}
      {methods.error ? (
        <p role="alert" style={{ ...NOTE, color: "var(--color-accent-700)" }}>
          {methods.error.message}
        </p>
      ) : null}

      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        {oidc.map((m) => (
          <button
            key={m.start_url}
            type="button"
            className="btn btn-primary"
            onClick={() =>
              window.location.assign(
                `${m.start_url}?next=${encodeURIComponent(next)}`,
              )
            }
          >
            Sign in with {m.display_name || "single sign-on"}
          </button>
        ))}

        {hasPassword ? (
          <form onSubmit={submit}>
            <div className="field" style={{ marginBottom: 14 }}>
              <label htmlFor="login-username">Username</label>
              <input
                id="login-username"
                className="input"
                autoComplete="username"
                autoCapitalize="none"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
            <div className="field" style={{ marginBottom: 16 }}>
              <label htmlFor="login-password">Password</label>
              <input
                id="login-password"
                className="input"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {formError ? (
              <p
                role="alert"
                style={{ ...NOTE, color: "var(--color-accent-700)" }}
              >
                {formError}
              </p>
            ) : null}
            <button
              type="submit"
              className={oidc.length > 0 ? "btn btn-secondary" : "btn btn-primary"}
              style={{ width: "100%" }}
              disabled={busy || !username || !password}
            >
              {busy ? "Signing in…" : "Sign in"}
            </button>
          </form>
        ) : null}
      </div>
    </Shell>
  );
}

function Shell({ children }: { children: ReactNode }) {
  return (
    <main
      className="page-x"
      style={{
        minHeight: "100dvh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div style={{ width: "100%", maxWidth: 360 }}>
        <div className="kick">FreeReps</div>
        <h1 style={{ fontSize: 30, lineHeight: 1.05, marginTop: 8 }}>
          Sign in
        </h1>
        <div
          style={{
            borderTop: "2px solid var(--color-text)",
            marginTop: 18,
            paddingTop: 20,
          }}
        >
          {children}
        </div>
      </div>
    </main>
  );
}
