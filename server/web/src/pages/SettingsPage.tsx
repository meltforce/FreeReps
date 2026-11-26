import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { fetchMe, type UserInfo } from "../api";
import PageHeader from "../components/PageHeader";
import AccountTab from "../components/settings/AccountTab";
import AlertsTab from "../components/settings/AlertsTab";
import FrontPageTab from "../components/settings/FrontPageTab";
import HevyTab from "../components/settings/HevyTab";
import IdentityTab from "../components/settings/IdentityTab";
import ImportTab from "../components/settings/ImportTab";
import IngestTab from "../components/settings/IngestTab";
import OpenGymTab from "../components/settings/OpenGymTab";
import OuraTab from "../components/settings/OuraTab";
import SourcesTab from "../components/settings/SourcesTab";
import UsersTab from "../components/settings/UsersTab";
import WithingsTab from "../components/settings/WithingsTab";
import { useIsDesktop } from "../hooks/useMediaQuery";

/* Hevy, openGym and Import are absent from the design's five-tab rail, but all
   three drive working integrations, so they stay rather than being dropped along
   with the old layout. */
const TABS = [
  { id: "identity", label: "Identity", render: () => <IdentityTab /> },
  { id: "account", label: "Account", render: () => <AccountTab /> },
  { id: "users", label: "Users", render: () => <UsersTab /> },
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

/* Account and Users exist only with application login; Users only for an
   admin, because /admin/users answers 404 to everyone else. */
function visibleTabs(me: UserInfo | undefined) {
  return TABS.filter((t) => {
    if (t.id === "account") return me?.auth_enabled === true;
    if (t.id === "users") return me?.auth_enabled === true && me.is_admin;
    return true;
  });
}

export default function SettingsPage() {
  const isDesktop = useIsDesktop();
  const [params, setParams] = useSearchParams();
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe });
  const tabs = visibleTabs(me.data);
  const raw = params.get("tab");
  const active: TabID = tabs.find((t) => t.id === raw)?.id ?? "identity";

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
          {tabs.map((tab) => (
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
          {tabs.map((tab) => (
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
          {tabs.find((t) => t.id === active)?.render()}
        </div>
      </div>
    </>
  );
}
