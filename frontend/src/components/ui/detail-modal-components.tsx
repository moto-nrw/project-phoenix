"use client";

import type { ReactNode } from "react";
import { Check, X } from "lucide-react";
import {
  BriefcaseIcon,
  BuildingsIcon,
  BusIcon,
  FileTextIcon,
  HeartIcon,
  HouseIcon,
  LockKeyIcon,
  NotePencilIcon,
  UserIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
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
}: DataFieldProps) {
  return (
    <div className={fullWidth ? "col-span-1 sm:col-span-2" : ""}>
      <dt className="flex items-center gap-1.5 text-xs text-gray-500">
        {icon ? (
          <span aria-hidden="true" className="shrink-0 text-gray-400">
            {icon}
          </span>
        ) : null}
        {label}
      </dt>
      <dd
        className={`mt-0.5 ${mono ? "font-mono text-xs break-all text-gray-600 md:text-sm" : "text-sm font-medium break-words text-gray-900"}`}
      >
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

// =============================================================================
// InfoSection - Shared component for information section containers
// =============================================================================

type AccentColor =
  "blue" | "orange" | "indigo" | "purple" | "red" | "amber" | "green" | "gray";

interface InfoSectionProps {
  readonly title: string;
  readonly icon: ReactNode;
  readonly children: ReactNode;
  /**
   * Historic accent selector. Section surfaces are neutral now (no colored
   * backgrounds behind icons), so this only keeps existing call sites
   * compiling unchanged; it no longer drives any color. Icon color comes
   * from the icon itself (MotoDuotoneIcon sets its own tone).
   */
  readonly accentColor?: AccentColor;
}

const bgColorClasses: Record<AccentColor, string> = {
  blue: "bg-gray-50",
  orange: "bg-gray-50",
  indigo: "bg-gray-50",
  purple: "bg-gray-50",
  red: "bg-gray-50",
  amber: "bg-gray-50",
  green: "bg-gray-50",
  gray: "bg-gray-50",
};

/**
 * Container component for information sections in detail modals.
 * Neutral surface (no accent-tinted backgrounds); the icon supplies its own
 * color (typically a MotoDuotoneIcon/MotoConceptIcon).
 */
export function InfoSection({
  title,
  icon,
  children,
  accentColor = "gray",
}: InfoSectionProps) {
  return (
    <div
      className={`rounded-xl border border-gray-100 ${bgColorClasses[accentColor]} p-3 md:p-4`}
    >
      <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
        <span className="h-3.5 w-3.5 md:h-4 md:w-4">{icon}</span>
        {title}
      </h3>
      {children}
    </div>
  );
}

// =============================================================================
// DataGrid - Shared component for grid layout of data fields
// =============================================================================

interface DataGridProps {
  readonly children: ReactNode;
}

/**
 * Grid layout for DataField components.
 * Provides responsive 1-2 column layout.
 */
export function DataGrid({ children }: DataGridProps) {
  return (
    <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
      {children}
    </dl>
  );
}

// =============================================================================
// InfoText - Shared component for text-only info sections
// =============================================================================

interface InfoTextProps {
  readonly children: ReactNode;
}

/**
 * Text content for info sections that don't use a data grid.
 */
export function InfoText({ children }: InfoTextProps) {
  return (
    <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
      {children}
    </p>
  );
}

// =============================================================================
// Common Icons - Shared SVG icons for detail modals
// =============================================================================

export const DetailIcons = {
  // Domain-flavored glyphs: self-colored via MotoDuotoneIcon, independent of
  // any wrapping accentColor (see InfoSection above).
  person: <MotoDuotoneIcon icon={UserIcon} tone="blue" size="100%" />,
  group: <MotoDuotoneIcon icon={UsersThreeIcon} tone="greenDeep" size="100%" />,
  heart: <MotoDuotoneIcon icon={HeartIcon} tone="coral" size="100%" />,
  notes: <MotoDuotoneIcon icon={NotePencilIcon} tone="teal" size="100%" />,
  document: <MotoDuotoneIcon icon={FileTextIcon} tone="neutral" size="100%" />,
  home: <MotoDuotoneIcon icon={HouseIcon} tone="neutral" size="100%" />,
  lock: <MotoDuotoneIcon icon={LockKeyIcon} tone="purple" size="100%" />,
  building: <MotoDuotoneIcon icon={BuildingsIcon} tone="petrol" size="100%" />,
  briefcase: (
    <MotoDuotoneIcon icon={BriefcaseIcon} tone="neutral" size="100%" />
  ),
  bus: <MotoDuotoneIcon icon={BusIcon} tone="cyan" size="100%" />,
  // Generic UI feedback: plain Lucide icons inheriting currentColor, so
  // callers that tint them via a wrapping text-color class keep working.
  check: <Check className="h-full w-full" strokeWidth={2} />,
  x: <X className="h-full w-full" strokeWidth={2} />,
} as const;
