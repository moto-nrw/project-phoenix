import { TrayIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Anfragen-Modul (#2429): eingereichte Wünsche von Eltern und
// Mitarbeitenden, über die die OGS entscheidet.
export const requestsConcept = defineMotoConcept(
  "requests",
  "Anfragen",
  TrayIcon,
  "blue",
  "function",
  "communication",
);
