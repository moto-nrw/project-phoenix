import { CoinsIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Träger-Portal (#5145): die Regeln für den Ferienbeitrag (#4664) und die
// Abrechnungsläufe an die Familien (#4668). Die Abrechnung des Trägers ist
// eine Rechnung an Familien, nicht die Lohnabrechnung (`payroll`).
export const contributionsConcept = defineMotoConcept(
  "contributions",
  "Beiträge",
  CoinsIcon,
  "gold",
  "function",
  "administration",
);
