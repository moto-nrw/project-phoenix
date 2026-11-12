import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerBillingRunTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerBillingRun,
    title: "Ferien abrechnen",
    question: "Wie stelle ich die Rechnungen für die Ferien aus?",
    summary:
      "Ein Abrechnungslauf erstellt die Rechnungen für einen Ferienzeitraum. Sie prüfen die Vorschau, korrigieren einzelne Beträge, geben den Lauf frei und erstellen die Schreiben an die Familien. Eine ausgestellte Rechnung stornieren Sie oder schreiben einen Betrag gut. Unter `Familien` sehen Sie je Familie alle Rechnungen und die offene Forderung vor Zahlungseingang.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "FileText",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Abrechnung`.",
      "Der Ferienzeitraum ist veröffentlicht, und die Anmeldungen sind bestätigt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Rechnungsnummern festlegen",
        description: "Das machen Sie einmal vor der ersten Freigabe.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Abrechnungsvorgaben`.",
          "Wählen Sie bei `Rechnungsnummern der Ferien` den Knopf `Muster festlegen`.",
          "Tragen Sie das Muster ein, zum Beispiel FE-{JJJJ}-{NNNN}. {NNNN} ist die laufende Nummer, {JJJJ} das Jahr.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Lauf starten und prüfen",
        steps: [
          "Wählen Sie `Lauf starten`.",
          "Wählen Sie den Ferienzeitraum und dann `Lauf starten`.",
          "Prüfen Sie die Vorschau unter `Rechnungen`. Je Familie steht dort eine Rechnung.",
          "Die Positionen einer Rechnung sehen Sie im Menü mit den drei Punkten unter `Aufschlüsselung ansehen`.",
        ],
      },
      {
        title: "Vom Ferienzeitraum zur Abrechnung",
        description:
          "Sie bearbeiten gerade einen Ferienzeitraum und wollen zu seinen Rechnungen.",
        steps: [
          "Öffnen Sie den Ferienzeitraum unter `Ferienbetreuung`.",
          "Wählen Sie oben rechts `Zur Abrechnung`. Der Knopf steht bei `Planung`, `Formular` und `Anmeldungen`.",
          "Hat der Zeitraum genau einen Lauf, öffnet sich dieser Lauf. Sonst sehen Sie die Abrechnung nur mit den Läufen dieses Zeitraums.",
          "Den Filter `Ferienzeitraum` entfernen Sie mit dem Kreuz. Dann sehen Sie wieder alle Läufe.",
          "Hat der Zeitraum noch keinen Lauf, wählen Sie `Lauf starten`. Der Zeitraum ist schon gewählt.",
          "Im Lauf führt `Zum Ferienzeitraum` unter dem Titel zurück zur Planung.",
        ],
      },
      {
        title: "Einen Betrag korrigieren",
        steps: [
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten.",
          "Wählen Sie `Betrag für … korrigieren`.",
          "Tragen Sie den neuen Betrag und eine Begründung ein.",
          "Wählen Sie `Korrigieren`.",
        ],
      },
      {
        title: "Lauf freigeben",
        steps: [
          "Wählen Sie `Lauf freigeben`.",
          "Prüfen Sie Zahl und Summe der Rechnungen und wählen Sie `Freigeben`.",
        ],
      },
      {
        title: "Leeren Lauf verwerfen",
        description:
          "Hat ein offener Lauf nichts abzurechnen, lässt er sich nicht freigeben. Das passiert zum Beispiel, wenn alle Anmeldungen storniert sind. Solange er offen ist, starten Sie für den Ferienzeitraum keinen neuen Lauf. Dann verwerfen Sie ihn.",
        steps: [
          "Öffnen Sie den Lauf. Steht dort `Dieser Lauf hat nichts abzurechnen`, wählen Sie oben `Leeren Lauf verwerfen`.",
          "Wählen Sie im Fenster `Verwerfen`.",
          "Der Lauf steht jetzt als `Verworfen` in der Liste. Er hat nichts ausgestellt. Wann er verworfen wurde, steht unter dem Titel.",
          "Der Ferienzeitraum ist danach frei. Kommen neue Anmeldungen dazu, wählen Sie im verworfenen Lauf `Neuen Lauf starten`.",
        ],
      },
      {
        title: "Bestätigungsschreiben erstellen",
        description:
          "Jede Familie bekommt ein Schreiben im Briefkopf des Trägers. Es bestätigt die Anmeldung und ist zugleich die Rechnung.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Bestätigungsschreiben` den Knopf `Schreiben erstellen`.",
          "Prüfen Sie die `Fälligkeiten`. Sie stehen auf den Rechnungen fest. Nur ältere Rechnungen haben noch das Feld `Zahlen bis`.",
          "Schreiben Sie bei `Hinweise für die Eltern` zum Beispiel die Packliste und was bei Krankheit gilt.",
          "Tragen Sie bei `Notfallnummern der Betreuung` je Betreuung den Namen und die Telefonnummer ein.",
          "Wählen Sie `Vorschau ansehen`. Sie sehen das Schreiben der ersten Familie.",
          "Wählen Sie `Schreiben erstellen`.",
          "Das Schreiben einer Familie laden Sie über das Menü mit den drei Punkten bei ihrer Rechnung herunter: `Schreiben herunterladen`.",
        ],
      },
      {
        title: "Schreiben einer Rechnung ansehen",
        description:
          "Das geht, sobald die Schreiben erstellt sind. Sie sehen genau das Schreiben, das die Familie bekommt.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Wählen Sie in der Karte `Rechnungen` eine Zeile. Rechts neben der Liste steht das Schreiben dieser Rechnung als PDF. Über dem Schreiben stehen Nummer, Empfänger und Betrag.",
          "Wählen Sie eine andere Zeile, um deren Schreiben zu sehen.",
          "Mit `Herunterladen` speichern Sie genau dieses Schreiben. Mit `In neuem Tab öffnen` sehen Sie es groß.",
        ],
      },
      {
        title: "Schreiben per Mail senden und drucken",
        description:
          "Jede Familie bekommt ihr Schreiben als PDF im Anhang einer Mail. Hat die Familie ein Konto in der Eltern-App mit derselben Mail-Adresse, liegt es auch dort unter `Dokumente`. Familien ohne E-Mail geben Sie das gedruckte Schreiben.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Versand per Mail` den Knopf `An alle Familien senden`. moto schickt nur Mails, die noch nicht gesendet sind.",
          "Neben jeder Familie steht, was aus der Mail geworden ist. `Gesendet` heißt: Die Mail ist raus. `Zugestellt` heißt: Sie ist angekommen. `Nicht angekommen` und `Nicht zustellbar` heißen: Die Mail kam nicht an.",
          "Bei `Verzögert` kommt die Mail später an. Warten Sie dann. `Erneut senden` steht erst danach wieder bereit. Spätestens bei `Überfällig` können Sie neu senden.",
          "Bei `Nicht gesendet` und `Abgemeldet` zählt, ob die Sperre noch besteht. Besteht sie, fehlt `Erneut senden`, und der Hinweis sagt, was zu tun ist. Drucken Sie das Schreiben dann und geben Sie es der Familie. Ist sie weg, können Sie neu senden.",
          "Wählen Sie bei dieser Familie `Erneut senden`. Mit `Senden` schicken Sie das Schreiben an eine einzelne Familie.",
          "Wählen Sie `Alle Schreiben drucken`. Sie bekommen alle Schreiben des Laufs in einer Datei.",
        ],
      },
      {
        title: "Versand und Exporte einer Rechnung ansehen",
        description:
          "Das geht, sobald der Lauf freigegeben ist. Sie sehen, was mit einer Rechnung geschah. Ändern können Sie dort nichts.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten.",
          "Wählen Sie `Versand und Exporte ansehen`.",
          "Jeder Schritt steht mit Zeit und Konto da: Ausstellung, Versand, Storno und Lastschrift.",
          "Jede Datei für Kämmerei, Steuerbüro oder Bank steht mit Nummer und Dateiname da.",
        ],
      },
      {
        title: "Rechnung stornieren und neu ausstellen",
        description:
          "Eine ausgestellte Rechnung ändert sich nie. Stimmt sie nicht, stornieren Sie sie. Der Storno ist ein eigenes Dokument mit eigener Nummer und eigenem Schreiben.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten.",
          "Wählen Sie `Rechnung stornieren`.",
          "Schreiben Sie eine Begründung, zum Beispiel: Anschrift war falsch. Neue Rechnung folgt.",
          "Wählen Sie `Stornieren`.",
          "Wählen Sie im Menü der Rechnung `Aufschlüsselung ansehen` und dann `Neu ausstellen`.",
          "moto öffnet einen Lauf in der Vorschau. Dort steht die Familie mit dem Hinweis `Ersetzt` und der alten Nummer.",
          "Hatte die alte Rechnung Korrekturen, stehen sie bei `Übernommene Korrekturen`. Sie zählen schon in der neuen Rechnung mit. Prüfen Sie jede davon.",
          "Stimmt eine Korrektur nicht mehr, wählen Sie bei ihr das Menü mit den drei Punkten und dann `Ändern` oder `Entfernen`.",
          "Prüfen Sie die Rechnung und wählen Sie `Lauf freigeben`. Die neue Rechnung bekommt eine neue Nummer.",
        ],
      },
      {
        title: "Einen Betrag gutschreiben",
        description:
          "Mit einer Gutschrift wird ein Teil der Rechnung erlassen, zum Beispiel für einen Krankheitstag. Sie ist ein eigenes Dokument mit eigener Nummer und eigenem Schreiben.",
        steps: [
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten.",
          "Wählen Sie `Gutschrift für …` beim Kind. Oben steht die offene Forderung für das Kind. Das ist der Stand vor Zahlungseingang.",
          "Tragen Sie den Betrag und eine Begründung ein.",
          "Wählen Sie `Gutschreiben`.",
        ],
      },
      {
        title: "Übersicht für die Kämmerei herunterladen",
        description: "Das geht, sobald der Lauf freigegeben ist.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Wählen Sie oben in der Karte `Rechnungen` das Menü mit den drei Punkten.",
          "Wählen Sie unter `Exportieren` das Format: `PDF`, `Excel` oder `CSV`.",
          "Geben Sie die Datei an die Kämmerei weiter.",
        ],
      },
      {
        title: "Liste für die Erstattung herunterladen",
        description:
          "Das geht, sobald der Lauf freigegeben ist und ein Kostenträger Beiträge übernimmt.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Die Karte `Kostenübernahme` zeigt je Kostenträger die Zahl der Rechnungen und den Betrag.",
          "Wählen Sie in der Zeile des Kostenträgers das Menü mit den drei Punkten.",
          "Wählen Sie unter `Liste für die Erstattung` das Format: `PDF`, `Excel` oder `CSV`.",
          "Schicken Sie die Liste an den Kostenträger. Sie nennt je Kind die Wochen, den Grundbetrag, die Geschwisterermäßigung, den übernommenen Betrag, ob ein Nachweis vorlag und den Tag der Bestätigung.",
        ],
      },
      {
        title: "Rechnungen und Beträge einer Familie ansehen",
        description:
          "Hier sehen Sie alle Rechnungen, Stornos und Gutschriften einer Familie über alle Läufe und ihre Debitorennummer. Eine Familie ist eine E-Mail-Adresse. Familien aus der Ferienbetreuung stehen hier schon, wenn ihre Anmeldung mit Debitorennummer bestätigt ist.",
        steps: [
          "Wählen Sie in der Navigation `Abrechnung`.",
          "Wählen Sie oben den Reiter `Familien`.",
          "Die Kacheln zeigen die Summen `Ausgestellt`, `Storniert`, `Gutgeschrieben` und `Offene Forderung`. Sie rechnen mit den Familien, die die Liste gerade zeigt.",
          "Suchen Sie die Familie nach Name, E-Mail oder Debitorennummer. Mit dem Filter `Mit offener Forderung` sehen Sie nur Familien, deren Rechnungen noch Geld verlangen. Mit `Ohne Debitorennummer` sehen Sie Familien ohne Nummer. Denselben Filter setzt ein Klick auf `ohne Debitorennummer` oben in der Zeile mit den Zahlen.",
          "Unter `Debitorennummer` steht die Nummer der Familie. Steht auf älteren Rechnungen eine andere, sehen Sie sie darunter. `Nur Ferien` heißt: Die Familie hat keinen Betreuungsvertrag.",
          "Öffnen Sie das Drei-Punkte-Menü neben der Nummer. Wählen Sie `Debitorennummer ändern` oder `Debitorennummer eintragen`. Tragen Sie die Nummer ein und wählen Sie `Speichern`. Ausgestellte Rechnungen behalten ihre Nummer.",
          "Wählen Sie die Familie. Sie sehen ihre Summen, wer zahlt, den Zahlungsweg und jedes Dokument mit Ferienzeitraum, Kindern, Betrag und Stand. Eine Familie ohne Rechnung öffnet sich nicht.",
          "Wählen Sie ein Dokument, um seinen Abrechnungslauf zu öffnen. Dort stornieren Sie oder schreiben gut.",
        ],
      },
    ],
    result:
      "Jede Rechnung hat ihre Nummer. Die Nummern sind lückenlos. Ausgestellte Rechnungen und ihre Schreiben lassen sich nicht mehr ändern. Die Übersicht zeigt jede Position je Familie, die Summe jeder Rechnung und die Gesamtsumme. Storno und Gutschrift stehen dort als eigene Zeilen mit Minusbetrag und der Nummer der Rechnung.",
    notes: [
      "Bezahlt ein Kostenträger, bekommt die Familie eine Rechnung über 0 €. Im Schreiben steht, wer die Kosten übernimmt. Die Übersicht für die Kämmerei nennt unter der Gesamtsumme je Kostenträger den Betrag, den er übernimmt.",
      "Ferien werden im Voraus abgerechnet. Grundlage ist der Beitrag, den moto beim Bestätigen der Anmeldung festgehalten hat.",
      "Jede Korrektur steht als eigene Position auf der Rechnung. Unter der Rechnung sehen Sie alle Korrekturen mit Datum und Begründung.",
      "Wird nach dem Start noch eine Anmeldung bestätigt, kommt sie bis zur Freigabe in den Lauf. Nach der Freigabe starten Sie für sie einen neuen Lauf.",
      "Logo, Farbe, Absender und Fußzeile im Schreiben kommen aus `Briefkopf und Vorlagen`. Die Ansprechperson kommt aus `Allgemein`, die Bankverbindung aus `Abrechnungsvorgaben`. Als Verwendungszweck steht im Schreiben die Debitorennummer der Rechnung. Hat die Rechnung keine, steht dort die Rechnungsnummer. Einen eigenen Zahlungstext schreiben Sie in den Einstellungen unter `Allgemein` im Bereich `Ferienbetreuung`.",
      "Die Rechnung übernimmt die Debitorennummer der Anmeldung. Hat die Anmeldung keine, nimmt sie die Nummer der Familie aus dem Reiter `Familien`. Die Nummer steht im Schreiben unter der Rechnungsnummer und in der Übersicht für die Kämmerei in der Spalte `Debitorennummer`.",
      "Storno und Gutschrift bekommen ihre Nummer aus demselben Nummernkreis wie die Rechnungen. In der Aufschlüsselung einer Rechnung steht unter `Verlauf` ihr Storno, ihre Gutschriften und die neue Rechnung.",
      "Sind die Schreiben des Laufs schon erstellt, bekommen Storno und Gutschrift ihr Schreiben sofort. Sie senden es wie die anderen Schreiben unter `Versand per Mail`.",
      "Nach einem Storno lässt sich die Debitorennummer der Anmeldung wieder ändern. Die neue Rechnung übernimmt sie.",
      "Eine übernommene Korrektur senkt den Betrag höchstens auf 0 €. Ist der Beitrag jetzt niedriger als auf der alten Rechnung, steht das bei der Korrektur.",
      "Gutschriften der alten Rechnung werden nicht übernommen. Sie sind eigene Dokumente. Die Vorschau nennt sie nur als Hinweis.",
      "Hat die Familie bei der Anmeldung ihre Anschrift angegeben, steht sie im Schreiben und in der Übersicht für die Kämmerei. Die Rechnung behält sie, auch wenn die Anmeldung später gelöscht wird.",
      "Den Text des Schreibens legen Sie in den Einstellungen unter `Briefkopf und Vorlagen` fest. Erstellte Schreiben behalten den Text, mit dem sie entstanden sind.",
      "`Offene Forderung` und `Forderung offen` heißen: Die Rechnung ist ausgestellt, nicht storniert und nicht gutgeschrieben. Das ist der Stand vor Zahlungseingang. Zahlungen sieht moto nicht. Eine offene Forderung heißt also nicht, dass die Familie im Rückstand ist. Ob sie gezahlt hat, prüfen Sie in Ihrer Buchhaltung.",
      "In der Spalte `Zahlung` steht `Familie zahlt selbst`, `Kostenübernahme` mit dem Namen des Kostenträgers oder `Beitragsfrei`, darunter der Zahlungsweg. Eine stornierte Rechnung zählt dabei nicht mit.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Abrechnung`? Ihre Rolle ist nicht Abrechnung. Fragen Sie im Träger-Büro nach dieser Rolle.",
      "Im Ferienzeitraum fehlt `Zur Abrechnung`? Der Zeitraum ist noch ein Entwurf, oder Ihre Rolle ist nicht Abrechnung.",
      "Im Lauf fehlt `Zum Ferienzeitraum`? Ihre Rolle darf die Ferienbetreuung nicht öffnen.",
      "`Lauf freigeben` lässt sich nicht wählen? Es fehlt das Muster der Rechnungsnummern, oder der Lauf hat keine Rechnung.",
      "Statt `Lauf freigeben` steht `Leeren Lauf verwerfen`? Der Lauf hat nichts abzurechnen. Verwerfen Sie ihn, dann ist der Ferienzeitraum frei.",
      "Sie sehen oben in der Karte `Rechnungen` kein Menü mit den drei Punkten? Der Lauf ist noch nicht freigegeben.",
      "Ein Klick auf eine Rechnung zeigt kein Schreiben? Die Schreiben des Laufs sind noch nicht erstellt. Erstellen Sie sie bei `Bestätigungsschreiben`.",
      "Auf dem Handy oder einem schmalen Bildschirm steht das Schreiben an der Stelle der Liste. Mit `Zurück zu den Rechnungen` kommen Sie zur Liste.",
      "Statt des Schreibens steht, dass sich die Datei auf diesem Gerät hier nicht zeigen lässt? Wählen Sie `In neuem Tab öffnen` oder `Herunterladen`.",
      "Im Menü fehlt `Versand und Exporte ansehen`? Der Lauf ist noch nicht freigegeben, oder die Zeile ist ein Storno oder eine Gutschrift. Beide stehen bei ihrer Rechnung.",
      "Im Menü einer Rechnung fehlt `Rechnung stornieren`? Die Rechnung hat schon eine Gutschrift oder ist schon storniert. Schreiben Sie dann den Rest gut.",
      "Vor der Freigabe steht, dass Anmeldungen noch keine Debitorennummer haben? Ihr Träger fragt die Nummer ab. Diese Anmeldungen wurden ohne Nummer bestätigt, und auch die Familie hat keine. Tragen Sie die Nummer in der Anmeldung ein. Freigeben können Sie trotzdem. Nennt Ihr eigener Zahlungstext die Debitorennummer, lassen sich die Schreiben ohne Nummer aber nicht erstellen.",
      "Vor der Freigabe steht `Schreiben brauchen die Debitorennummer`? Ihr eigener Zahlungstext nennt die Debitorennummer, und Rechnungen mit Betrag haben keine. Tragen Sie die Nummern vor dem Freigeben in den Anmeldungen ein. Das Feld dafür erscheint, wenn in den Einstellungen unter `Allgemein` im Bereich `Ferienbetreuung` `Debitorennummer beim Bestätigen abfragen` an ist. Freigeben geht trotzdem, der Knopf heißt dann `Trotzdem freigeben`. Die Schreiben lassen sich danach nur ohne den Platzhalter erstellen.",
    ],
    troubleshootingDetails: [
      "Die Freigabe meldet, dass sich der Lauf geändert hat? Seit Ihrer Prüfung hat sich etwas an den Rechnungen geändert. Zum Beispiel ein Betrag, eine Debitorennummer, die Fälligkeit oder eine neue bestätigte Anmeldung. Das gilt auch, wenn Zahl und Summe gleich bleiben. Prüfen Sie die neue Vorschau und geben Sie dann frei.",
      "Das Verwerfen meldet, dass der Lauf etwas abzurechnen hat? Seit dem Öffnen kam etwas dazu, zum Beispiel eine bestätigte Anmeldung. Prüfen Sie die neue Vorschau und geben Sie den Lauf frei.",
      "Eine bestätigte Anmeldung fehlt in der Vorschau? Sie hat schon eine Rechnung, oder beim Bestätigen wurde kein Beitrag festgehalten.",
      "Die Schreiben lassen sich nicht erstellen, weil eine Bankverbindung fehlt? Die Verwaltung trägt sie unter `Abrechnungsvorgaben` ein. Danach erstellen Sie die Schreiben.",
      "Die Schreiben lassen sich nicht erstellen, weil Rechnungen keine Debitorennummer haben? Ihr eigener Zahlungstext nennt die Debitorennummer. Nach der Freigabe lässt sich die Nummer nicht mehr eintragen. Löschen Sie in den Einstellungen unter `Allgemein` im Bereich `Ferienbetreuung` den eigenen Zahlungstext oder nehmen Sie den Platzhalter {Debitorennummer} heraus. Danach erstellen Sie die Schreiben.",
      "Eine Mail kommt auch nach `Erneut senden` nicht an? Vielleicht stimmt die Adresse nicht. Drucken Sie das Schreiben und geben Sie es der Familie.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayContribution,
      HELP_TOPICS.traegerHolidayRegistrationReview,
      HELP_TOPICS.traegerBillingTerms,
    ],
  };
}
