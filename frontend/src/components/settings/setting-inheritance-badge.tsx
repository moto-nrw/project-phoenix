"use client";

import type { Scope } from "~/lib/settings-helpers";
import { getScopeName, getScopeColor } from "~/lib/settings-helpers";

interface SettingInheritanceBadgeProps {
  scope: Scope;
  isOverridden: boolean;
}

export function SettingInheritanceBadge({
  scope,
  isOverridden,
}: SettingInheritanceBadgeProps) {
  const scopeName = getScopeName(scope);
  const colorClass = getScopeColor(scope);

  return (
    <div className="flex items-center gap-2">
      <span
        className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${colorClass}`}
      >
        {scopeName}
      </span>
      {isOverridden && (
        <span className="text-xs text-gray-500" title="Dieser Wert überschreibt den Standardwert">
          (überschrieben)
        </span>
      )}
    </div>
  );
}
