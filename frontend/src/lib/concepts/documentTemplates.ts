import { NotebookIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Vorlagen des Trägers je Dokumentart (#4677): der Text der Briefe mit
// Platzhaltern. Teilt sich die Szene des Notizbuchs mit den Wochenprotokollen.
export const documentTemplatesConcept = defineMotoConcept(
  "documentTemplates",
  "Vorlagen",
  NotebookIcon,
  "petrol",
  "function",
  "administration",
);
