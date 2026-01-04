// BadgeDisplay component - extracted to reduce complexity in PageHeaderWithSearch
"use client";

import type React from "react";

interface BadgeDisplayProps {
  readonly count: number | string;
  readonly label?: string;
  readonly icon?: React.ReactNode;
  readonly showLabel?: boolean;
  readonly size?: "sm" | "md";
}

/**
 * Badge display component for counts with optional label and icon
 */
export function BadgeDisplay({
  count,
  label,
  icon,
  showLabel = true,
  size = "md",
}: BadgeDisplayProps) {
  const paddingClass =
    size === "sm" ? "px-2 py-1.5 gap-1.5" : "px-3 py-1.5 gap-2";

  return (
    <div
      className={`flex items-center rounded-full border border-gray-100 bg-gray-50 ${paddingClass}`}
    >
      {icon && <span className="text-gray-500">{icon}</span>}
      <span className="text-sm font-semibold text-gray-900">{count}</span>
      {showLabel && label && (
        <span className="hidden text-xs text-gray-500 md:inline">{label}</span>
      )}
    </div>
  );
}

/**
 * Compact badge for mobile views (no label shown)
 */
export function BadgeDisplayCompact({
  count,
  icon,
}: Readonly<{ count: number | string; icon?: React.ReactNode }>) {
  return (
    <div className="flex items-center gap-1.5 rounded-full border border-gray-100 bg-gray-50 px-2 py-1.5">
      {icon && <span className="text-gray-500">{icon}</span>}
      <span className="text-sm font-semibold text-gray-900">{count}</span>
    </div>
  );
}
