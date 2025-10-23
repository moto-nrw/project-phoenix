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
   * Solid fallback color (hex or CSS var) used when gradients are not supported.
   */
  colorToken: string;
  /**
   * Gradient definition applied as background-image. May reuse the solid color.
   */
  gradientToken: string;
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
    gradientToken: "linear-gradient(135deg, #83CD2D 0%, #64B225 100%)",
    icon: "home",
    textClass: "text-white",
  },
  otherRoom: {
    colorToken: "#5080D8",
    gradientToken: "linear-gradient(135deg, #6A99F0 0%, #3A66C4 100%)",
    icon: "door-open",
    textClass: "text-white",
  },
  transit: {
    colorToken: "#D946EF",
    gradientToken: "linear-gradient(135deg, #D946EF 0%, #A21CAF 100%)",
    icon: "bus",
    textClass: "text-white",
  },
  schoolyard: {
    colorToken: "#F78C10",
    gradientToken: "linear-gradient(135deg, #FDBA74 0%, #F97316 100%)",
    icon: "trees",
    textClass: "text-white",
  },
  home: {
    colorToken: "#FF3130",
    gradientToken: "linear-gradient(135deg, #FF6B6B 0%, #E11D48 100%)",
    icon: "house",
    textClass: "text-white",
  },
} as const;

export function getBadgeThemeEntry(
  key: StudentLocationBadgeTokenKey,
): StudentLocationBadgeThemeEntry {
  return STUDENT_LOCATION_BADGE_TOKENS[key];
}
