import { useSearchParams } from "react-router-dom";
import PageHeader from "../components/PageHeader";
import AlertsTab from "../components/settings/AlertsTab";
import FrontPageTab from "../components/settings/FrontPageTab";
import HevyTab from "../components/settings/HevyTab";
import IdentityTab from "../components/settings/IdentityTab";
import ImportTab from "../components/settings/ImportTab";
import IngestTab from "../components/settings/IngestTab";
import OpenGymTab from "../components/settings/OpenGymTab";
import OuraTab from "../components/settings/OuraTab";
import SourcesTab from "../components/settings/SourcesTab";
import WithingsTab from "../components/settings/WithingsTab";
import { useIsDesktop } from "../hooks/useMediaQuery";

/* Hevy, openGym and Import are absent from the design's five-tab rail, but all
   three drive working integrations, so they stay rather than being dropped along
   with the old layout. */
const TABS = [
  { id: "identity", label: "Identity", render: () => <IdentityTab /> },
  { id: "sources", label: "Sources", render: () => <SourcesTab /> },
  { id: "oura", label: "Oura", render: () => <OuraTab /> },
  { id: "withings", label: "Withings", render: () => <WithingsTab /> },
  { id: "hevy", label: "Hevy", render: () => <HevyTab /> },
  { id: "opengym", label: "openGym", render: () => <OpenGymTab /> },
  { id: "front-page", label: "Front page", render: () => <FrontPageTab /> },
  { id: "ingest", label: "Ingest", render: () => <IngestTab /> },
  { id: "import", label: "Import", render: () => <ImportTab /> },
  { id: "alerts", label: "Alerts", render: () => <AlertsTab /> },
] as const;

type TabID = (typeof TABS)[number]["id"];

const VALID = new Set<string>(TABS.map((t) => t.id));

export default function SettingsPage() {
  const isDesktop = useIsDesktop();
  const [params, setParams] = useSearchParams();
  const raw = params.get("tab");
  const active: TabID = raw && VALID.has(raw) ? (raw as TabID) : "identity";

  const setTab = (id: TabID) => {
    const p = new URLSearchParams(params);
    p.set("tab", id);
    setParams(p, { replace: true });
  };

  if (!isDesktop) {
    // One scrolling page rather than a rail: the phone has no room beside the
    // content, and the sections are short enough to read in sequence.
    return (
      <>
        <PageHeader kicker="FreeReps" title="Settings" />
        <div style={{ borderTop: "2px solid var(--color-text)" }}>
          {TABS.map((tab) => (
            <section
              key={tab.id}
              className="page-x"
              style={{
                paddingTop: 20,
                paddingBottom: 24,
                borderBottom: "2px solid var(--color-text)",
              }}
            >
              {tab.render()}
            </section>
          ))}
        </div>
      </>
    );
  }

  return (
    <>
      <PageHeader kicker="FreeReps" title="Settings" />

      <div
        style={{
          display: "flex",
          borderTop: "2px solid var(--color-text)",
          minHeight: 640,
        }}
      >
        <div className="rail">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              className="rail-item"
              aria-selected={tab.id === active}
              style={{ padding: "11px 20px" }}
              onClick={() => setTab(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div
          className="page-x"
          style={{
            flex: 1,
            minWidth: 0,
            maxWidth: 960,
            paddingTop: 26,
            paddingBottom: 40,
          }}
        >
          {TABS.find((t) => t.id === active)?.render()}
        </div>
      </div>
    </>
  );
}
