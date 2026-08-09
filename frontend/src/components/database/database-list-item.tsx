"use client";

import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

interface DatabaseListItemProps {
  title: string;
  subtitle: ReactNode;
  isSelected: boolean;
  onSelect: () => void;
  trailingAccessory?: ReactNode;
}

export function DatabaseListItem({
  title,
  subtitle,
  isSelected,
  onSelect,
  trailingAccessory,
}: DatabaseListItemProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={isSelected ? "true" : undefined}
      className={cn(
        "flex w-full items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-left transition-colors hover:bg-gray-50",
        isSelected && "bg-moto-green-soft/60 hover:bg-moto-green-soft/70",
      )}
    >
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "truncate text-sm text-gray-900",
            isSelected ? "font-semibold" : "font-medium",
          )}
        >
          {title}
        </div>
        <div className="truncate text-xs text-gray-500">{subtitle}</div>
      </div>
      {trailingAccessory}
      <ChevronRight
        className={cn(
          "h-4 w-4 shrink-0",
          isSelected ? "text-moto-green" : "text-gray-400",
        )}
        aria-hidden
      />
    </button>
  );
}
