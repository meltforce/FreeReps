import { useSearchParams } from "react-router-dom";
import IdentityTab from "../components/settings/IdentityTab";
import ImportTab from "../components/settings/ImportTab";
import StatsTab from "../components/settings/StatsTab";
import ImportLogsTab from "../components/settings/ImportLogsTab";

const TABS = [
  { id: "identity", label: "Identity" },
  { id: "import", label: "Import" },
  { id: "stats", label: "Stats" },
  { id: "logs", label: "Import Logs" },
] as const;

type TabID = (typeof TABS)[number]["id"];

const VALID_TABS = new Set<string>(TABS.map((t) => t.id));

export default function SettingsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const paramTab = searchParams.get("tab");
  const tab: TabID = paramTab && VALID_TABS.has(paramTab) ? (paramTab as TabID) : "identity";

  function setTab(id: TabID) {
    setSearchParams({ tab: id }, { replace: true });
  }

  return (
    <div>
      <h2 className="text-2xl font-bold text-zinc-100 mb-4">Settings</h2>

      <nav className="flex gap-1 overflow-x-auto scrollbar-none mb-6 border-b border-zinc-800 pb-2">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors ${
              tab === t.id
                ? "bg-cyan-600 text-white"
                : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
            }`}
          >
            {t.label}
          </button>
        ))}
      </nav>

      <div>
        {tab === "identity" && <IdentityTab />}
        {tab === "import" && <ImportTab />}
        {tab === "stats" && <StatsTab />}
        {tab === "logs" && <ImportLogsTab />}
      </div>
    </div>
  );
}
