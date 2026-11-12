import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/schliesstage` (#4702): Schließtage des Trägers und Feiertage je Bundesland. */
export function traegerClosingDaysTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerClosingDays,
    title: "Schließtage für Schulen planen",
    question: "Wie plane ich einen Brückentag für mehrere Schulen?",
    summary:
      "Ein Schließtag gilt für die Schulen, die Sie wählen. An diesem Tag ist die OGS zu. Die Feiertage kommen aus dem Bundesland der Schule.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarSlash",
    requirements: [
      "Sehen kann die Seite jede Rolle im Träger-Büro.",
      "Schließtage anlegen darf Ihre Rolle, wenn sie Einstellungen verwalten darf. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Schließtag anlegen",
        steps: [
          "Wählen Sie in der Navigation `Schließtage`.",
          "Wählen Sie `Schließtag anlegen`.",
          "Tragen Sie den `Grund` ein, zum Beispiel Brückentag.",
          "Tragen Sie `Von` und `Bis` ein. Für einen Tag tragen Sie zweimal dasselbe Datum ein.",
          "Wählen Sie bei `Gilt für` die Schulen. `Alle Schulen` wählt alle auf einmal.",
          "Prüfen Sie unter `Ferienbetreuung an diesen Tagen`, welche gebuchten Ferienwochen betroffen sind.",
          "Wählen Sie `Speichern`. Sind Ferienwochen betroffen, heißt der Knopf `Speichern und Eltern informieren`.",
        ],
      },
      {
        title: "Schließtag ändern oder löschen",
        steps: [
          "Wählen Sie beim Schließtag die drei Punkte.",
          "Wählen Sie `Ändern` oder `Löschen`.",
          "Unter `Ferienbetreuung an diesen Tagen` steht, welche Tage wieder geöffnet sind und wie viele Familien eine E-Mail bekommen.",
          "Wählen Sie `Speichern` oder beim Löschen `Ja, löschen` und dann `Endgültig löschen`. Die Eltern bekommen die E-Mail sofort.",
        ],
      },
      {
        title: "Eltern zu einem Schließtag der Schule informieren",
        description:
          "Trägt eine Schule selbst einen Schließtag in eine gebuchte Ferienwoche ein, bekommen die Eltern nicht von selbst eine E-Mail. Das gilt auch, wenn die Schule ihn ändert oder löscht. Sie oder die Schule lösen den Versand aus.",
        steps: [
          "Wählen Sie in der Navigation `Schließtage`.",
          "Oben steht dann die Karte `Eltern noch nicht informiert`. Je Schließtag stehen dort Schule, Grund, Tage, Kinder und Familien.",
          "Wählen Sie in der Zeile `Eltern informieren`. Ein Klick genügt.",
          "Mehrere Schließtage auf einmal: Wählen Sie oben `Alle Eltern informieren`.",
          "Die Zeile zeigt `Wird gesendet` und dann `Gesendet`.",
        ],
      },
      {
        title: "Feiertage prüfen",
        steps: [
          "Wählen Sie in der Navigation `Schließtage`.",
          "Oben wählen Sie das Jahr.",
          "In der `Jahresübersicht` sind Feiertage blau und Schließtage orange.",
          "Unter `Feiertage nach Bundesland` stehen die Feiertage und die Schulen dazu.",
        ],
      },
    ],
    result:
      "Die gewählten Schulen haben an diesem Tag zu. Ferienwochen mit diesem Tag sind nicht mehr buchbar. Termine fallen aus, und Mitarbeitende haben kein Soll. Eltern mit einer Buchung in der Woche bekommen eine E-Mail mit den geschlossenen Tagen.",
    notes: [
      "Die Schulen sehen den Schließtag in ihrem Kalender mit `Vom Träger`. Ändern kann ihn nur das Träger-Büro.",
      "Der Elternbeitrag ändert sich durch einen Schließtag nicht, auch nicht bei tageweise gebuchten Wochen. Das steht auch in der E-Mail an die Eltern.",
      "Jede Familie bekommt eine E-Mail, auch wenn mehrere Kinder oder Wochen betroffen sind. Speichern Sie den Schließtag noch einmal, bekommt niemand eine zweite E-Mail.",
      "Das Bundesland einer Schule legt das moto-Team fest. Stimmt es nicht, schreiben Sie dem moto-Team.",
      "Bisherige Buchungen einer Ferienwoche bleiben, auch wenn Sie später einen Schließtag in die Woche legen. Niemand verliert seinen Platz, und niemand rückt von der Warteliste nach. Neue Buchungen der Woche sind dann nicht möglich.",
    ],
    differences: [
      "Sie sehen keinen Knopf `Schließtag anlegen`? Ihre Rolle darf keine Einstellungen verwalten. Fragen Sie im Träger-Büro nach der Rolle Verwaltung.",
    ],
    troubleshootingDetails: [
      "Beim Speichern steht, die gebuchten Ferientage haben sich geändert? Inzwischen hat jemand gebucht oder storniert. Prüfen Sie die neue Liste im Dialog und speichern Sie noch einmal.",
      "Auch ohne Schulhinweis können E-Mails ausfallen. Die Karte `Eltern noch nicht informiert` zeigt ihre Zahl. Wählen Sie `Erneut senden`. Bereits angenommene E-Mails werden nicht wiederholt.",
      "Eine Zeile zeigt `Nicht angekommen`? Wählen Sie noch einmal `Eltern informieren`. Familien, die schon eine E-Mail haben, bekommen keine zweite.",
      "Eine Zeile zeigt `Ergebnis unbekannt`? Schreiben Sie dem moto-Team. Es prüft den Versand.",
      "Nach dem Klick steht, der Schließtag wurde geändert? Die Schule hat ihn inzwischen geändert. Prüfen Sie die Zeile und wählen Sie noch einmal `Eltern informieren`.",
    ],
    related: [HELP_TOPICS.traegerHolidayPeriod, HELP_TOPICS.traegerSchoolView],
  };
}
