"use client";

import type { ReactNode } from "react";
import { ListGroupHeader } from "~/components/ui/list-group-header";
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

// Die Tönung bleibt: in der Kinderliste zeigt sie eine Klasse mit fehlenden
// Ankunftszeiten an, auch wenn die Gruppe zugeklappt ist.
const HEADER_BG: Record<GroupHeaderVariant, string> = {
  neutral: "bg-gray-50",
  warning: "bg-moto-orange-soft",
  success: "bg-moto-green-soft/40",
};

const BADGE_TONES: Record<GroupHeaderVariant, StatusBadgeTone> = {
  neutral: "gray",
  warning: "orange",
  success: "green",
};

/**
 * Kopf einer Gruppe in den Listen der Datenverwaltung (`GroupedList`), gebaut
 * aus `ListGroupHeader` (#6098): der Name mit Zähler klappt die Gruppe auf
 * und zu, die Sammelaktion steht am rechten Rand.
 */
export function GroupHeader({
  title,
  count,
  countSuffix,
  variant = "neutral",
  isOpen,
  onToggle,
  bulkAction,
}: GroupHeaderProps) {
  return (
    <ListGroupHeader
      // Der Zähler steht im Knopf, damit er zum Namen der Gruppe gehört
      // („Klasse 1a 13“), nicht nur zu ihrer Beschreibung.
      title={title}
      titleAddon={
        <StatusBadge
          label={countSuffix ? `${count} ${countSuffix}` : String(count)}
          tone={BADGE_TONES[variant]}
          compact
        />
      }
      className={cn("border-b border-gray-200", HEADER_BG[variant])}
      collapsible={{ expanded: isOpen, onToggle }}
      meta={bulkAction ?? undefined}
    />
  );
}
