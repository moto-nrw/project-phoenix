import { SunIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Orange, matching LOCATION_COLORS.SCHOOLYARD (#F78C10) — the badge for
// this very status. Every other status concept sits in the hue family of
// its badge; Schulhof was the one that did not, so the dashboard tile and
// the student-list pill showed the same status in two colors. Amber is
// reserved for LOCATION_COLORS.WARNING now.
export const schoolyardConcept = defineMotoConcept(
  "schoolyard",
  "Schulhof",
  SunIcon,
  "orange",
  "status",
  "presence",
);
