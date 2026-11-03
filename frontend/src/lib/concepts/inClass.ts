import { ChalkboardTeacherIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Indigo like LOCATION_COLORS.IN_CLASS: the child was checked out with "Geht zum
// Unterricht" and is expected back, so it is neither at home nor still before
// its first check-in (#5656).
export const inClassConcept = defineMotoConcept(
  "inClass",
  "Im Unterricht",
  ChalkboardTeacherIcon,
  "indigo",
  "status",
  "presence",
);
