import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Bankverbindungen unter `/eltern/bankverbindungen`. Reine Leseansicht mit
 * Export; die IBAN pflegen die Eltern selbst im Portal. Sichtbar nur mit
 * `guardians:financial` (sidebar.tsx, PARENT_SUB_PAGES-Filter).
 */
export function bankDetailsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadBankDetails,
    title: "Bankverbindungen einsehen",
    question: "Wo finde ich die Bankverbindungen der Familien?",
    summary: "Sie sehen je Kind die hinterlegte IBAN und können sie ausgeben.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Landmark",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Bankverbindungen`.",
      "Wählen Sie `Alle Kinder` oder `Ohne IBAN`.",
      "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
      "Wählen Sie `Exportieren`.",
      "Wählen Sie unter `Format` `Excel`, `PDF` oder `Word`.",
      "Wählen Sie `Herunterladen`.",
    ],
    result:
      "Die Liste wird als Datei erstellt und heruntergeladen. Sie enthält immer alle Kinder, auch wenn die Liste gerade gefiltert ist.",
    notes: [
      "Die IBAN tragen Sie beim Kind ein, im Reiter `Erziehungsberechtigte`.",
      "Die Datei enthält die vollständigen IBANs und wird protokolliert. Geben Sie sie nicht per E-Mail weiter.",
      "`Ohne IBAN` zeigt, bei welchen Kindern noch etwas fehlt.",
    ],
    differences: [
      "Fehlt bei einem Kind die IBAN, steht dort `Nicht zugeordnet`.",
    ],
    troubleshootingDetails: [
      "Fehlt `Bankverbindungen` in der Seitenleiste? Dafür braucht es ein eigenes Recht. Fragen Sie Ihre Leitung.",
      "`Exportieren` bleibt grau? Dann ist die Liste leer.",
    ],
    related: [
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
