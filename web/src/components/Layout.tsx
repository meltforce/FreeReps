import { ReactNode } from "react";
import { NavLink } from "react-router-dom";

const NAV_ITEMS = [
  { to: "/", label: "Dashboard" },
  { to: "/sleep", label: "Sleep" },
  { to: "/workouts", label: "Workouts" },
  { to: "/metrics", label: "Metrics" },
  { to: "/correlations", label: "Correlations" },
];

export default function Layout({ children }: { children: ReactNode }) {
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
        </div>
      </header>
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </div>
  );
}
