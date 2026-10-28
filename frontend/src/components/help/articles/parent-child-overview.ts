import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/children`. Live geprueft: die Kindseite zeigt oben `Heute` mit
 * `Aktueller Status`, `Abholung` und drei Knoepfen, darunter die drei
 * Reiter `Betreuung`, `Angaben` und `Kontakte`. Die langen Bereichsnamen
 * aus `parentChild.areas.*.title` stehen nur im aria-label.
 */
export function parentChildOverviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildOverview,
    title: "Mein Kind und den heutigen Tag ansehen",
    question: "Wo sehe ich, wie der Tag meines Kindes läuft?",
    summary: "Die Seite `Kinder` zeigt den heutigen Tag und drei Bereiche.",
    group: "mein-kind",
    audience: "parent",
    icon: "UserRoundSearch",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Haben Sie mehrere Kinder, wählen Sie oben das Kind.",
      "Lesen Sie unter `Heute` den `Aktueller Status` und die `Abholung`.",
      "Wählen Sie darunter einen Reiter: `Betreuung`, `Angaben` oder `Kontakte`.",
    ],
    result: "Sie sehen den gewählten Reiter mit allen Angaben dazu.",
    notes: [
      "`Betreuung`: Abwesenheiten, Wochenplan, gebuchte Betreuung und der Weg nach Hause.",
      "`Angaben`: persönliche Angaben, Klasse und Gesundheit.",
      "`Kontakte`: Abholberechtigte, Kontakte und verbundene Konten.",
      "Unter `Heute` stehen die Knöpfe zum Abmelden, zur Abholzeit und für eine Nachricht.",
    ],
    differences: [
      "Haben Sie mehrere Kinder, heißt der Punkt unten `Meine Kinder`.",
      "Ist Ihr Kind heute abgemeldet, steht dort `Heute abgemeldet, keine Abholung`.",
      "Wurde die Abholzeit für heute geändert, steht die Zeit mit dem Hinweis `geändert für heute`.",
      "Ist für heute nichts eingetragen, steht dort `Für heute ist keine Abholung eingetragen`.",
    ],
    troubleshootingDetails: [
      "Sie sehen kein Kind? Dann hat die OGS Ihren Zugang noch nicht freigegeben.",
    ],
    related: [
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentCalendar,
    ],
  };
}
