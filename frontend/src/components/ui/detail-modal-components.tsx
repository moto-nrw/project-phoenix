"use client";

import type { ReactNode } from "react";

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
}: DataFieldProps) {
  return (
    <div className={fullWidth ? "col-span-1 sm:col-span-2" : ""}>
      <dt className="text-xs text-gray-500">{label}</dt>
      <dd
        className={`mt-0.5 ${mono ? "font-mono text-xs break-all text-gray-600 md:text-sm" : "text-sm font-medium break-words text-gray-900"}`}
      >
        {children}
      </dd>
    </div>
  );
}

// =============================================================================
// InfoSection - Shared component for information section containers
// =============================================================================

type AccentColor =
  | "blue"
  | "orange"
  | "indigo"
  | "purple"
  | "red"
  | "amber"
  | "green"
  | "gray";

interface InfoSectionProps {
  readonly title: string;
  readonly icon: ReactNode;
  readonly children: ReactNode;
  /** Accent color for the section background and icon */
  readonly accentColor?: AccentColor;
}

const bgColorClasses: Record<AccentColor, string> = {
  blue: "bg-blue-50/30",
  orange: "bg-orange-50/30",
  indigo: "bg-indigo-50/30",
  purple: "bg-purple-50/30",
  red: "bg-red-50/30",
  amber: "bg-amber-50/30",
  green: "bg-green-50/30",
  gray: "bg-gray-50",
};

const iconColorClasses: Record<AccentColor, string> = {
  blue: "text-blue-600",
  orange: "text-orange-600",
  indigo: "text-indigo-600",
  purple: "text-purple-600",
  red: "text-red-600",
  amber: "text-amber-600",
  green: "text-green-600",
  gray: "text-gray-600",
};

/**
 * Container component for information sections in detail modals.
 * Provides consistent styling with accent colors and headers.
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
        <span
          className={`h-3.5 w-3.5 md:h-4 md:w-4 ${iconColorClasses[accentColor]}`}
        >
          {icon}
        </span>
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
  person: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
      />
    </svg>
  ),
  group: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
      />
    </svg>
  ),
  heart: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
      />
    </svg>
  ),
  notes: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
      />
    </svg>
  ),
  document: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
      />
    </svg>
  ),
  home: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
      />
    </svg>
  ),
  lock: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
      />
    </svg>
  ),
  building: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
      />
    </svg>
  ),
  briefcase: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  ),
  check: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 13l4 4L19 7"
      />
    </svg>
  ),
  x: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M6 18L18 6M6 6l12 12"
      />
    </svg>
  ),
  bus: (
    <svg
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      className="h-full w-full"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
      />
    </svg>
  ),
} as const;
