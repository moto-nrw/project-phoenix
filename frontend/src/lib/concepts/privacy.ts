import { ShieldCheckIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Der Bereich „Datenschutz" der Einstellungen (#5750). Gleiches Schild wie
// „Zuständig": die Szene hängt am Glyph, nicht am Konzept.
export const privacyConcept = defineMotoConcept(
  "privacy",
  "Datenschutz",
  ShieldCheckIcon,
  "neutral",
  "function",
  "system",
);
