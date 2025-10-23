import type { CSSProperties } from "react";
import clsx from "clsx";
import type { StudentLocationStatus } from "~/lib/student-location-helpers";
import { getStudentLocationBadge } from "~/lib/student-location-helpers";

export interface LocationBadgeProps {
  locationStatus: StudentLocationStatus | null;
  className?: string;
}

/**
 * Unified LocationBadge component that displays student location
 * using structured LocationStatus data.
 *
 * Replaces legacy StatusBadge and ModernStatusBadge components.
 */
export function LocationBadge({
  locationStatus,
  className = "",
}: LocationBadgeProps) {
  const badgeConfig = getStudentLocationBadge(locationStatus);

  const style: CSSProperties = {
    backgroundColor: badgeConfig.colorToken,
  };

  if (badgeConfig.gradientToken) {
    style.backgroundImage = badgeConfig.gradientToken;
  }

  return (
    <span
      className={clsx(
        "inline-flex items-center rounded-full px-3 py-1.5 text-xs font-semibold transition-all duration-300",
        badgeConfig.textClass,
        className,
      )}
      style={style}
    >
      {badgeConfig.label}
    </span>
  );
}
