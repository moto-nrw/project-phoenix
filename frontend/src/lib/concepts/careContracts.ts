import { SignatureIcon } from "@phosphor-icons/react/ssr";
import { defineMotoConcept } from "./define";

// Betreuungsverträge des Trägers mit den Eltern (#4678): Vorlage je Angebot,
// Entwurf aus der angenommenen Anmeldung, Versand.
export const careContractsConcept = defineMotoConcept(
  "careContracts",
  "Verträge",
  SignatureIcon,
  "blue",
  "function",
  "administration",
);
