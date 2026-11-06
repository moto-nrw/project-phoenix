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

interface DataGridProps {
  readonly children: ReactNode;
  /**
   * Column count from `sm` upwards; below that the grid stacks. `1` is a
   * plain list of fields (a balance preview), `4` a row of short figures
   * (import preview, key figures) that starts two-up on narrow screens.
   * Default `2` is the detail-view layout (BAUARTEN-SPEC Bauart 2 Regel 2).
   */
  readonly columns?: 1 | 2 | 4;
}

const gridColumnClasses: Record<
  NonNullable<DataGridProps["columns"]>,
  string
> = {
  1: "grid grid-cols-1 gap-y-2 md:gap-y-3",
  2: "grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3",
  4: "grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-4 md:gap-x-4 md:gap-y-3",
};

/** Grid layout for DataField components; see `columns`. */
export function DataGrid({ children, columns = 2 }: DataGridProps) {
  return <dl className={gridColumnClasses[columns]}>{children}</dl>;
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
        <Skeleton className="h-3 w-20" />
      </dt>
      <dd className="mt-0.5 text-sm font-medium text-gray-900">
        <Skeleton className="h-4 w-2/3" />
      </dd>
    </div>
  );
}
