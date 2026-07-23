// Generic label+color status pill following the DataTableStatusBadge recipe
// (gray-50 pill, colored dot, label tinted via inline style). Use it whenever
// a status/type chip's color is data-driven from LOCATION_COLORS instead of
// hand-rolling tinted Tailwind pills.
export function StatusDotBadge({
  label,
  color,
}: {
  readonly label: string;
  readonly color: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span style={{ color }}>{label}</span>
    </span>
  );
}
