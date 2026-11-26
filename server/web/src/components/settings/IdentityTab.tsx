import { useQuery } from "@tanstack/react-query";
import { fetchMe, fetchStats, fetchVersion } from "../../api";
import { useTheme, type ThemePreference } from "../../theme";
import { formatNumber } from "../../utils/format";
import BirthDateRow from "./BirthDateRow";
import MaxHeartRateRow from "./MaxHeartRateRow";
import { MONO, Row, TabHeader } from "./parts";

const THEMES: { value: ThemePreference; label: string }[] = [
  { value: "auto", label: "Auto" },
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
];

export default function IdentityTab() {
  const { theme, setTheme } = useTheme();
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const version = useQuery({ queryKey: ["version"], queryFn: fetchVersion });
  const stats = useQuery({ queryKey: ["stats"], queryFn: fetchStats });

  const method = me.data?.auth_method;

  const rows = stats.data
    ? `${formatNumber(stats.data.total_metric_rows)} metric rows · ${formatNumber(
        stats.data.total_workouts,
      )} workouts · ${formatNumber(stats.data.total_sleep_nights)} nights`
    : "—";

  return (
    <>
      <TabHeader title="Identity">
        {method === "session"
          ? "Signed in through application login. Sessions and access tokens are under Account."
          : method === "dev"
            ? "Requests run as the local development user; no login is configured."
            : "This server holds one person's data. Identity comes from the reverse proxy, so there is nothing to log into."}
      </TabHeader>

      <Row label="Display name">
        {me.data?.display_name || me.data?.login || "—"}
      </Row>
      <Row label="User ID">
        <span style={MONO}>{me.data?.login ?? "—"}</span>
      </Row>
      <Row label="Provided by">
        {method === "session"
          ? "Application login"
          : method === "dev"
            ? "Local development user"
            : me.data?.tailnet
              ? `Tailscale · ${me.data.tailnet}`
              : "Tailscale reverse proxy header"}
      </Row>
      <Row label="Server version">
        <span style={MONO}>{version.data?.version ?? "—"}</span>
      </Row>
      <Row label="Database">TimescaleDB · {rows}</Row>

      <BirthDateRow />
      <MaxHeartRateRow />

      <Row label="Appearance">
        <span className="seg">
          {THEMES.map((t) => (
            <label key={t.value} className="seg-opt">
              <input
                type="radio"
                name="theme"
                value={t.value}
                checked={theme === t.value}
                onChange={() => setTheme(t.value)}
              />
              {t.label}
            </label>
          ))}
        </span>
        <p
          style={{
            font: "400 12px var(--font-body)",
            color: "var(--color-neutral-600)",
            margin: "8px 0 0",
          }}
        >
          Auto follows the system setting. The choice is stored in this browser.
        </p>
      </Row>
    </>
  );
}
