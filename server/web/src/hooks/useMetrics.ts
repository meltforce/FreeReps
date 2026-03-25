import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { fetchAvailableMetrics, type MetricMeta } from "../api";

function fallbackLabel(m: MetricMeta): string {
  if (m.display_label) return m.display_label;
  return m.metric_name
    .replace(/^oura_/, "")
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export interface MetricOption {
  value: string;
  label: string;
  unit: string;
  category: string;
  isCumulative: boolean;
  multiplier: number;
}

export interface MetricGroup {
  label: string;
  metrics: MetricOption[];
}

export function useAvailableMetrics() {
  const query = useQuery({
    queryKey: ["available-metrics"],
    queryFn: fetchAvailableMetrics,
    staleTime: 5 * 60 * 1000,
  });

  const options: MetricOption[] = useMemo(
    () =>
      (query.data ?? []).map((m) => ({
        value: m.metric_name,
        label: fallbackLabel(m),
        unit: m.display_unit,
        category: m.category,
        isCumulative: m.is_cumulative,
        multiplier: m.display_multiplier,
      })),
    [query.data],
  );

  const groups: MetricGroup[] = useMemo(() => {
    const byCategory = new Map<string, MetricOption[]>();
    for (const m of options) {
      const list = byCategory.get(m.category) ?? [];
      list.push(m);
      byCategory.set(m.category, list);
    }
    // Sort categories in a readable order
    const order = [
      "cardiovascular", "sleep", "body", "fitness", "activity",
      "oura", "hearing", "respiratory", "nutrition", "lab", "other",
    ];
    return order
      .filter((cat) => byCategory.has(cat))
      .map((cat) => ({
        label: cat.charAt(0).toUpperCase() + cat.slice(1),
        metrics: byCategory.get(cat)!,
      }));
  }, [options]);

  const lookup = useMemo(() => {
    const map = new Map<string, MetricOption>();
    for (const m of options) map.set(m.value, m);
    return map;
  }, [options]);

  return { ...query, options, groups, lookup };
}
