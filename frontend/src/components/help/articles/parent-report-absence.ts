import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Abwesenheit melden (i18n `parentChildCare.sick`). Krank geht meist sofort
 * durch, andere Gruende sind eine Anfrage -- je nach Einstellung der OGS.
 */
export function parentReportAbsenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentReportAbsence,
    title: "Eine Abwesenheit melden",
    question: "Wie melde ich mein Kind krank oder ab?",
    summary: "Sie melden der OGS, an welchen Tagen Ihr Kind nicht kommt.",
    group: "mein-kind",
    audience: "parent",
    icon: "BellRing",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Tippen Sie unter `Heute` auf den Knopf mit dem Namen Ihres Kindes und `abmelden`.",
      "Wählen Sie im Fenster `Abwesenheit melden` bei `Grund der Abwesenheit` `Krank melden` oder `Entschuldigen`.",
      "Wählen Sie bei `Tage der Abwesenheit` den `Erster Tag` und den `Letzter Tag`.",
      "Tragen Sie bei `Grund / Hinweis an die OGS` kurz ein, worum es geht. Das ist ein Pflichtfeld.",
      "Tippen Sie auf `Krankmeldung an die OGS senden` oder `Entschuldigung an die OGS senden`.",
    ],
    result:
      "Die Abmeldung steht am Kind. Die Anmeldung bei der OGS bleibt unverändert.",
    notes: [
      "`Krank melden` ist für Krankheitstage. Die OGS sieht die Meldung sofort.",
      "`Entschuldigen` ist für Termine und geplante Abwesenheiten.",
      "Für einen einzelnen Tag wählen Sie denselben Tag zweimal.",
    ],
    differences: [
      "Manche OGS bestätigen jede Meldung erst. Dann steht `Freigabe ausstehend`, und Ihr Kind gilt bis dahin als erwartet.",
      "Am Kind steht danach `Krank` oder `Entschuldigt` mit den Tagen.",
      "Wurde eine Anfrage abgelehnt, steht dort `Abgelehnt`.",
    ],
    troubleshootingDetails: [
      "moto meldet, für einen Tag liege schon eine Abmeldung vor? Prüfen Sie die bestehende Meldung mit `Anfrage bearbeiten`.",
      "Bei Ihrer offenen Anfrage fehlt `Anfrage bearbeiten`? Dann dürfen Sie für dieses Kind keine Abwesenheit melden. Fragen Sie die OGS.",
      "Ihr Kind kommt nur später? Ändern Sie stattdessen die Abholzeit oder schreiben Sie der OGS.",
    ],
    related: [
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentMessages,
    ],
  };
}
