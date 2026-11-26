// Decisions of the login flow, kept free of React and of api.ts.

export interface AuthMethod {
  type: "oidc" | "password";
  display_name?: string;
  start_url?: string;
}

/** The /login URL that returns to `path` (path + query) after the login. */
export function loginURL(path: string): string {
  if (!path || path === "/" || path.startsWith("/login")) return "/login";
  return `/login?next=${encodeURIComponent(path)}`;
}

/** next from the query, restricted to a local path. */
export function safeNext(search: string): string {
  const next = new URLSearchParams(search).get("next") ?? "";
  if (!next.startsWith("/") || next.startsWith("//") || next.includes("\\")) {
    return "/";
  }
  return next;
}

/**
 * The provider URL /login hands over to at once: only when OIDC is the only
 * method, and never after an error (a refusal would loop) or a sign-out (the
 * provider signs a known browser in again without asking, which would undo it).
 */
export function autoStartURL(
  methods: AuthMethod[],
  search: string,
): string | null {
  const q = new URLSearchParams(search);
  if (q.has("error") || q.has("signed_out")) return null;
  if (
    methods.length !== 1 ||
    methods[0].type !== "oidc" ||
    !methods[0].start_url
  ) {
    return null;
  }
  return `${methods[0].start_url}?next=${encodeURIComponent(safeNext(search))}`;
}

/** Only a session can be ended; Tailscale WhoIs signs the next request in again. */
export function canSignOut(authMethod: string | undefined): boolean {
  return authMethod === "session";
}

/** Ends the session and lands on /login?signed_out=1. False when the server refused. */
export async function signOut(): Promise<boolean> {
  const res = await fetch("/auth/logout", { method: "POST" });
  if (!res.ok) return false;
  window.location.assign("/login?signed_out=1");
  return true;
}

/** Text for the ?error= codes of /auth/* and for a failed password login. */
const LOGIN_ERRORS: Record<string, string> = {
  no_account: "No access is set up for this account.",
  oidc_denied: "Sign-in was cancelled at the provider.",
  oidc_failed: "Sign-in at the provider failed.",
  oidc_unavailable: "The sign-in provider is unreachable.",
  invalid_state: "The sign-in expired. Please try again.",
  identity_in_use: "This login already belongs to another account.",
  login_failed: "Sign-in failed.",
  invalid_credentials: "Wrong username or password.",
  rate_limited: "Too many failed attempts. Please try again later.",
};

export function loginErrorText(code: string): string {
  return LOGIN_ERRORS[code] ?? LOGIN_ERRORS.login_failed;
}
