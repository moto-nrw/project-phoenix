"use client";

import { ChevronDown, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
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
  warning: "bg-moto-orange-soft",
  success: "bg-moto-green-soft/40",
};

const CHEVRON_COLOR: Record<GroupHeaderVariant, string> = {
  neutral: "text-gray-500",
  warning: "text-moto-orange",
  success: "text-moto-green",
};

const BADGE_TONES: Record<GroupHeaderVariant, StatusBadgeTone> = {
  neutral: "gray",
  warning: "orange",
  success: "green",
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
        <StatusBadge
          label={countSuffix ? `${count} ${countSuffix}` : String(count)}
          tone={BADGE_TONES[variant]}
          compact
        />
      </button>
      {bulkAction ? <div className="shrink-0">{bulkAction}</div> : null}
    </div>
  );
}
