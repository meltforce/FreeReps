import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  deleteSourcePriority,
  fetchSourcePriority,
  saveSourcePriority,
} from "../../api";
import { formatNumber, formatTimeAgo } from "../../utils/format";
import { sourceLabelLong } from "../../utils/sourceLabel";
import { TabHeader } from "./parts";

const DEFAULT_CATEGORY = "_default";

/**
 * Surfaces the source-priority logic that already runs in the ingest path but
 * had no UI: when two devices report the same metric, the source higher in the
 * list wins.
 */
export default function SourcesTab() {
  const queryClient = useQueryClient();
  const config = useQuery({
    queryKey: ["source-priority"],
    queryFn: fetchSourcePriority,
  });
  const [order, setOrder] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!config.data) return;
    const rule = (config.data.rules ?? []).find((r) => r.category === DEFAULT_CATEGORY);
    setOrder(rule?.sources ?? config.data.default ?? config.data.sources ?? []);
  }, [config.data]);

  // Keyed by the source value itself, so the lookup cannot silently miss the
  // way the import-log job names did.
  const activityBySource = new Map(
    (config.data?.activity ?? []).map((a) => [a.source, a]),
  );

  function move(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= order.length) return;
    const next = [...order];
    [next[index], next[target]] = [next[target], next[index]];
    setOrder(next);
  }

  async function save() {
    setSaving(true);
    setError(null);
    try {
      await saveSourcePriority(DEFAULT_CATEGORY, order);
      queryClient.invalidateQueries({ queryKey: ["source-priority"] });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  const saved =
    (config.data?.rules ?? []).find((r) => r.category === DEFAULT_CATEGORY)?.sources ??
    config.data?.default ??
    [];
  const changed = order.join("|") !== saved.join("|");

  // Every rule that is not the default one. The ingest path honours these, so
  // hiding them would make the order above look absolute when it is not.
  const overrides = (config.data?.rules ?? []).filter(
    (r) => r.category !== DEFAULT_CATEGORY,
  );

  async function removeOverride(category: string) {
    setError(null);
    try {
      await deleteSourcePriority(category);
      queryClient.invalidateQueries({ queryKey: ["source-priority"] });
    } catch (e) {
      setError(e instanceof Error ? e.message : "Remove failed");
    }
  }

  return (
    <>
      <TabHeader title="Sources">
        When two devices report the same metric, the source higher in this list
        wins. Move an entry up to prefer it.
      </TabHeader>

      <div style={{ paddingTop: 4 }}>
        {order.map((src, i) => {
          const activity = activityBySource.get(src);
          return (
            <div
              key={src || "(healthkit)"}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 16,
                padding: "14px 0",
                borderBottom: "1px solid var(--color-neutral-300)",
              }}
            >
              <span className="kick num" style={{ width: 20, flex: "none" }}>
                {i + 1}
              </span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ font: "600 14px var(--font-body)" }}>
                  {sourceLabelLong(src)}
                </div>
                <div
                  style={{
                    font: "400 12px var(--font-body)",
                    color: "var(--color-neutral-600)",
                    marginTop: 2,
                  }}
                >
                  {i === 0
                    ? "Wins on conflict"
                    : i === 1
                      ? "Used when rank 1 has no reading"
                      : `Used when ranks 1–${i} have no reading`}
                </div>
              </div>
              <span
                className={`tag ${activity ? "tag-accent" : "tag-neutral"}`}
                style={{ flex: "none" }}
              >
                {activity ? "Delivering" : "No data"}
              </span>
              <span
                style={{
                  width: 190,
                  flex: "none",
                  textAlign: "right",
                  font: "400 12px var(--font-body)",
                  color: "var(--color-neutral-600)",
                }}
              >
                {activity
                  ? `${formatTimeAgo(activity.last_seen)} · ${formatNumber(activity.rows)} rows`
                  : "nothing stored"}
              </span>
              <span style={{ display: "flex", gap: 4, flex: "none" }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ fontSize: 11, padding: "4px 8px" }}
                  disabled={i === 0}
                  onClick={() => move(i, -1)}
                  aria-label={`Move ${sourceLabelLong(src)} up`}
                >
                  ↑
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  style={{ fontSize: 11, padding: "4px 8px" }}
                  disabled={i === order.length - 1}
                  onClick={() => move(i, 1)}
                  aria-label={`Move ${sourceLabelLong(src)} down`}
                >
                  ↓
                </button>
              </span>
            </div>
          );
        })}
      </div>

      {error ? (
        <p style={{ color: "var(--color-accent-700)", fontSize: 13, marginTop: 14 }}>
          {error}
        </p>
      ) : null}

      <div style={{ paddingTop: 20 }}>
        <button
          type="button"
          className="btn btn-primary"
          onClick={save}
          disabled={saving || !changed}
        >
          {saving ? "Saving…" : "Save order"}
        </button>
      </div>

      {overrides.length > 0 ? (
        <div style={{ paddingTop: 32 }}>
          <h3 style={{ fontSize: 15, fontWeight: 700 }}>Category exceptions</h3>
          <p
            style={{
              font: "400 12px/1.5 var(--font-body)",
              color: "var(--color-neutral-600)",
              margin: "6px 0 0",
              maxWidth: "62ch",
            }}
          >
            These categories ignore the order above and use their own. Without
            this list the order above would look like it governed everything.
          </p>
          <div
            style={{ borderTop: "2px solid var(--color-text)", marginTop: 12 }}
          >
            {overrides.map((rule) => (
              <div
                key={rule.category}
                style={{
                  display: "flex",
                  alignItems: "baseline",
                  gap: 16,
                  padding: "12px 0",
                  borderBottom: "1px solid var(--color-neutral-300)",
                }}
              >
                <span
                  className="kick"
                  style={{ width: 140, flex: "none", color: "var(--color-text)" }}
                >
                  {rule.category}
                </span>
                <span
                  style={{
                    flex: 1,
                    font: "400 13px var(--font-body)",
                    color: "var(--color-neutral-700)",
                  }}
                >
                  {rule.sources.map(sourceLabelLong).join(" → ")}
                </span>
                <button
                  type="button"
                  className="btn btn-ghost"
                  style={{ fontSize: 12, flex: "none" }}
                  onClick={() => removeOverride(rule.category)}
                >
                  Remove
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </>
  );
}
