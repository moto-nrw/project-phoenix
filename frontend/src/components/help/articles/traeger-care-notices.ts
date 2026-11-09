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
        ],
      },
      {
        title: "Bescheide an die Familien senden",
        description:
          "moto speichert jeden ausgestellten Bescheid einmal als PDF. Dieses Original bleibt gleich, auch wenn sich später Briefkopf, Vorlage, Adresse oder Vertrag ändern. Die Familie bekommt den Bescheid erst, wenn Sie senden.",
        steps: [
          "Öffnen Sie den ausgestellten Jahreslauf und wählen Sie `An Familien senden`.",
          "Wählen Sie `Originale speichern`. Gesendet wird dabei noch nichts.",
          "Wählen Sie `An alle senden` und bestätigen Sie. Jede Familie mit E-Mail-Adresse bekommt das Original als Anhang.",
          "Hat die Familie mit dieser Adresse ein Konto in der Eltern-App, liegt der Bescheid dort unter `Dokumente`.",
          "Familien ohne E-Mail geben Sie den Bescheid auf Papier. Wählen Sie `Drucken` und dann `Nur Familien ohne E-Mail`.",
          "Einen einzelnen Bescheid senden Sie über das Menü seiner Zeile mit `Senden` oder `Erneut senden`.",
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
      "E-Mail und Eltern-App sind keine förmliche Zustellung. Brauchen Sie einen Nachweis, dass die Familie den Bescheid bekommen hat? Dann stellen Sie ihn zusätzlich auf anderem Weg zu.",
      "Ein Änderungsbescheid ist ein eigener Bescheid mit eigenem Original. Senden Sie ihn genauso. Der alte Bescheid bleibt unverändert.",
      "Fehlt ein Preis oder gibt es einen doppelten Vertrag, bleibt die bisherige Bewertung gültig. Prüfen Sie die Hinweise im Monatslauf.",
      "Das Essen steht nicht im Bescheid. Die Mensa rechnet es nach den Essen ab.",
      "Der `Gesamtbetrag aller Bescheide` ist die Forderung vor Zahlungseingang. Zahlungen sieht moto nicht, auch nicht nach der Lastschrift. Ob eine Rate bezahlt ist, prüfen Sie in Ihrer Buchhaltung.",
      "Ermäßigungen und Anteile von Kostenträgern stehen als eigene Zeilen im Bescheid.",
    ],
    troubleshootingDetails: [
      "Ein Bescheid geht nicht raus und es steht `Die Person zahlt nicht mehr für diesen Vertrag`: Empfänger oder Zahler haben sich nach dem Ausstellen geändert. Der Bescheid geht nur an die Adresse, die er nennt. Stellen Sie mit einem neuen Jahreslauf einen Änderungsbescheid aus.",
      "Ein Bescheid steht nicht in der Eltern-App: Das Kind hat noch keinen Schülerdatensatz, oder das Konto der Familie hat eine andere E-Mail-Adresse. Die Familie bekommt ihn per E-Mail oder auf Papier.",
      "`Bescheide ausstellen` geht nicht und oben steht `Rechtsbehelfsbelehrung fehlt`: Tragen Sie die Belehrung in der Vorlage `Bescheid` ein. Dafür brauchen Sie das Recht `Einstellungen`.",
      "Ein Jahreslauf lässt sich nicht starten: Für dieses Jahr gibt es schon einen offenen Lauf. Öffnen Sie ihn in der Liste. Stellen Sie die Bescheide aus oder schließen Sie den leeren Lauf ab.",
      "Die Vorschau ist leer: Es gibt nichts Neues auszustellen. Wählen Sie `Jahreslauf abschließen` und dann `Abschließen`. Danach können Sie einen neuen Jahreslauf für andere Schulen starten.",
      "Im Monatslauf steht `Durch aufgehobenen Bescheid gesperrt`: Ein Änderungsbescheid hat den Monat auf 0,00 € gesetzt. Der Monat bleibt gesperrt, damit ihn niemand doppelt abrechnet. Wollen Sie ihn abrechnen, stellen Sie mit `Neuen Bescheid ausstellen` einen neuen Bescheid aus.",
    ],
    related: [
      HELP_TOPICS.traegerCareMonths,
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerDocumentKinds,
    ],
  };
}
