/**
 * The red unread-count pill shown on parent-OGS message surfaces (staff inbox,
 * staff student-detail card, parent messages list, parent child detail). One
 * component so the size, weight, 99+ cap, color, and a11y label stay identical
 * everywhere instead of drifting across four hand-copied spans.
 */
export function UnreadBadge({
  count,
  className,
}: Readonly<{ count: number; className?: string }>) {
  if (count <= 0) return null;
  return (
    <span
      className={`bg-moto-red inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-bold text-white ${className ?? ""}`}
      aria-label={`${count} ungelesene Nachrichten`}
    >
      {count > 99 ? "99+" : count}
    </span>
  );
}
