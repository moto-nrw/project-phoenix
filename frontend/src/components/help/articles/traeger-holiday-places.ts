import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/ferien/[id]/plaetze` (#4699): Plätze, Warteliste, Nachrücken. */
export function traegerHolidayPlacesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayPlaces,
    title: "Plätze und Warteliste in den Ferien",
    question: "Wie lege ich Plätze je Ferienwoche fest?",
    summary:
      "Sie legen je Woche fest, wie viele Kinder mitmachen können. Plätze gehen in der Reihenfolge der Anmeldung. Ist eine Woche voll, kommt das Kind auf die Warteliste und rückt automatisch nach.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Hourglass",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
      "Der Ferienzeitraum hat Wochen. Die legen Sie im Reiter `Planung` an.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Plätze festlegen",
        steps: [
          "Wählen Sie in der Navigation `Ferienbetreuung`.",
          "Öffnen Sie den Ferienzeitraum.",
          "Wählen Sie den Reiter `Plätze`.",
          "Wählen Sie bei der Woche `Plätze ändern`.",
          "Schalten Sie `Plätze begrenzen` ein und stellen Sie die Zahl ein.",
          "Wählen Sie `Plätze speichern`.",
        ],
      },
      {
        title: "Warteliste ansehen",
        steps: [
          "Unter jeder Woche steht `Warteliste` mit allen Kindern, die auf einen Platz warten.",
          "Die Zahl vorne ist der Rang. Das Kind mit der 1 bekommt den nächsten freien Platz.",
          "Wählen Sie ein Kind, um seine Anmeldung zu öffnen.",
        ],
      },
      {
        title: "Wann ein Kind nachrückt",
        steps: [
          "Sie stornieren eine Anmeldung mit Platz.",
          "Sie buchen ein Kind aus der Woche heraus.",
          "Sie geben der Woche mehr Plätze oder heben die Grenze auf.",
        ],
        ordered: false,
      },
    ],
    result:
      "Das nächste Kind der Warteliste hat sofort einen Platz. Die Anmeldung gilt dann auch für diese Woche. Ist die Anmeldung verbindlich, ist es der Platz auch. Die Eltern bekommen eine E-Mail. Ist die Anmeldung bestätigt, rechnet moto den Ferienbeitrag mit der neuen Woche neu.",
    notes: [
      "Die Plätze gelten für alle Zeitmodelle einer Woche zusammen.",
      "Bei einer Woche mit einzelnen Tagen gelten die Plätze an jedem Tag. Unter `Plätze je Tag` sehen Sie, wie voll jeder Tag ist. Ein Kind bekommt seinen Platz, wenn alle seine Tage frei sind. Sonst wartet es mit allen Tagen dieser Woche. Ein Kind mit anderen Tagen kann dann vor ihm einen Platz bekommen.",
      "Eingegangene und bestätigte Anmeldungen zählen. Stornierte zählen nicht.",
      "Geschwister bekommen ihre Plätze einzeln. Ein Kind kann einen Platz haben und das andere auf der Warteliste stehen.",
      "Weniger Plätze nehmen niemandem den Platz weg. Neue Anmeldungen kommen dann auf die Warteliste.",
      "Kinder auf der Warteliste zahlen nichts. Sie stehen auch nicht auf der Teilnehmerliste der ausrichtenden OGS.",
      "Eltern sehen nach dem Absenden und in der E-Mail, in welchen Wochen ihr Kind einen Platz hat und wo es auf der Warteliste steht.",
      "Ohne Grenze bekommt jedes angemeldete Kind einen Platz. So ist jede neue Woche eingestellt.",
    ],
    differences: [
      "Vergibt der Ferienzeitraum die Plätze nach Kriterien? Dann bekommt beim Anmelden niemand sofort einen Platz. Bei der Woche steht dann zum Beispiel `3 offen`. Die Plätze vergibt Ihr Büro im Reiter `Vergabe`. Danach gelten Warteliste und Nachrücken wie hier beschrieben.",
      "Bei einer Woche steht `Voll`? Alle Plätze sind vergeben. Neue Anmeldungen für diese Woche kommen auf die Warteliste.",
      "Bei einer Woche steht `2 Tage voll` oder `Alle Tage voll`? Die Woche wird tageweise gebucht. An diesen Tagen sind alle Plätze vergeben.",
      "Bei einer Woche steht `Ohne Grenze`? Für die Woche gibt es keine Grenze.",
    ],
    troubleshootingDetails: [
      "Die Warteliste rückt nicht nach, obwohl Sie mehr Plätze eingetragen haben? Laden Sie die Seite neu. Steht das Kind noch auf der Warteliste, ist die Zahl nicht höher als die vergebenen Plätze.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayRegistrationReview,
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayContribution,
      HELP_TOPICS.traegerHolidayAllocation,
    ],
  };
}
