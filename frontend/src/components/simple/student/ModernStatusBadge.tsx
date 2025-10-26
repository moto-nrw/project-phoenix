import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
} from "@/lib/location-helper";

interface ModernStatusBadgeProps {
  location?: string;
  roomName?: string;
}

const DEFAULT_STATUS = {
  label: LOCATION_STATUSES.UNKNOWN,
  bgColor: LOCATION_COLORS.UNKNOWN,
  shadow: "0 8px 25px rgba(107, 114, 128, 0.4)",
  badgeColor: "text-white backdrop-blur-sm",
};

export function ModernStatusBadge({ location, roomName }: ModernStatusBadgeProps) {
  const normalized = location?.trim();

  const status = (() => {
    if (!normalized) {
      return DEFAULT_STATUS;
    }

    if (normalized === "Bus" || isTransitLocation(normalized)) {
      return {
        label: normalized === "Bus" ? "Bus" : LOCATION_STATUSES.TRANSIT,
        bgColor: LOCATION_COLORS.TRANSIT,
        shadow: "0 8px 25px rgba(217, 70, 239, 0.4)",
        badgeColor: "text-white backdrop-blur-sm",
      };
    }

    if (isHomeLocation(normalized)) {
      return {
        label: LOCATION_STATUSES.HOME,
        bgColor: LOCATION_COLORS.HOME,
        shadow: "0 8px 25px rgba(255, 49, 48, 0.4)",
        badgeColor: "text-white backdrop-blur-sm",
      };
    }

    if (isSchoolyardLocation(normalized)) {
      return {
        label: LOCATION_STATUSES.SCHOOLYARD,
        bgColor: LOCATION_COLORS.SCHOOLYARD,
        shadow: "0 8px 25px rgba(247, 140, 16, 0.4)",
        badgeColor: "text-white backdrop-blur-sm",
      };
    }

    const parsed = parseLocation(normalized);
    if (parsed.status === LOCATION_STATUSES.PRESENT) {
      const label = roomName ?? parsed.room ?? LOCATION_STATUSES.PRESENT;
      const bgColor = roomName ? LOCATION_COLORS.GROUP_ROOM : LOCATION_COLORS.OTHER_ROOM;
      const shadow = roomName
        ? "0 8px 25px rgba(131, 205, 45, 0.4)"
        : "0 8px 25px rgba(80, 128, 216, 0.4)";
      return {
        label,
        bgColor,
        shadow,
        badgeColor: "text-white backdrop-blur-sm",
      };
    }

    return DEFAULT_STATUS;
  })();

  return (
    <span
      className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ${status.badgeColor} transition-all duration-300`}
      style={{ backgroundColor: status.bgColor, boxShadow: status.shadow }}
    >
      <span className="w-2 h-2 bg-white/80 rounded-full mr-2 animate-pulse" />
      {status.label}
    </span>
  );
}
