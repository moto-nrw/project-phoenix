import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Abholzeit fuer einen Tag (i18n `parentChildCare.pickup`). Immer Anfrage.
 * Seit #3945 auch die Abholart, wenn die Schule das erlaubt; dann heisst der
 * Knopf `Abholung für ... ändern`.
 */
export function parentPickupChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentPickupChange,
    title: "Die Abholung für einen Tag ändern",
    question:
      "Wie ändere ich die Abholzeit oder die Abholart für einen einzelnen Tag?",
    summary:
      "Sie fragen eine andere Abholzeit oder Abholart an. Der Wochenplan bleibt.",
    group: "mein-kind",
    audience: "parent",
    icon: "Clock3",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Tippen Sie unter `Heute` auf `Abholzeit für ... ändern`.",
      "Wählen Sie bei `Tag` den betroffenen Tag.",
      "Tragen Sie bei `Abholzeit` die neue Zeit ein, zum Beispiel 15:30.",
      "Holt jemand anderes ab? Wählen Sie bei `Wer holt ab?` die Person.",
      "Tragen Sie bei `Grund für die Änderung` kurz ein, warum.",
      "Haben Sie eine Person gewählt, unterschreiben Sie die Erlaubnis im Feld. Tippen Sie auf `Unterschrift bestätigen`.",
      "Tippen Sie auf `Anfrage senden`.",
    ],
    result:
      "Die Änderung gilt erst, nachdem die OGS sie bestätigt hat. Der reguläre Wochenplan bleibt unverändert. Eine gewählte Person darf nur an diesem Tag abholen.",
    differences: [
      "Steht dort `Abholung für ... ändern`? Dann können Sie auch die Abholart ändern. Wählen Sie bei `Abholart an diesem Tag` zum Beispiel `Wird abgeholt`. Lassen Sie `Abholzeit` leer, wenn die Uhrzeit gleich bleibt.",
      "Hat die OGS die Abholart für den Tag schon festgelegt? Dann steht sie im Fenster. Für eine Änderung schreiben Sie der OGS.",
      "Wählen Sie `Geht allein` für den Tag? Dann unterschreiben Sie auch diese Erlaubnis.",
    ],
    notes: [
      "Der Grund ist Pflicht und darf höchstens 255 Zeichen lang sein.",
      "Bei `Wer holt ab?` stehen nur Personen, die Sie bei den Kontakten eingetragen haben. Eine neue Person tragen Sie vorher dort ein.",
      "Nach dem Senden finden Sie bei der Anfrage `Nachweis (PDF)`. Darin stehen Ihre Erklärung und Ihre Unterschrift.",
      "Soll sich die Zeit dauerhaft ändern? Dann fragen Sie eine Änderung der Betreuung an.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Ändern der Abholzeit sei für diese OGS nicht aktiviert? Dann schreiben Sie der OGS eine Nachricht.",
      "`Wer holt ab?` fehlt? Dann haben Sie noch keine weitere Person bei den Kontakten eingetragen.",
    ],
    related: [
      HELP_TOPICS.parentCareChange,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentGuardians,
    ],
  };
}
