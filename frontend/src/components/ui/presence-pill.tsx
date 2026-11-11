import type { ResolvedPresence } from "~/lib/presence-status";
import { PresenceHint } from "./presence-hint";
import { StatusColorBadge } from "./status-color-badge";

export type PresencePillSize = "sm" | "md" | "lg";
export type PresencePillVariant = "simple" | "modern";

/** The kit pill shape for the badge's own size and variant props. */
function pillShape(
  size: PresencePillSize,
  variant: PresencePillVariant,
): { size?: "xs"; compact?: boolean } {
  if (size === "sm") return { size: "xs" };
  return size === "md" && variant === "simple" ? { compact: true } : {};
}

interface PresencePillProps {
  readonly presence: ResolvedPresence;
  readonly size: PresencePillSize;
  readonly variant: PresencePillVariant;
  readonly showLocationSince: boolean;
}

/**
 * Renders a resolved presence: the pill, „seit … Uhr“ and at most one hint
 * below it. Shared by `LocationBadge` and `PresenceBadge`; it decides nothing.
 */
export function PresencePill({
  presence,
  size,
  variant,
  showLocationSince,
}: PresencePillProps) {
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
