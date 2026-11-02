import { NotebookIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Wochenprotokoll je Gruppe (#4769): die Woche aus den Abenden der Tage.
export const weekProtocolsConcept = defineMotoConcept(
  "weekProtocols",
  "Wochenprotokolle",
  NotebookIcon,
  "greenDeep",
  "function",
  "planning",
);
