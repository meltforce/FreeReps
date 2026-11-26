import { useQuery } from "@tanstack/react-query";
import { Link, NavLink } from "react-router-dom";
import type { ReactNode } from "react";
import { fetchMe } from "../api";
import { canSignOut, signOut } from "../auth";
import { useIsDesktop } from "../hooks/useMediaQuery";

const NAV_ITEMS = [
  { to: "/", label: "Today", end: true },
  { to: "/sleep", label: "Sleep" },
  { to: "/workouts", label: "Workouts" },
  { to: "/metrics", label: "Metrics" },
  { to: "/correlations", label: "Correlations" },
  { to: "/trends", label: "Trends" },
];

/* Metrics and Correlations are absent: they need width the phone does not have,
   so below 768px they stay reachable by URL only. Settings sits under More. */
const TAB_ITEMS = [
  { to: "/", label: "Today", end: true },
  { to: "/sleep", label: "Sleep" },
  { to: "/workouts", label: "Train" },
  { to: "/trends", label: "Trends" },
  { to: "/settings", label: "More" },
];

export default function Layout({ children }: { children: ReactNode }) {
  const isDesktop = useIsDesktop();
  const { data: user } = useQuery({
    queryKey: ["me"],
    queryFn: fetchMe,
    staleTime: 5 * 60 * 1000,
  });

  return (
    <div className="min-h-screen flex flex-col">
      {isDesktop ? (
        <nav className="nav">
          {/* A plain Link: the brand points at Today but must not carry the
              active mark, which belongs to the Today nav item. */}
          <Link to="/" className="nav-brand">
            FreeReps
          </Link>
          {NAV_ITEMS.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end}>
              {item.label}
            </NavLink>
          ))}
          <NavLink
            to="/settings"
            className="ml-auto"
            style={{ color: "var(--color-neutral-600)" }}
          >
            {user?.display_name || user?.login || "Settings"}
          </NavLink>
          {canSignOut(user?.auth_method) ? (
            <button
              type="button"
              className="btn btn-ghost"
              style={{ fontSize: 13, fontWeight: 500, fontFamily: "inherit" }}
              onClick={() => void signOut()}
            >
              Sign out
            </button>
          ) : null}
        </nav>
      ) : null}

      <main className="flex-1 flex flex-col">{children}</main>

      {!isDesktop ? (
        <>
          {/* Reserves the tab bar's height so the last row is not covered. */}
          <div aria-hidden className="h-[76px]" />
          <nav className="tabs">
            {TAB_ITEMS.map((item) => (
              <NavLink key={item.to} to={item.to} end={item.end}>
                {item.label}
              </NavLink>
            ))}
          </nav>
        </>
      ) : null}
    </div>
  );
}
