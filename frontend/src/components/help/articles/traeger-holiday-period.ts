import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/ferien` (#4659): Ferienzeitraum mit Wochen und Zeitmodellen. */
export function traegerHolidayPeriodTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayPeriod,
    title: "Ferienzeitraum anlegen",
    question: "Wie plane ich Ferienbetreuung für mehrere Schulen?",
    summary:
      "Ein Ferienzeitraum hat buchbare Wochen. Jede Woche findet an einer Schule Ihres Trägers statt.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "SunHorizon",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Ferienzeitraum anlegen",
        steps: [
          "Wählen Sie in der Navigation `Ferien`.",
          "Wählen Sie `Ferienzeitraum anlegen`.",
          "Tragen Sie den `Name des Ferienzeitraums` ein, zum Beispiel Osterferien 2027.",
          "Tragen Sie `Anmeldebeginn` und `Anmeldeschluss` ein.",
          "Ist die Anmeldung verbindlich? Dann lassen Sie den Haken bei `Die Anmeldung ist verbindlich.`.",
          "Schreiben Sie unter `Hinweise für Eltern`, was Eltern wissen müssen. Zum Beispiel, ob es Mittagessen gibt oder ein Lunchpaket nötig ist.",
        ],
      },
      {
        title: "Wochen und Zeitmodelle planen",
        steps: [
          "Tragen Sie bei jeder Woche `Erster Tag` und `Letzter Tag` ein.",
          "Wählen Sie die `Ausrichtende OGS`. Dort findet die Betreuung in dieser Woche statt.",
          "Tragen Sie die Zeitmodelle ein, zum Beispiel „bis 13 Uhr“ und „bis 16 Uhr“. Jedes Zeitmodell hat `Beginn`, `Ende` und `Preis in Euro`.",
          "Weitere Wochen fügen Sie mit `Woche hinzufügen` hinzu.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Veröffentlichen",
        steps: [
          "Öffnen Sie den Ferienzeitraum in der Liste `Ferien`.",
          "Wählen Sie oben `Veröffentlichen`.",
          "Bestätigen Sie mit `Veröffentlichen`.",
        ],
      },
    ],
    result:
      "Der Ferienzeitraum ist für die Anmeldung freigegeben. Sie können ihn danach weiter ändern.",
    notes: [
      "Ein gespeicherter Ferienzeitraum ist zuerst ein Entwurf. Ein Entwurf ist nicht für die Anmeldung freigegeben.",
      "Ein veröffentlichter Ferienzeitraum wird nicht wieder zum Entwurf.",
      "Aus den Preisen rechnet moto den Beitrag der Familien. Geschwister und Befreiung legt die Abrechnung unter `Beiträge` fest.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Ferien`? Ihre Rolle darf keine Anmeldungen verwalten. Fragen Sie im Träger-Büro nach der Rolle Verwaltung.",
    ],
    troubleshootingDetails: [
      "`Veröffentlichen` lässt sich nicht wählen? Speichern Sie zuerst Ihre Änderungen. Jede Woche braucht mindestens ein Zeitmodell.",
      "Eine Schule fehlt bei `Ausrichtende OGS`? Das moto-Team ordnet die Schulen Ihrem Träger zu.",
    ],
    related: [
      HELP_TOPICS.traegerLogin,
      HELP_TOPICS.traegerHolidayForm,
      HELP_TOPICS.traegerHolidayContribution,
    ],
  };
}
