interface Props {
  lines?: number;
  height?: string;
}

export default function LoadingSkeleton({
  lines = 3,
  height = "h-4",
}: Props) {
  return (
    <div className="space-y-3 animate-pulse">
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className={`bg-zinc-800 rounded ${height}`}
          style={{ width: `${100 - i * 15}%` }}
        />
      ))}
    </div>
  );
}
