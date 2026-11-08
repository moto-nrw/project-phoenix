import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareNoticesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareNotices,
    title: "Jahresbescheide ausstellen",
    question: "Wie stelle ich einen Gebührenbescheid für das Jahr aus?",
    summary:
      "Ein Jahresbescheid setzt den Beitrag für mehrere Monate eines Jahres fest. Jede Familie bekommt je Vertrag einen Bescheid mit Monatsraten. Die Raten sind jeweils am 1. des Monats fällig. Für Monate mit Bescheid berechnet der Monatslauf keine Betreuung erneut. Essen rechnet er weiterhin ab.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarDays",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Die Preisliste hat Preise für die Monate. Der Bescheid rechnet wie der Monatslauf.",
      "Die Vorlage `Bescheid` hat eine Rechtsbehelfsbelehrung. Ohne sie stellt moto keinen Bescheid aus.",
      "Das Muster der Rechnungsnummern für Monatsbeiträge ist festgelegt. Der Bescheid nimmt seine Nummer daraus.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Lauf starten",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Jahresbescheide`.",
          "Wählen Sie `Jahreslauf starten`.",
          "Wählen Sie bei `Ab Monat` den ersten Monat, zum Beispiel September. Der Bescheid gilt bis Dezember.",
          "Wählen Sie bei `Schulen` die Schulen. Alle sind schon gewählt.",
          "Wählen Sie bei `Bescheiddatum` das Datum auf dem Bescheid. Lassen Sie das Feld leer, gilt der Tag der Ausstellung.",
          "Wählen Sie `Vorschau öffnen`.",
        ],
      },
      {
        title: "Vorschau prüfen",
        description:
          "Vor der Ausstellung ändert sich nichts. Die Vorschau zeigt immer den Stand von heute.",
        steps: [
          "Je Kind steht eine Zeile mit Zeitraum, Monatsrate und Jahresbetrag.",
          "Wählen Sie eine Zeile. Sie sehen Seite 1 des Bescheids so, wie die Familie ihn bekommt.",
          "Unter `I. Berechnung` steht je Modul der Betrag im Monat und für den Zeitraum.",
          "Unter `II. Abrechnung` stehen die Raten. Ein Monat, der schon begonnen hat, ist sofort fällig.",
        ],
      },
      {
        title: "Bescheide ausstellen",
        steps: [
          "Wählen Sie `Bescheide ausstellen`.",
          "Prüfen Sie Zahl und Summe und wählen Sie `Ausstellen`.",
          "Jeder Bescheid bekommt seine Nummer. Danach ändert er sich nicht mehr.",
          "Hat sich seit Ihrer Prüfung etwas geändert, stellt moto nichts aus. Die Seite zeigt den neuen Stand. Prüfen Sie ihn und stellen Sie dann aus.",
          "Wählen Sie bei einem Bescheid `Drucken`, um ihn auszudrucken.",
        ],
      },
      {
        title: "Vertrag geändert: Änderungsbescheid",
        description:
          "Ein ausgestellter Bescheid ändert sich nie. Ändert sich ein Vertrag, gibt es einen Änderungsbescheid.",
        steps: [
          "Starten Sie für dasselbe Jahr einen neuen Jahreslauf, zum Beispiel ab dem Monat der Änderung.",
          "Die Vorschau zeigt nur Kinder, bei denen sich etwas geändert hat. Sie stehen als `Änderungsbescheid`.",
          "Der Änderungsbescheid nennt den alten Bescheid. Je Zeile steht `bisher`, `neu` und `Zu-/Abgang`.",
          "Stellen Sie die Bescheide aus. Von da an gilt für diese Monate der Änderungsbescheid.",
        ],
      },
    ],
    result:
      "Jede Familie hat ihren Bescheid mit den Raten bis Dezember. Für diese Monate berechnet der Monatslauf keine Betreuung erneut. Er zählt die Kinder unter `Schon abgerechnet`.",
    differences: [
      "Hat ein Kind für einen Monat schon eine Monatsrechnung, nimmt der Bescheid diesen Monat nicht auf. Er beginnt mit dem nächsten Monat.",
      "Endet ein Vertrag, setzt der Änderungsbescheid die Monate danach auf 0,00 €. Das ist ein Abgang.",
    ],
    notes: [
      "Fehlt ein Preis oder gibt es einen doppelten Vertrag, bleibt die bisherige Bewertung gültig. Prüfen Sie die Hinweise im Monatslauf.",
      "Das Essen steht nicht im Bescheid. Die Mensa rechnet es nach den Essen ab.",
      "Ermäßigungen und Anteile von Kostenträgern stehen als eigene Zeilen im Bescheid.",
    ],
    troubleshootingDetails: [
      "`Bescheide ausstellen` geht nicht und oben steht `Rechtsbehelfsbelehrung fehlt`: Tragen Sie die Belehrung in der Vorlage `Bescheid` ein. Dafür brauchen Sie das Recht `Einstellungen`.",
      "Ein Jahreslauf lässt sich nicht starten: Für dieses Jahr gibt es schon einen offenen Lauf. Öffnen Sie ihn in der Liste. Stellen Sie die Bescheide aus oder schließen Sie den leeren Lauf ab.",
      "Die Vorschau ist leer: Es gibt nichts Neues auszustellen. Wählen Sie `Jahreslauf abschließen` und dann `Abschließen`. Danach können Sie einen neuen Jahreslauf für andere Schulen starten.",
    ],
    related: [
      HELP_TOPICS.traegerCareMonths,
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerDocumentKinds,
    ],
  };
}
