import { useCallback, useEffect, useState } from "react";
import {
  fetchSourcePriority,
  saveSourcePriority,
  deleteSourcePriority,
  type SourcePriorityConfig,
} from "../../api";

function sourceLabel(s: string): string {
  return s === "" ? "(HealthKit / HAE)" : s;
}

interface SortableListProps {
  sources: string[];
  onChange: (sources: string[]) => void;
}

function SortableList({ sources, onChange }: SortableListProps) {
  function move(idx: number, dir: -1 | 1) {
    const next = [...sources];
    const target = idx + dir;
    if (target < 0 || target >= next.length) return;
    [next[idx], next[target]] = [next[target], next[idx]];
    onChange(next);
  }

  return (
    <div className="space-y-1">
      {sources.map((src, i) => (
        <div key={src} className="flex items-center gap-2 bg-zinc-800 rounded px-3 py-1.5 text-sm">
          <span className="text-zinc-500 font-mono w-4">{i + 1}</span>
          <span className="text-zinc-200 flex-1">{sourceLabel(src)}</span>
          <button
            onClick={() => move(i, -1)}
            disabled={i === 0}
            className="text-zinc-500 hover:text-zinc-200 disabled:opacity-30 text-xs"
          >
            ▲
          </button>
          <button
            onClick={() => move(i, 1)}
            disabled={i === sources.length - 1}
            className="text-zinc-500 hover:text-zinc-200 disabled:opacity-30 text-xs"
          >
            ▼
          </button>
        </div>
      ))}
    </div>
  );
}

interface CategoryOverrideProps {
  category: string;
  currentSources: string[] | null;
  allSources: string[];
  onSave: (sources: string[]) => void;
  onDelete: () => void;
}

function CategoryOverride({ category, currentSources, allSources, onSave, onDelete }: CategoryOverrideProps) {
  const [expanded, setExpanded] = useState(currentSources !== null);
  const [localSources, setLocalSources] = useState<string[]>(currentSources ?? allSources);

  const hasOverride = currentSources !== null;

  function handleEnable() {
    setExpanded(true);
    setLocalSources(currentSources ?? allSources);
  }

  return (
    <div className="border border-zinc-800 rounded-lg p-3">
      <div className="flex items-center justify-between">
        <button
          onClick={() => hasOverride ? setExpanded(!expanded) : handleEnable()}
          className="text-sm text-zinc-300 hover:text-zinc-100 font-medium"
        >
          {category}
          {hasOverride && <span className="ml-2 text-xs text-cyan-400">custom</span>}
        </button>
        {hasOverride && (
          <button
            onClick={onDelete}
            className="text-xs text-zinc-500 hover:text-red-400"
          >
            Reset
          </button>
        )}
      </div>
      {expanded && (
        <div className="mt-2">
          <SortableList sources={localSources} onChange={setLocalSources} />
          <button
            onClick={() => onSave(localSources)}
            className="mt-2 px-3 py-1 text-xs bg-cyan-600 hover:bg-cyan-500 text-white rounded transition-colors"
          >
            Save
          </button>
        </div>
      )}
    </div>
  );
}

export default function SourcePriorityTab() {
  const [config, setConfig] = useState<SourcePriorityConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [defaultSources, setDefaultSources] = useState<string[]>([]);

  const load = useCallback(() => {
    setError(null);
    fetchSourcePriority()
      .then((c) => {
        setConfig(c);
        const defaultRule = c.rules.find((r) => r.category === "_default");
        setDefaultSources(defaultRule?.sources ?? c.default ?? c.sources);
      })
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (error) return <p className="text-red-400">{error}</p>;
  if (!config) return <p className="text-zinc-500">Loading...</p>;

  if (config.sources.length < 2) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-6">
        <p className="text-zinc-400">
          Source priority requires data from at least two sources. Currently only{" "}
          {config.sources.length === 0 ? "no sources" : sourceLabel(config.sources[0])} found.
        </p>
      </div>
    );
  }

  async function handleSaveDefault() {
    try {
      await saveSourcePriority("_default", defaultSources);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    }
  }

  async function handleSaveCategory(category: string, sources: string[]) {
    try {
      await saveSourcePriority(category, sources);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    }
  }

  async function handleDeleteCategory(category: string) {
    try {
      await deleteSourcePriority(category);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    }
  }

  function getCategoryOverride(category: string): string[] | null {
    const rule = config!.rules.find((r) => r.category === category);
    return rule?.sources ?? null;
  }

  return (
    <div>
      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 mb-4">
        <h3 className="text-sm font-medium text-zinc-300 mb-1">Default Priority</h3>
        <p className="text-xs text-zinc-500 mb-3">
          When multiple sources report the same metric, the highest-priority source wins.
          Drag to reorder. #1 = highest priority.
        </p>
        <SortableList sources={defaultSources} onChange={setDefaultSources} />
        <button
          onClick={handleSaveDefault}
          className="mt-3 px-3 py-1.5 text-sm bg-cyan-600 hover:bg-cyan-500 text-white rounded-md transition-colors"
        >
          Save Default
        </button>
      </div>

      <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
        <h3 className="text-sm font-medium text-zinc-300 mb-1">Per-Category Overrides</h3>
        <p className="text-xs text-zinc-500 mb-3">
          Override the default priority for specific metric categories. Click a category to customize.
        </p>
        <div className="space-y-2">
          {config.categories.map((cat) => (
            <CategoryOverride
              key={cat}
              category={cat}
              currentSources={getCategoryOverride(cat)}
              allSources={defaultSources}
              onSave={(sources) => handleSaveCategory(cat, sources)}
              onDelete={() => handleDeleteCategory(cat)}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
