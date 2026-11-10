import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/ferien/kriterien` und `/traeger/ferien/[id]/vergabe` (#4700):
 * Plätze nach Kriterien vergeben. moto sortiert vor, das Büro entscheidet.
 */
export function traegerHolidayAllocationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayAllocation,
    title: "Ferienplätze nach Kriterien vergeben",
    question: "Wie vergebe ich Ferienplätze nach Kriterien?",
    summary:
      "Eltern beantworten Ihre Fragen mit Ja oder Nein. moto sortiert die Anmeldungen danach vor. Ihr Büro entscheidet über jeden Platz.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CheckSquare",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
      "Der Ferienzeitraum hat noch keine Anmeldung. Danach bleibt die Art der Platzvergabe so.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Kriterien festlegen",
        steps: [
          "Wählen Sie in der Navigation `Ferienbetreuung`.",
          "Wählen Sie oben `Kriterien der Platzvergabe`.",
          "Wählen Sie eine Vorlage oder `Eigenes Kriterium`.",
          "Geben Sie den Namen und die `Frage an die Eltern` ein. Wählen Sie bei `Gilt für`, ob die Frage für die Familie oder für jedes Kind gilt.",
          "Bringen Sie die Kriterien mit den Pfeilen in die richtige Reihenfolge. Das oberste Kriterium zählt am meisten.",
          "Wählen Sie `Kriterien speichern`.",
        ],
      },
      {
        title: "Ferienzeitraum nach Kriterien vergeben",
        steps: [
          "Öffnen Sie den Ferienzeitraum im Reiter `Planung`.",
          "Wählen Sie bei `Wie werden die Plätze vergeben?` die Karte `Nach Kriterien`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Plätze vergeben",
        steps: [
          "Öffnen Sie den Ferienzeitraum und wählen Sie den Reiter `Vergabe`.",
          "Je Woche stehen die offenen Anmeldungen in der Reihenfolge des Vorschlags. Die Linie zeigt, wo die freien Plätze enden.",
          "Wählen Sie bei einem Kind `Platz geben` oder `Auf die Warteliste`. Weicht das vom Vorschlag ab, schreiben Sie eine `Begründung`.",
          "Oder wählen Sie bei der Woche `Vorschlag übernehmen`. Mit mehreren Wochen gibt es oben `Vorschlag für alle Wochen übernehmen`.",
          "Bestätigen Sie im Fenster.",
        ],
      },
      {
        title: "Angaben einer Familie prüfen",
        steps: [
          "Öffnen Sie die Anmeldung.",
          "Wählen Sie bei `Angaben für die Vergabe` den Knopf `Prüfen`.",
          "Wählen Sie je Kriterium `Trifft zu`, `Trifft nicht zu` oder `Nicht geprüft`.",
          "Wählen Sie `Prüfung speichern`.",
        ],
      },
    ],
    result:
      "Die Eltern bekommen eine E-Mail mit dem Platz oder der Warteliste. Danach rücken Kinder von der Warteliste automatisch nach, wenn ein Platz frei wird. Die Warteliste ist nach Ihren Kriterien sortiert, dann nach Eingang.",
    notes: [
      "moto schlägt nur vor. Jede Entscheidung trifft Ihr Büro. Unter `Verlauf der Entscheidungen` steht jede Entscheidung mit Vorschlag und Begründung.",
      "Bei gleichen Kriterien zählt, wer sich zuerst angemeldet hat.",
      "Eine Prüfung des Büros zählt vor der Angabe der Eltern. Sie ändert den Vorschlag für offene Plätze.",
      "Fragen Sie nur, was Sie für die Vergabe brauchen. Fragen Sie nicht nach Gesundheit oder Förderbedarf.",
      "Ein Kind mit offenem Platz zahlt nichts. Es steht auch nicht auf der Teilnehmerliste der OGS.",
      "Eine Anmeldung mit offenem Platz können Sie erst bestätigen, wenn das Büro entschieden hat.",
    ],
    differences: [
      "Bei einem Kriterium steht `ungeklärt`? Die Eltern haben dazu nichts gesagt, zum Beispiel weil das Kriterium später dazukam. Es zählt erst, wenn Ihr Büro es geprüft hat.",
      "Der Reiter `Vergabe` fehlt? Der Ferienzeitraum vergibt die Plätze nach Eingang.",
      "Die Karten bei `Wie werden die Plätze vergeben?` lassen sich nicht wählen? Es gibt schon Anmeldungen.",
    ],
    troubleshootingDetails: [
      "Es erscheint, dass sich die Liste geändert hat? Jemand hat gerade entschieden oder eine Angabe geprüft. moto lädt die Liste neu. Prüfen Sie Ihre Entscheidung noch einmal.",
      "`Platz geben` geht nicht, weil kein Platz frei ist? Erhöhen Sie im Reiter `Plätze` die Zahl der Plätze.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayPlaces,
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayRegistrationReview,
    ],
  };
}
