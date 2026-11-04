import { ShieldCheckIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Primary responsibility for a slot, as opposed to merely supervising it.
// Carries the ShieldCheck the ResponsibilityBadge used before the concept
// system existed; "function" kind keeps it out of the unique-primary-color
// rule that only governs core and status concepts.
export const responsibilityConcept = defineMotoConcept(
  "responsibility",
  "Zuständig",
  ShieldCheckIcon,
  "teal",
  "function",
  "people",
);
