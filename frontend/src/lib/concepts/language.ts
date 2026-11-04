import { TranslateIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Die drei Huellen-Konzepte der Eltern-App. Sie stehen in der Navigation
// neben den Zielen und muessen deshalb dieselbe Duotone-Sprache sprechen.
export const languageConcept = defineMotoConcept(
  "language",
  "Sprache",
  TranslateIcon,
  "petrol",
  "function",
  "system",
);
