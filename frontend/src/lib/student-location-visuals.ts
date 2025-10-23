import type { CSSProperties } from "react";
import type { StudentLocationStatus } from "./student-location-helpers";

type LocationCardThemeKey =
  | "groupRoom"
  | "otherRoom"
  | "transit"
  | "schoolyard"
  | "home";

interface LocationCardThemeConfig {
  overlayGradient: string;
  badgeShadow: string;
  hoverBorderClass: string;
  hoverRingClass: string;
  glowClassName: string;
  badgeClassName: string;
}

export interface StudentLocationCardVisuals {
  overlayStyle: CSSProperties;
  badgeStyle: CSSProperties;
  badgeClassName: string;
  hoverBorderClass: string;
  hoverRingClass: string;
  glowClassName: string;
}

const LOCATION_CARD_THEMES: Record<LocationCardThemeKey, LocationCardThemeConfig> =
  {
    groupRoom: {
      overlayGradient:
        "linear-gradient(135deg, rgba(131,205,45,0.28) 0%, rgba(100,178,37,0.16) 100%)",
      badgeShadow: "0 8px 25px rgba(131, 205, 45, 0.35)",
      hoverBorderClass: "md:hover:border-emerald-200/50",
      hoverRingClass: "md:group-hover:ring-emerald-200/60",
      glowClassName:
        "bg-gradient-to-r from-transparent via-emerald-100/35 to-transparent",
      badgeClassName: "backdrop-blur-sm",
    },
    otherRoom: {
      overlayGradient:
        "linear-gradient(135deg, rgba(106,153,240,0.28) 0%, rgba(58,102,196,0.16) 100%)",
      badgeShadow: "0 8px 25px rgba(80, 128, 216, 0.35)",
      hoverBorderClass: "md:hover:border-blue-200/50",
      hoverRingClass: "md:group-hover:ring-blue-200/60",
      glowClassName:
        "bg-gradient-to-r from-transparent via-blue-100/35 to-transparent",
      badgeClassName: "backdrop-blur-sm",
    },
    transit: {
      overlayGradient:
        "linear-gradient(135deg, rgba(217,70,239,0.26) 0%, rgba(162,28,175,0.18) 100%)",
      badgeShadow: "0 8px 25px rgba(217, 70, 239, 0.28)",
      hoverBorderClass: "md:hover:border-fuchsia-200/50",
      hoverRingClass: "md:group-hover:ring-fuchsia-200/60",
      glowClassName:
        "bg-gradient-to-r from-transparent via-fuchsia-100/35 to-transparent",
      badgeClassName: "backdrop-blur-sm",
    },
    schoolyard: {
      overlayGradient:
        "linear-gradient(135deg, rgba(253,186,116,0.26) 0%, rgba(249,115,22,0.18) 100%)",
      badgeShadow: "0 8px 25px rgba(247, 140, 16, 0.3)",
      hoverBorderClass: "md:hover:border-amber-200/50",
      hoverRingClass: "md:group-hover:ring-amber-200/60",
      glowClassName:
        "bg-gradient-to-r from-transparent via-amber-100/35 to-transparent",
      badgeClassName: "backdrop-blur-sm",
    },
    home: {
      overlayGradient:
        "linear-gradient(135deg, rgba(255,107,107,0.26) 0%, rgba(225,29,72,0.18) 100%)",
      badgeShadow: "0 8px 25px rgba(255, 49, 48, 0.3)",
      hoverBorderClass: "md:hover:border-rose-200/50",
      hoverRingClass: "md:group-hover:ring-rose-200/60",
      glowClassName:
        "bg-gradient-to-r from-transparent via-rose-100/35 to-transparent",
      badgeClassName: "backdrop-blur-sm",
    },
  };

function resolveThemeKey(
  status: StudentLocationStatus | null,
): LocationCardThemeKey {
  if (!status) {
    return "home";
  }

  switch (status.state) {
    case "PRESENT_IN_ROOM":
      return status.room?.isGroupRoom ? "groupRoom" : "otherRoom";
    case "TRANSIT":
      return "transit";
    case "SCHOOLYARD":
      return "schoolyard";
    case "HOME":
    default:
      return "home";
  }
}

export function getLocationCardVisuals(
  status: StudentLocationStatus | null,
): StudentLocationCardVisuals {
  const key = resolveThemeKey(status);
  const theme = LOCATION_CARD_THEMES[key];

  return {
    overlayStyle: {
      backgroundImage: theme.overlayGradient,
    },
    badgeStyle: {
      boxShadow: theme.badgeShadow,
    },
    badgeClassName: theme.badgeClassName,
    hoverBorderClass: theme.hoverBorderClass,
    hoverRingClass: theme.hoverRingClass,
    glowClassName: theme.glowClassName,
  };
}
