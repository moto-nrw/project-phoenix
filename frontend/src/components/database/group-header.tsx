"use client";

import { ChevronDown, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

export type GroupHeaderVariant = "neutral" | "warning" | "success";

interface GroupHeaderProps {
  title: string;
  count: number;
  countSuffix?: string;
  variant?: GroupHeaderVariant;
  isOpen: boolean;
  onToggle: () => void;
  bulkAction?: ReactNode;
}

const HEADER_BG: Record<GroupHeaderVariant, string> = {
  neutral: "bg-gray-50",
  warning: "bg-[#FFF8ED]",
  success: "bg-moto-green-soft/40",
};

const CHEVRON_COLOR: Record<GroupHeaderVariant, string> = {
  neutral: "text-gray-500",
  warning: "text-moto-orange",
  success: "text-moto-green",
};

const BADGE_STYLES: Record<GroupHeaderVariant, string> = {
  neutral: "bg-gray-200 text-gray-700",
  warning: "bg-[#FFE8D0] text-moto-orange",
  success: "bg-moto-green-soft text-moto-green",
};

export function GroupHeader({
  title,
  count,
  countSuffix,
  variant = "neutral",
  isOpen,
  onToggle,
  bulkAction,
}: GroupHeaderProps) {
  const Chevron = isOpen ? ChevronDown : ChevronRight;

  return (
    <div
      className={cn(
        "flex w-full items-center gap-2 border-b border-gray-200 px-4 py-3",
        HEADER_BG[variant],
      )}
    >
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={isOpen}
        className="flex flex-1 items-center gap-2 text-left"
      >
        <Chevron className={cn("h-4 w-4", CHEVRON_COLOR[variant])} />
        <span className="text-sm font-bold text-gray-900">{title}</span>
        <span
          className={cn(
            "rounded-full px-2 py-0.5 text-xs font-semibold",
            BADGE_STYLES[variant],
          )}
        >
          {count}
          {countSuffix ? ` ${countSuffix}` : ""}
        </span>
      </button>
      {bulkAction ? <div className="shrink-0">{bulkAction}</div> : null}
    </div>
  );
}
