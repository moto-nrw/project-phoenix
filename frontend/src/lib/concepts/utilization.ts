import { ChartBarIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Gold, not orange: orange belongs to the Schulhof status above, and the
// unique-primary-color rule for status concepts allows only one owner.
// Auslastung is a metric rather than a presence state, so it has no badge
// hue of its own to match.
export const utilizationConcept = defineMotoConcept(
  "utilization",
  "Auslastung",
  ChartBarIcon,
  "gold",
  "status",
  "presence",
);
