import { LockKeyIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Die Anmeldung eines Kontos: Passwort und Code per E-Mail (#7051). Gleiches
// Schloss wie „Berechtigungen“; die Szene hängt am Glyph, nicht am Konzept.
export const signInConcept = defineMotoConcept(
  "signIn",
  "Anmeldung",
  LockKeyIcon,
  "greenDeep",
  "function",
  "system",
);
