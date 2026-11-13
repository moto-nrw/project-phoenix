import { PersonIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Alleinerziehende (Ermäßigungsregel des Trägers): eine erwachsene Person,
// als ganze Figur, damit sie nicht wie der Kopf eines Kindes liest; Ton der
// Eltern.
export const singleParentConcept = defineMotoConcept(
  "singleParent",
  "Alleinerziehende",
  PersonIcon,
  "blue",
  "function",
  "people",
);
