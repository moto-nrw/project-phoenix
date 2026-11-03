import type { ReactNode } from "react";

import { Skeleton } from "~/components/ui/skeleton";

// =============================================================================
// DataField - Shared component for dt/dd data display patterns
// =============================================================================

interface DataFieldProps {
  readonly label: string;
  readonly children: ReactNode;
  /** Whether this field spans full width (col-span-2) */
  readonly fullWidth?: boolean;
  /** Use monospace font for values like IDs */
  readonly mono?: boolean;
  /**
   * Optional leading glyph for the label. Decorative only — the label carries
   * the meaning, so the icon is hidden from assistive technology. Exists so a
   * detail panel that wants icon rows does not build its own field row again
   * (BAUARTEN-SPEC Bauart 2 Regel 2).
   */
  readonly icon?: ReactNode;
  /**
   * Label and value on one line, the value flush right. For statement-like
   * rows (a balance preview, a status list) that would otherwise be built as
   * a local `<dl>` with `justify-between` (BAUARTEN-SPEC Bauart 2 Regel 2,
   * #3119). Stacked (label above value) stays the default.
   */
  readonly inline?: boolean;
}

/**
 * Displays a label/value pair using semantic dt/dd elements.
 * Used consistently across all detail modals.
 */
export function DataField({
  label,
  children,
  fullWidth = false,
  mono = false,
  icon,
  inline = false,
}: DataFieldProps) {
  const span = fullWidth ? "col-span-1 sm:col-span-2" : "";
  const value = mono
    ? "font-mono text-xs break-all text-gray-600 md:text-sm"
    : "text-sm font-medium break-words text-gray-900";
  return (
    <div
      className={
        inline ? `flex items-center justify-between gap-4 ${span}` : span
      }
    >
      <dt className="flex items-center gap-1.5 text-xs text-gray-500">
        {icon ? (
          <span aria-hidden="true" className="shrink-0 text-gray-400">
            {icon}
          </span>
        ) : null}
        {label}
      </dt>
      <dd className={inline ? `text-right ${value}` : `mt-0.5 ${value}`}>
        {children}
      </dd>
    </div>
  );
}

/**
 * Loading twin of DataField, colocated so the two share one markup shape:
 * same wrapper, same dt/dd, bars instead of text. Compose inside a real
 * InfoSection/DataGrid — static chrome (titles, icons) renders real.
 */
export function DataFieldSkeleton({
  fullWidth = false,
}: Readonly<{ fullWidth?: boolean }>) {
  return (
    <div
      className={fullWidth ? "col-span-1 sm:col-span-2" : ""}
      aria-hidden="true"
    >
      <dt className="text-xs text-gray-500">
        <Skeleton className="h-3 w-20 rounded" />
      </dt>
      <dd className="mt-0.5 text-sm font-medium text-gray-900">
        <Skeleton className="h-4 w-2/3 rounded" />
      </dd>
    </div>
  );
}
