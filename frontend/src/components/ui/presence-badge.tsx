import { resolvePresence, type PresenceStudent } from "@/lib/presence-status";
import {
  PresencePill,
  type PresencePillSize,
  type PresencePillVariant,
} from "./presence-pill";

/**
 * PresenceBadge — simplified binary-mode counterpart to LocationBadge.
 *
 * Three location states only: Anwesend (green), Schulhof (orange) and absent
 * (gray Zuhause; Schule for an expected child before the first check-in,
 * #3260). No room details, no "Unterwegs", no display-mode switching. Day
 * status and today's plan follow the same presence status table as
 * LocationBadge (`resolvePresence`).
 *
 * Consumers should usually import {@link StudentPresenceBadge} instead —
 * that wrapper routes detailed-mode tenants to LocationBadge automatically
 * via `usePresenceMode()`. Use PresenceBadge directly only when you want
 * to force the binary visual regardless of tenant mode.
 */
interface PresenceBadgeProps {
  /** The child as loaded; today's plan is read from its planning fields. */
  readonly student: PresenceStudent;
  readonly variant?: PresencePillVariant;
  readonly size?: PresencePillSize;
  /** Show "seit XX:XX Uhr" under the badge. Default: false. */
  readonly showLocationSince?: boolean;
}

const BINARY_VIEW = { mode: "binary" } as const;

export function PresenceBadge({
  student,
  variant = "modern",
  size = "md",
  showLocationSince = false,
}: PresenceBadgeProps) {
  return (
    <PresencePill
      presence={resolvePresence(student, BINARY_VIEW)}
      size={size}
      variant={variant}
      showLocationSince={showLocationSince}
    />
  );
}
