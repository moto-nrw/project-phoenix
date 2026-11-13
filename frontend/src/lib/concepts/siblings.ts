import { UsersThreeIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Geschwister einer Familie (Ermäßigungsregel des Trägers): mehrere Kinder,
// darum drei Köpfe im Ton der Kinder, nicht im Ton der Gruppe.
export const siblingsConcept = defineMotoConcept(
  "siblings",
  "Geschwister",
  UsersThreeIcon,
  "greenVivid",
  "function",
  "people",
);
