import { FolderOpenIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Dateiablage (#2596): gemeinsame Dateien der OGS mit Ordner-Freigaben.
export const filesConcept = defineMotoConcept(
  "files",
  "Dateien",
  FolderOpenIcon,
  "neutral",
  "function",
  "system",
);
