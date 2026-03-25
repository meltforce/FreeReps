import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { saveMetricVisibility } from "../../api";
import { useAvailableMetrics, type MetricOption } from "../../hooks/useMetrics";

export default function MetricVisibilityTab() {
  const { options, groups, isLoading } = useAvailableMetrics();
  const queryClient = useQueryClient();
  const [local, setLocal] = useState<Record<string, boolean>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [initialized, setInitialized] = useState(false);

  // Initialize local state from server data once
  if (options.length > 0 && !initialized) {
    const init: Record<string, boolean> = {};
    for (const m of options) init[m.value] = m.visible;
    setLocal(init);
    setInitialized(true);
  }

  function toggle(metric: string) {
    setLocal((prev) => ({ ...prev, [metric]: !prev[metric] }));
  }

  function toggleCategory(metrics: MetricOption[]) {
    const allVisible = metrics.every((m) => local[m.value]);
    setLocal((prev) => {
      const next = { ...prev };
      for (const m of metrics) next[m.value] = !allVisible;
      return next;
    });
  }

  const handleSave = useCallback(async () => {
    setSaving(true);
    setError(null);
    try {
      await saveMetricVisibility(local);
      queryClient.invalidateQueries({ queryKey: ["available-metrics"] });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }, [local, queryClient]);

  if (isLoading) return <p className="text-zinc-500">Loading metrics...</p>;
  if (error) return <p className="text-red-400">{error}</p>;

  const changed = options.some((m) => local[m.value] !== m.visible);
  const visibleCount = Object.values(local).filter(Boolean).length;

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="text-sm font-medium text-zinc-300">Visible Metrics</h3>
          <p className="text-xs text-zinc-500 mt-0.5">
            {visibleCount} of {options.length} metrics visible on dashboard and dropdowns
          </p>
        </div>
        <button
          onClick={handleSave}
          disabled={saving || !changed}
          className="px-4 py-1.5 text-sm bg-cyan-600 hover:bg-cyan-500 disabled:opacity-40 text-white rounded-md transition-colors"
        >
          {saving ? "Saving..." : "Save"}
        </button>
      </div>

      <div className="space-y-1">
        {groups.map((group) => {
          const groupMetrics = group.metrics;
          return (
            <div key={group.label} className="bg-zinc-900 border border-zinc-800 rounded-lg overflow-hidden">
              <button
                onClick={() => toggleCategory(groupMetrics)}
                className="w-full flex items-center justify-between px-4 py-2.5 text-sm hover:bg-zinc-800/50 transition-colors"
              >
                <span className="font-medium text-zinc-300">{group.label}</span>
                <span className="text-xs text-zinc-500">
                  {groupMetrics.filter((m) => local[m.value]).length}/{groupMetrics.length}
                </span>
              </button>
              <div className="border-t border-zinc-800">
                {groupMetrics.map((m) => (
                  <label
                    key={m.value}
                    className="flex items-center gap-3 px-4 py-1.5 text-sm hover:bg-zinc-800/30 cursor-pointer transition-colors"
                  >
                    <input
                      type="checkbox"
                      checked={local[m.value] ?? false}
                      onChange={() => toggle(m.value)}
                      className="accent-cyan-500 rounded w-3.5 h-3.5"
                    />
                    <span className={local[m.value] ? "text-zinc-200" : "text-zinc-500"}>
                      {m.label}
                    </span>
                    <span className="text-zinc-600 text-xs ml-auto">{m.unit}</span>
                  </label>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
