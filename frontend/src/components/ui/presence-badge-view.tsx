import type { ResolvedPresence } from "~/lib/presence-status";
import { PresenceHint } from "./presence-hint";
import { StatusColorBadge } from "./status-color-badge";

export type PresenceBadgeSize = "sm" | "md" | "lg";
export type PresenceBadgeVariant = "simple" | "modern";

/** The kit pill shape for the badge's own size and variant props. */
function pillShape(
  size: PresenceBadgeSize,
  variant: PresenceBadgeVariant,
): { size?: "xs"; compact?: boolean } {
  if (size === "sm") return { size: "xs" };
  return size === "md" && variant === "simple" ? { compact: true } : {};
}

interface PresenceBadgeViewProps {
  readonly presence: ResolvedPresence;
  readonly size: PresenceBadgeSize;
  readonly variant: PresenceBadgeVariant;
  readonly showLocationSince: boolean;
}

/**
 * Renders a resolved presence: the pill, „seit … Uhr“ and at most one hint
 * below it. Shared by `LocationBadge` and `PresenceBadge`; it decides nothing.
 */
export function PresenceBadgeView({
  presence,
  size,
  variant,
  showLocationSince,
}: PresenceBadgeViewProps) {
  const { pill, since, hint } = presence;
  return (
    <div className="flex max-w-full min-w-0 flex-col items-end">
      <StatusColorBadge
        {...pillShape(size, variant)}
        wrap
        label={pill.label}
        color={pill.color}
        data-location-status={pill.locationStatus}
        data-presence-state={pill.presenceState}
        title={pill.title}
      />
      {showLocationSince && since !== null && (
        <span className="mt-0.5 text-xs text-gray-500">seit {since} Uhr</span>
      )}
      {hint && (
        <PresenceHint status={hint.status} size={size} title={hint.title} />
      )}
    </div>
  );
}
