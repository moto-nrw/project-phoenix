import { FileTextIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Dokumente des Trägers an die Familie (#4671), etwa Bestätigungsschreiben.
export const documentsConcept = defineMotoConcept(
  "documents",
  "Dokumente",
  FileTextIcon,
  "petrol",
  "function",
  "communication",
);
