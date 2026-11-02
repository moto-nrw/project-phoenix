import { QuestionIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

export const unknownConcept = defineMotoConcept(
  "unknown",
  "Unbekannt",
  QuestionIcon,
  "stone",
  "status",
  "presence",
);
