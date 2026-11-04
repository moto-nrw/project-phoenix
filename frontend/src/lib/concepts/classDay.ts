import { ChalkboardTeacherIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Schul-Portal ("moto schule", #2207): der Klassentag einer Lehrkraft.
// Gleiches Bild wie "Schulen", aber im Blau des Schul-Portals — dort geht
// es um EINE Klasse am heutigen Tag, nicht um die Schulliste des Betriebs.
export const classDayConcept = defineMotoConcept(
  "classDay",
  "Klassenansicht",
  ChalkboardTeacherIcon,
  "blue",
  "function",
  "planning",
);
