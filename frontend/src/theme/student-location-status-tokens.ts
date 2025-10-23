// theme/student-location-status-tokens.ts
// Centralized styling tokens for student location badges.

export type StudentLocationBadgeTokenKey =
  | "groupRoom"
  | "otherRoom"
  | "transit"
  | "schoolyard"
  | "home";

export interface StudentLocationBadgeThemeEntry {
  /**
   * Solid background color (hex or CSS var).
   */
  colorToken: string;
  /**
   * Box shadow definition for depth effect.
   */
  shadow: string;
  /**
   * Icon identifier (kept as string so rendering implementation can decide how to map it).
   */
  icon: string;
  /**
   * Utility class for text color to maintain contrast with the badge background.
   */
  textClass: string;
}

export const STUDENT_LOCATION_BADGE_TOKENS: Record<
  StudentLocationBadgeTokenKey,
  StudentLocationBadgeThemeEntry
> = {
  groupRoom: {
    colorToken: "#83CD2D",
    shadow: "0 8px 25px rgba(131, 205, 45, 0.4)",
    icon: "home",
    textClass: "text-white backdrop-blur-sm",
  },
  otherRoom: {
    colorToken: "#5080D8",
    shadow: "0 8px 25px rgba(80, 128, 216, 0.4)",
    icon: "door-open",
    textClass: "text-white backdrop-blur-sm",
  },
  transit: {
    colorToken: "#D946EF",
    shadow: "0 8px 25px rgba(217, 70, 239, 0.4)",
    icon: "bus",
    textClass: "text-white backdrop-blur-sm",
  },
  schoolyard: {
    colorToken: "#F78C10",
    shadow: "0 8px 25px rgba(247, 140, 16, 0.4)",
    icon: "trees",
    textClass: "text-white backdrop-blur-sm",
  },
  home: {
    colorToken: "#FF3130",
    shadow: "0 8px 25px rgba(255, 49, 48, 0.4)",
    icon: "house",
    textClass: "text-white backdrop-blur-sm",
  },
} as const;

export function getBadgeThemeEntry(
  key: StudentLocationBadgeTokenKey,
): StudentLocationBadgeThemeEntry {
  return STUDENT_LOCATION_BADGE_TOKENS[key];
}
