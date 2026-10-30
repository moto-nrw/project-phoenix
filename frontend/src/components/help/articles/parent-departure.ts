import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Abschnitt `So geht ... nach Hause` im Reiter `Betreuung` (Komponente
 * `DepartureSection` in components/parent/child-master-data.tsx). Je
 * Wochentag ein Kaestchen pro Weg; `accompanied` steht nur lesend da, weil
 * ein begleitetes Kind ein zweites Kind betrifft. Geaendert wird immer per
 * Anfrage (`requestButton`), nie direkt.
 */
export function parentDepartureTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDeparture,
    title: "Den Weg nach Hause ändern",
    question: "Wie ändere ich, wie mein Kind nach Hause geht?",
    summary: "Sie sehen den Weg je Wochentag und fragen eine Änderung an.",
    group: "mein-kind",
    audience: "parent",
    icon: "Repeat",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Wählen Sie den Reiter `Betreuung`.",
      "Gehen Sie ganz nach unten zum Abschnitt `So geht ... nach Hause`. Dort steht der Name Ihres Kindes. Sie sehen für jeden Wochentag, wie Ihr Kind nach Hause geht.",
      "Tippen Sie auf `Bearbeiten`.",
      "Setzen Sie je Wochentag einen Haken bei `Geht allein`, `Bus` oder `Wird abgeholt`.",
      "Tippen Sie auf `Änderung anfragen`.",
    ],
    result:
      "Die OGS prüft Ihre Anfrage. Bis zur Bestätigung gilt der bisherige Weg. Ihre Anfrage steht darunter bei `Beantragte Änderung`.",
    notes: [
      "Pro Tag sind mehrere Wege möglich. Dann darf Ihr Kind jeden davon nehmen.",
      "Die Tage sind `Mo` bis `Fr`. Jeder Tag hat eine eigene Auswahl.",
      "Mit `Abbrechen` verwerfen Sie Ihre Auswahl. Der bisherige Weg bleibt.",
      "Gibt es weitere Sorgeberechtigte mit App-Zugang, wählen Sie unter `Anfrage teilen (optional)`, wer mitlesen darf.",
    ],
    differences: [
      "Steht dort `Beantragte Änderung` mit `In Prüfung`, wartet schon eine Änderung auf die OGS. Bis dahin gibt es kein `Bearbeiten`.",
      "Geht Ihr Kind mit einem anderen Kind, steht beim Tag `Geht mit anderem Kind`. Dann gibt es kein `Bearbeiten`.",
      "Steht dort `Es gibt keine weiteren Sorgeberechtigten mit Zugang zur Eltern-App`? Dann gibt es niemanden zum Mitlesen.",
    ],
    troubleshootingDetails: [
      "Es gibt kein `Bearbeiten`? Geht Ihr Kind an einem Tag mit einem anderen Kind, ändern Sie den Weg nicht selbst. Schreiben Sie der OGS.",
      "moto sagt, Änderungsanfragen seien deaktiviert? Dann nimmt Ihre OGS keine Änderungen über die App an.",
    ],
    related: [
      HELP_TOPICS.parentCareChange,
      HELP_TOPICS.parentGuardians,
      HELP_TOPICS.parentPickupChange,
    ],
  };
}
