interface Props {
  value: string;
  onChange: (value: string) => void;
  options: string[];
}

export default function TimeRangeSelector({ value, onChange, options }: Props) {
  return (
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
  );
}
