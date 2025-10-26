import {
  LOCATION_STATUSES,
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
} from "@/lib/location-helper";

interface StatusBadgeProps {
  location?: string;
}

const DEFAULT_STYLE = {
  label: LOCATION_STATUSES.UNKNOWN,
  indicator: "bg-gray-500",
  textColor: "text-gray-800",
  background: "bg-gray-100",
};

export function StatusBadge({ location }: StatusBadgeProps) {
  const normalized = location?.trim();

  const status = (() => {
    if (!normalized) {
      return DEFAULT_STYLE;
    }

    if (normalized === "Bus" || isTransitLocation(normalized)) {
      return {
        label: normalized === "Bus" ? "Bus" : LOCATION_STATUSES.TRANSIT,
        indicator: "bg-fuchsia-500",
        textColor: "text-fuchsia-800",
        background: "bg-fuchsia-100",
      };
    }

    if (isHomeLocation(normalized)) {
      return {
        label: LOCATION_STATUSES.HOME,
        indicator: "bg-orange-500",
        textColor: "text-orange-800",
        background: "bg-orange-100",
      };
    }

    if (isSchoolyardLocation(normalized)) {
      return {
        label: LOCATION_STATUSES.SCHOOLYARD,
        indicator: "bg-yellow-500",
        textColor: "text-yellow-800",
        background: "bg-yellow-100",
      };
    }

    const parsed = parseLocation(normalized);
    if (parsed.status === LOCATION_STATUSES.PRESENT) {
      return {
        label: parsed.room ?? LOCATION_STATUSES.PRESENT,
        indicator: "bg-green-500",
        textColor: "text-green-800",
        background: "bg-green-100",
      };
    }

    return DEFAULT_STYLE;
  })();

  return (
    <div className={`inline-flex items-center rounded-full px-3 py-1 ${status.background} ${status.textColor} font-medium text-sm`}>
      <span className={`mr-1.5 inline-block h-2 w-2 rounded-full ${status.indicator}`} />
      {status.label}
    </div>
  );
}
