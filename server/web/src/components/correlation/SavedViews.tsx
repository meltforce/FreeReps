import { useState, useEffect } from "react";

export interface CorrelationView {
  name: string;
  xMetric: string;
  yMetric: string;
  timeRange: string;
  mode: "scatter" | "overlay";
}

const STORAGE_KEY = "freereps-correlation-views";

function loadViews(): CorrelationView[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    return JSON.parse(raw);
  } catch {
    return [];
  }
}

function saveViews(views: CorrelationView[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(views));
}

interface Props {
  current: Omit<CorrelationView, "name">;
  onLoad: (view: CorrelationView) => void;
}

export default function SavedViews({ current, onLoad }: Props) {
  const [views, setViews] = useState<CorrelationView[]>([]);
  const [showSave, setShowSave] = useState(false);
  const [name, setName] = useState("");

  useEffect(() => {
    setViews(loadViews());
  }, []);

  const handleSave = () => {
    if (!name.trim()) return;
    const newView = { ...current, name: name.trim() };
    const updated = [...views.filter((v) => v.name !== newView.name), newView];
    setViews(updated);
    saveViews(updated);
    setName("");
    setShowSave(false);
  };

  const handleDelete = (viewName: string) => {
    const updated = views.filter((v) => v.name !== viewName);
    setViews(updated);
    saveViews(updated);
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <span className="text-xs text-zinc-500">Saved Views</span>
        <button
          onClick={() => setShowSave(!showSave)}
          className="text-xs text-cyan-500 hover:text-cyan-400"
        >
          {showSave ? "Cancel" : "+ Save Current"}
        </button>
      </div>

      {showSave && (
        <div className="flex gap-2">
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleSave()}
            placeholder="View name..."
            className="bg-zinc-800 border border-zinc-700 text-zinc-100 rounded-md px-2 py-1 text-sm flex-1
                       focus:outline-none focus:ring-1 focus:ring-cyan-500"
          />
          <button
            onClick={handleSave}
            className="px-3 py-1 bg-cyan-600 text-white rounded-md text-sm hover:bg-cyan-500"
          >
            Save
          </button>
        </div>
      )}

      {views.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {views.map((v) => (
            <div key={v.name} className="flex items-center gap-0.5">
              <button
                onClick={() => onLoad(v)}
                className="px-2 py-1 bg-zinc-800 text-zinc-300 rounded-l-md text-xs hover:bg-zinc-700"
              >
                {v.name}
              </button>
              <button
                onClick={() => handleDelete(v.name)}
                className="px-1.5 py-1 bg-zinc-800 text-zinc-500 rounded-r-md text-xs hover:bg-zinc-700 hover:text-red-400"
              >
                x
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
