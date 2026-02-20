interface Props {
  value: string;
  onChange: (value: string) => void;
  options: string[];
  onPrev?: () => void;
  onNext?: () => void;
  canGoNext?: boolean;
  dateLabel?: string;
}

export default function TimeRangeSelector({
  value,
  onChange,
  options,
  onPrev,
  onNext,
  canGoNext = true,
  dateLabel,
}: Props) {
  return (
    <div className="flex items-center gap-2">
      {onPrev && (
        <button
          onClick={onPrev}
          className="px-2 py-1.5 rounded-md text-sm font-medium transition-colors
                     bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200"
          title="Previous period"
        >
          &lsaquo;
        </button>
      )}

      <div className="flex gap-1">
        {options.map((opt) => (
          <button
            key={opt}
            onClick={() => onChange(opt)}
            className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors
              ${
                value === opt
                  ? "bg-cyan-600 text-white"
                  : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200"
              }`}
          >
            {opt}
          </button>
        ))}
      </div>

      {onNext && (
        <button
          onClick={onNext}
          disabled={!canGoNext}
          className="px-2 py-1.5 rounded-md text-sm font-medium transition-colors
                     bg-zinc-800 text-zinc-400 hover:bg-zinc-700 hover:text-zinc-200
                     disabled:opacity-40 disabled:cursor-not-allowed"
          title="Next period"
        >
          &rsaquo;
        </button>
      )}

      {dateLabel && (
        <span className="text-xs text-zinc-500 ml-1 whitespace-nowrap">
          {dateLabel}
        </span>
      )}
    </div>
  );
}
