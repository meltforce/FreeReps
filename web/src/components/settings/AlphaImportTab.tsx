import { useRef, useState } from "react";
import { uploadAlphaCSV } from "../../api";

export default function AlphaImportTab() {
  const fileRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [result, setResult] = useState<{
    sets_received: number;
    sets_inserted: number;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleUpload() {
    const file = fileRef.current?.files?.[0];
    if (!file) return;

    setUploading(true);
    setError(null);
    setResult(null);

    try {
      const res = await uploadAlphaCSV(file);
      setResult(res);
      if (fileRef.current) fileRef.current.value = "";
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setUploading(false);
    }
  }

  return (
    <div className="space-y-6">
      <p className="text-sm text-zinc-400">
        Upload an Alpha Progression CSV export to import detailed set/rep/weight
        data for your strength training workouts.
      </p>

      <div className="flex items-end gap-3">
        <div className="flex-1">
          <label className="block text-xs text-zinc-500 uppercase mb-1">
            CSV File
          </label>
          <input
            ref={fileRef}
            type="file"
            accept=".csv,text/csv"
            disabled={uploading}
            className="w-full bg-zinc-900 border border-zinc-700 rounded-md px-3 py-2 text-sm text-zinc-100 file:bg-zinc-800 file:border-0 file:text-zinc-300 file:text-sm file:mr-3 file:px-3 file:py-1 file:rounded disabled:opacity-50"
          />
        </div>
        <button
          onClick={handleUpload}
          disabled={uploading}
          className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 disabled:bg-zinc-700 disabled:text-zinc-500 text-white text-sm font-medium rounded-md transition-colors shrink-0"
        >
          {uploading ? "Uploading..." : "Upload"}
        </button>
      </div>

      {error && <p className="text-sm text-red-400">{error}</p>}

      {result && (
        <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
          <p className="text-sm text-green-400 mb-2">Upload successful</p>
          <div className="flex gap-4 text-xs text-zinc-400">
            <span>Parsed: {result.sets_received} sets</span>
            <span>Imported: {result.sets_inserted} new</span>
            <span>Skipped: {result.sets_received - result.sets_inserted} duplicates</span>
          </div>
        </div>
      )}
    </div>
  );
}
