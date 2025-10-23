import type { CSSProperties } from "react";
import clsx from "clsx";
import type { StudentLocationStatus } from "~/lib/student-location-helpers";
import { getStudentLocationBadge } from "~/lib/student-location-helpers";

export interface LocationBadgeProps {
  locationStatus: StudentLocationStatus | null;
  className?: string;
  style?: CSSProperties;
}

/**
 * Unified LocationBadge component that displays student location
 * using structured LocationStatus data.
 *
 * Styled to match the original ModernStatusBadge design with
 * pulsing indicator, solid colors, and shadows.
 */
export function LocationBadge({
  locationStatus,
  className = "",
  style: styleProp,
}: LocationBadgeProps) {
  const badgeConfig = getStudentLocationBadge(locationStatus);

  const computedStyle: CSSProperties = {
    backgroundColor: badgeConfig.colorToken,
    boxShadow: badgeConfig.shadow,
    ...styleProp,
  };

  return (
    <span
      className={clsx(
        "inline-flex items-center rounded-full px-3 py-1.5 text-xs font-bold transition-all duration-300",
        badgeConfig.textClass,
        className,
      )}
      style={computedStyle}
    >
      <span className="mr-2 h-2 w-2 animate-pulse rounded-full bg-white/80"></span>
      {badgeConfig.label}
    </span>
  );
}
