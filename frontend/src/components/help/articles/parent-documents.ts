import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/parents/documents` (#4671, #4678, #4679, #4692): Schreiben und Verträge
 * des Trägers, den Betreuungsvertrag unterschreiben, und Dokumente, die der
 * Träger für Eltern freigegeben hat; die Kündigung eines Vertrags (#4682);
 * die Nachricht `Neues Dokument vom Träger` für alle drei Arten (#5941); das
 * Lastschrift-Mandat erteilen und widerrufen (#4696).
 */
export function parentDocumentsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDocuments,
    title: "Dokumente vom Träger finden",
    question: "Wo finde ich Schreiben vom Träger?",
    summary:
      "Unter `Dokumente` liegen die Schreiben und Verträge, die Ihnen der Träger geschickt hat. Dort finden Sie auch Dokumente, die der Träger für Eltern freigegeben hat, etwa eine Hausordnung.",
    group: "nachrichten",
    audience: "parent",
    icon: "FileText",
    instructionGroups: [
      {
        title: "Ein Schreiben laden",
        steps: [
          "Tippen Sie unten auf `Mehr`.",
          "Wählen Sie `Dokumente`.",
          "Wählen Sie beim Schreiben `Als PDF laden`. Bei einem freigegebenen Dokument wählen Sie `Herunterladen`.",
        ],
      },
      {
        title: "Den Betreuungsvertrag unterschreiben",
        description:
          "Ein Vertrag mit `Wartet auf Ihre Unterschrift` braucht Ihre Unterschrift. Das geht auch über den Link in der Mail, ohne Konto.",
        steps: [
          "Wählen Sie beim Vertrag `Lesen und unterschreiben`.",
          "Lesen Sie die gebuchte Betreuung und den Vertrag als PDF.",
          "Lesen Sie die Widerrufsbelehrung. Setzen Sie den Haken bei `Ich habe die Widerrufsbelehrung gelesen.`",
          "Soll die Betreuung schon vor dem Ende der Widerrufsfrist beginnen? Setzen Sie auch diesen Haken.",
          "Unterschreiben Sie mit dem Finger oder tippen Sie Ihren Namen. Wählen Sie `Unterschrift bestätigen`.",
          "Wählen Sie `Vertrag verbindlich unterschreiben`.",
        ],
      },
      {
        title: "Kinder in der Familie angeben",
        description:
          "Für die Geschwisterermäßigung zählt der Träger alle Kinder Ihrer Familie. Auch Kinder ohne Betreuungsvertrag zählen mit.",
        steps: [
          "Wählen Sie beim Vertrag `Lesen und unterschreiben` oder `Kinder in der Familie angeben`. Ohne Konto öffnen Sie den Link aus der Vertragsmail.",
          "Gehen Sie zur Karte `Kinder in Ihrer Familie`.",
          "Stellen Sie mit Minus und Plus ein, wie viele Kinder Ihre Familie hat.",
          "Wählen Sie `Angabe senden`.",
        ],
      },
      {
        title: "Lastschrift-Mandat erteilen",
        description:
          "Mit dem Mandat erlauben Sie Ihrem Träger, Beiträge abzubuchen. Wie Sie bezahlen, steht auf Ihrer Rechnung. Das geht nur, wenn Ihr Träger es in der App anbietet. Sonst fehlt der Knopf `Lastschrift`.",
        steps: [
          "Wählen Sie beim Vertrag `Lastschrift`.",
          "Prüfen Sie den `Kontoinhaber`. Tragen Sie die `IBAN` und Ihre Anschrift ein.",
          "Wählen Sie `Weiter`.",
          "Lesen Sie den Text des Mandats.",
          "Setzen Sie den Haken bei `Ich bin Inhaberin oder Inhaber dieses Kontos.`",
          "Wählen Sie `Mandat erteilen`.",
        ],
      },
    ],
    steps: [],
    result:
      "Sie bekommen das Dokument auf Ihr Gerät. Sie können es speichern oder drucken. Nach der Unterschrift steht beim Vertrag `Unterschrieben`, und Sie bekommen eine Bestätigung mit dem Vertrag per Mail.",
    notes: [
      "Das sind die Bestätigungsschreiben der Ferienbetreuung und die Betreuungsverträge. Das Bestätigungsschreiben ist auch Ihre Rechnung.",
      "Beim Betreuungsvertrag stehen die gebuchte Betreuung und der Beginn auf der Karte. Prüfen Sie im PDF jeden Wochentag und den Beitrag. Stimmt etwas nicht, melden Sie sich beim Träger.",
      "Steht beim Vertrag `Abgeschlossen` statt `Unterschrieben`? Dann gilt er seit der Annahme Ihrer Anmeldung. Sie müssen nichts unterschreiben. Heben Sie ihn gut auf.",
      "Sie möchten den Vertrag beenden? Wählen Sie beim Vertrag `Verträge hier kündigen`. Wählen Sie den Termin und dann `Jetzt kündigen`. Solange Sie den Vertrag widerrufen können, steht beim Vertrag auch `Vertrag widerrufen`. Wählen Sie dann `Widerruf bestätigen`. Sie bekommen sofort eine Bestätigung per Mail und auf dem Bildschirm, mit Datum und Uhrzeit des Eingangs. Ändern sich die Termine nach Mitternacht? Wählen Sie dann erneut.",
      "Haben Sie einen wichtigen Grund, den Vertrag früher zu beenden? Wählen Sie `Außerordentlich kündigen` und schreiben Sie Ihren Grund auf. Der Träger prüft ihn. Erkennt er ihn an, legt er das Vertragsende fest. Erkennt er ihn nicht an, gilt Ihre Kündigung als ordentliche Kündigung zum nächsten möglichen Termin. Die Entscheidung bekommen Sie per Mail. Sie steht auch beim Vertrag unter `Mitteilung des Trägers zum Vertragsende`. Bis dahin findet die Betreuung wie gebucht statt.",
      "Bei einem gekündigten Vertrag steht `Gekündigt zum` und das Datum. Bis dahin findet die Betreuung wie gebucht statt. Mit `Bestätigung ansehen` öffnen Sie Ihre Bestätigung.",
      "Sie möchten andere Tage oder Zeiten? Beantragen Sie das bei Ihrem Kind mit `Betreuung ändern`. Bestätigt die Schule, bekommt Ihr Vertrag eine neue Version.",
      "Sie dürfen Anmeldungen nur ansehen? Wählen Sie beim Vertrag `Kinder in Ihrer Familie`. Dort sehen Sie die Angabe und die bestätigte Zahl. Sie können die Zahl nicht ändern.",
      "Über den Link aus der Vertragsmail ändern Sie die Kinderzahl bis zur Unterschrift. Danach geht das in der Eltern-App unter `Dokumente`.",
      "Ihre Angabe zur Kinderzahl prüft der Träger. Bis dahin steht `Wird geprüft`. Hat er sie bestätigt, steht `Bestätigt`. Erst dann zählt sie für die Ermäßigung.",
      "Nach dem Mandat steht beim Vertrag `Lastschrift erteilt`. Unter `Lastschrift` sehen Sie Ihr Konto, die Mandatsreferenz und den Text. Mit `Als Datei speichern` legen Sie den Text bei Ihren Unterlagen ab.",
      "Sie möchten ein anderes Konto nutzen oder keine Lastschrift mehr? Wählen Sie unter `Lastschrift` den Knopf `Mandat widerrufen`. Ab der nächsten Rechnung bucht Ihr Träger nichts mehr ab. Danach können Sie ein neues Mandat erteilen.",
      "Jedes Schreiben bekommen Sie auch per Mail.",
      "Bei einem freigegebenen Dokument steht `Vom Träger für Eltern freigegeben`. Es gilt für alle Eltern, für die der Träger es freigegeben hat, zum Beispiel für eine Gruppe. Diese Dokumente kommen nicht per Mail.",
      "Sie möchten eine Nachricht auf dem Handy bekommen, wenn ein neues Dokument da ist? Schalten Sie unter `Einstellungen` den Punkt `Neues Dokument vom Träger` ein. Das gilt für Schreiben, Verträge und freigegebene Dokumente.",
      "Die Nachricht nennt kein Kind und keinen Betrag. Was gekommen ist, sehen Sie unter `Dokumente`.",
    ],
    troubleshootingDetails: [
      "Ein Schreiben fehlt? Hier liegt es nur, wenn es an die Mail-Adresse Ihres Kontos ging. Haben Sie bei der Anmeldung eine andere Adresse angegeben, bekommen Sie es nur per Mail.",
      "Statt der Liste steht, dass Ihnen die Berechtigung fehlt? Bitte fragen Sie die Schule.",
      "Der Link in der Mail gilt nicht mehr? Er gilt nur einmal und 30 Tage. Unterschreiben Sie in der Eltern-App unter `Dokumente`. Oder fragen Sie beim Träger nach einem neuen Link.",
      "Beim Vertrag fehlt `Lesen und unterschreiben`? Dann darf Ihr Konto für dieses Kind nicht unterschreiben. Bitte fragen Sie die Schule.",
      "Ein freigegebenes Dokument ist verschwunden? Der Träger hat die Freigabe zurückgenommen oder das Dokument gelöscht.",
      "Das Dokument ist da, aber es kam keine Nachricht? Prüfen Sie unter `Einstellungen` den Punkt `Neues Dokument vom Träger`. Schickt der Träger dasselbe Dokument noch einmal, kommt keine zweite Nachricht.",
      "Unter `Lastschrift` steht `Lastschrift hier nicht möglich`? Ihr Träger nimmt das Mandat nicht in der App an. Fragen Sie ihn nach einem Formular auf Papier.",
      "Unter `Lastschrift` steht `Neues Mandat nötig`? Mit diesem Mandat ist keine weitere Abbuchung möglich. Widerrufen Sie es und erteilen Sie ein neues. Ohne dieses Recht fragen Sie die Schule.",
      "Sie sehen das Mandat, aber `Mandat widerrufen` fehlt? Das Mandat erteilen und widerrufen nur Eltern, die das Kind auch anmelden dürfen. Das Recht vergibt die Schule.",
      "`Verträge hier kündigen` fehlt? Der Vertrag gilt noch nicht, oder er ist schon beendet. Oder Ihnen fehlt das Recht: Kündigen dürfen nur Eltern, die das Kind auch anmelden dürfen. Dann steht beim Vertrag ein Hinweis. Das Recht vergibt die Schule.",
    ],
    related: [
      HELP_TOPICS.parentCareChange,
      HELP_TOPICS.parentHolidayRegistration,
      HELP_TOPICS.parentNotifications,
    ],
  };
}
