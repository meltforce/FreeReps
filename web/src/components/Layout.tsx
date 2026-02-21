import { ReactNode, useEffect, useState } from "react";
import { NavLink } from "react-router-dom";
import { fetchMe, type UserInfo } from "../api";

const NAV_ITEMS = [
  { to: "/", label: "Dashboard" },
  { to: "/sleep", label: "Sleep" },
  { to: "/workouts", label: "Workouts" },
  { to: "/metrics", label: "Metrics" },
  { to: "/correlations", label: "Correlations" },
  { to: "/trends", label: "Trends" },
];

export default function Layout({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null);

  useEffect(() => {
    fetchMe().then(setUser).catch(() => {});
  }, []);

  return (
    <div className="min-h-screen bg-zinc-950">
      <header className="border-b border-zinc-800 bg-zinc-900/50 backdrop-blur sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-3">
          <NavLink to="/" className="flex items-center gap-2 shrink-0">
            <h1 className="text-xl font-bold text-zinc-100 tracking-tight">
              FreeReps
            </h1>
            <span className="text-xs text-zinc-500 font-mono">v0.2</span>
          </NavLink>

          <nav className="ml-6 flex gap-1 overflow-x-auto scrollbar-none">
            {NAV_ITEMS.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === "/"}
                className={({ isActive }) =>
                  `px-3 py-1.5 rounded-md text-sm font-medium whitespace-nowrap transition-colors ${
                    isActive
                      ? "bg-cyan-600 text-white"
                      : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
                  }`
                }
              >
                {item.label}
              </NavLink>
            ))}
          </nav>

          {user && (
            <NavLink
              to="/settings"
              className={({ isActive }) =>
                `ml-auto flex items-center gap-1.5 text-sm font-medium shrink-0 transition-colors ${
                  isActive
                    ? "text-cyan-400"
                    : "text-zinc-400 hover:text-zinc-200"
                }`
              }
            >
              <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path d="M10 8a3 3 0 100-6 3 3 0 000 6zM3.465 14.493a1.23 1.23 0 00.41 1.412A9.957 9.957 0 0010 18c2.31 0 4.438-.784 6.131-2.1.43-.333.604-.903.408-1.41a7.002 7.002 0 00-13.074.003z" />
              </svg>
              {user.display_name || user.login}
            </NavLink>
          )}
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </div>
  );
}
