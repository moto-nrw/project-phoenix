"use client";

import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "~/lib/utils";
import { Checkbox } from "~/components/ui/checkbox";

interface DatabaseListItemProps {
  title: string;
  subtitle: ReactNode;
  isSelected: boolean;
  onSelect: () => void;
  trailingAccessory?: ReactNode;
  selectionMode?: boolean;
  isChecked?: boolean;
  onToggleSelection?: () => void;
}

export function DatabaseListItem({
  title,
  subtitle,
  isSelected,
  onSelect,
  trailingAccessory,
  selectionMode = false,
  isChecked = false,
  onToggleSelection,
}: DatabaseListItemProps) {
  const content = (
    <>
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "truncate text-sm text-gray-900",
            isSelected || isChecked ? "font-semibold" : "font-medium",
          )}
        >
          {title}
        </div>
        <div className="truncate text-xs text-gray-500">{subtitle}</div>
      </div>
      {trailingAccessory}
    </>
  );

  if (selectionMode) {
    return (
      <label
        className={cn(
          "flex w-full cursor-pointer items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-left transition-colors hover:bg-gray-50",
          isChecked && "bg-moto-green-soft/60 hover:bg-moto-green-soft/70",
        )}
      >
        <Checkbox checked={isChecked} onChange={onToggleSelection} />
        {content}
      </label>
    );
  }

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
      {content}
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
