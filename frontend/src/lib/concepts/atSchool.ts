import { BackpackIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Petrol like LOCATION_COLORS.AT_SCHOOL: an expected child before the first
// check-in is still in class, not at home (#3260).
export const atSchoolConcept = defineMotoConcept(
  "atSchool",
  "Schule",
  BackpackIcon,
  "petrol",
  "status",
  "presence",
);
