import { SunHorizonIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Ferienwochen der ausrichtenden OGS (#4665): die bestätigten Kinder der
// Wochen, die die Schule für ihren Träger ausrichtet.
export const holidayWeeksConcept = defineMotoConcept(
  "holidayWeeks",
  "Ferienwochen",
  SunHorizonIcon,
  "orange",
  "function",
  "administration",
);
