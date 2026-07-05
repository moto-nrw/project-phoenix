"use client";

import * as React from "react";

import { usePresenceMode } from "~/lib/tenant-context";
import type {
  DisplayMode,
  StudentLocationContext,
} from "@/lib/location-helper";
import { LocationBadge } from "./location-badge";
import { PresenceBadge } from "./presence-badge";

/**
 * StudentPresenceBadge — mode-aware wrapper.
 *
 * Picks between the detailed `LocationBadge` (rooms, activities, seit-time)
 * and the simplified `PresenceBadge` (Anwesend / Schulhof / Abwesend) based
 * on the tenant's current `presenceMode`. Pass the same props you would to
 * LocationBadge; binary-mode tenants silently ignore `displayMode`,
 * `userGroups`, `groupRooms`, etc.
 *
 * Use this component as the default in student lists, cards, dashboards —
 * anywhere a tenant might flip between the two modes without the page
 * wanting to change.
 */
interface StudentPresenceBadgeProps {
  readonly student: StudentLocationContext;
  /** Used by LocationBadge in detailed mode; ignored in binary mode. */
  readonly displayMode: DisplayMode;
  readonly userGroups?: string[];
  readonly groupRooms?: string[];
  readonly supervisedRooms?: string[];
  readonly isGroupRoom?: boolean;
  readonly variant?: "simple" | "modern";
  readonly size?: "sm" | "md" | "lg";
  readonly showLocationSince?: boolean;
}

export function StudentPresenceBadge(props: StudentPresenceBadgeProps) {
  const mode = usePresenceMode();

  if (mode === "binary") {
    return (
      <PresenceBadge
        student={props.student}
        variant={props.variant}
        size={props.size}
        showLocationSince={props.showLocationSince}
      />
    );
  }

  return <LocationBadge {...props} />;
}
