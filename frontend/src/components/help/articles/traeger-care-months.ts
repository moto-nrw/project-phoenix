import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareMonthsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareMonths,
    title: "Monatsbeiträge abrechnen",
    question: "Wie rechne ich die Betreuung jeden Monat ab?",
    summary:
      "Ein Monatslauf rechnet einen Monat für die gewählten Schulen nach den gültigen Verträgen ab. Der Beitrag eines Kindes kommt aus der Preisliste: als Summe der gebuchten Wochentage oder als Staffel nach Tagen je Woche. Es zählt der Vertrag, nicht die Anwesenheit. Vor der Freigabe prüfen Sie die Vorschau, ändern einzelne Rechnungen und schließen Rechnungen aus, die noch nicht stimmen. Nach der Freigabe erstellen Sie die Schreiben, senden sie an die Familien und stornieren eine falsche Rechnung.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "CalendarCheck",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Abrechnung`.",
      "Die Kinder haben einen gültigen Betreuungsvertrag. Module und gebuchte Wochentage kommen aus dem Vertrag.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Preise festlegen",
        description:
          "Jede Schule hat eigene Preise. Jedes Modul hat eine Preisform. `Je Wochentag`: Jeder gebuchte Tag hat einen Monatspreis, der Beitrag ist die Summe. `Nach Tagen je Woche`: Der Beitrag hängt nur von der Zahl der gebuchten Tage ab, zum Beispiel 3 Tage = 60,00 €.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Preisliste`.",
          "Wählen Sie bei der Schule `Neue Preise`.",
          "Wählen Sie bei `Gültig ab` den Monat, ab dem die Preise gelten.",
          "Wählen Sie bei `Beitragsfreie Monate` die Monate ohne Beitrag, zum Beispiel `Aug`. Dann stellt der Monatslauf für diese Schule im August keine Rechnung aus.",
          "Wählen Sie je Modul `Je Wochentag` oder `Nach Tagen je Woche`.",
          "Tragen Sie unter `1. Kind` die Monatspreise ein, zum Beispiel 16,00. Ein leeres Feld setzt keinen Preis. Dann gilt der Preis davor.",
          "Hat jedes weitere Kind einen eigenen Preis, tragen Sie ihn unter `Jedes weitere Kind` ein, zum Beispiel 12,50. Leer heißt: gleicher Preis wie beim 1. Kind. Legen Sie unter `Ermäßigungen` eine Geschwisterregel mit `Preis laut Preisliste` an.",
          "Kostet ein Modul an jedem Tag gleich viel, tragen Sie nur Montag ein. Wählen Sie dann `Mo-Preise für alle Tage übernehmen`.",
          "Kostet ein Modul dasselbe wie ein anderes, wählen Sie `Werte übernehmen von …` und dann das andere Modul.",
          "Ist das Essen nicht im Preis, setzen Sie den Haken bei `Essen extra`. Das Essen rechnet die Mensa ab.",
          "Wählen Sie `Preise speichern`.",
        ],
      },
      {
        title: "Rechnungsnummern festlegen",
        description: "Das machen Sie einmal vor der ersten Freigabe.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen` und dann `Abrechnungsvorgaben`.",
          "Wählen Sie bei `Rechnungsnummern der Monatsbeiträge` den Knopf `Muster festlegen`.",
          "Tragen Sie ein Muster ein, zum Beispiel MB-{JJJJ}-{NNNN}. Nehmen Sie ein anderes Kürzel als bei den Ferien.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Monatslauf starten und prüfen",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Monatsbeiträge`.",
          "Wählen Sie `Monatslauf starten` und dann den Monat.",
          "Wählen Sie bei `Schulen` die Schulen. Alle sind schon gewählt. Eine Schule, die schon in einem offenen Lauf dieses Monats ist, lässt sich nicht wählen.",
          "Wählen Sie bei `Rechnungsdatum` das Datum, das auf jeder Rechnung steht, zum Beispiel den 30.10. für Oktober. Lassen Sie das Feld leer, gilt der Tag der Freigabe.",
          "Wählen Sie `Vorschau öffnen`.",
          "Prüfen Sie die Rechnungen. Je Schule steht eine Karte. Jede Rechnung hat je Modul eine Zeile mit den gebuchten Wochentagen.",
          "Steht oben `Ohne Preis`, fehlt für einen gebuchten Wochentag oder eine Zahl von Tagen je Woche ein Preis. Wählen Sie `Preis eintragen`. Danach kommt das Kind in den Lauf.",
          "Steht oben `Doppelter Vertrag – bitte prüfen`, rechnen zwei Verträge eines Kindes am selben Tag dasselbe Modul ab. Die Freigabe rechnet dieses Kind mit allen seinen Verträgen nicht ab, alle anderen Kinder schon. Nach der Freigabe steht es unter `Nicht abgerechnet – Doppelter Vertrag`. Beenden oder ändern Sie einen der Verträge. Starten Sie dann einen neuen Monatslauf für diesen Monat. Er rechnet nur dieses Kind ab.",
        ],
      },
      {
        title: "Vorschau prüfen und Rechnungen ausschließen",
        description:
          "Sie können eine Rechnung aus diesem Lauf nehmen. Sie kommt im nächsten Lauf dieses Monats wieder. Hat ein Kind eine Rechnung für die Betreuung und eine eigene `Mensa-Rechnung`, schließen Sie jede für sich aus.",
        steps: [
          "Wählen Sie bei `Monatslauf` den Knopf `Prüfliste` und dann `Excel` oder `CSV`. Die Liste hat je Position eine Zeile. Rechnungsnummern stehen noch nicht darin.",
          "Stimmt eine Rechnung noch nicht, wählen Sie in ihrer Zeile das Menü mit den drei Punkten und dann `Ausschließen`.",
          "Prüfen Sie im Fenster, welche Rechnung Sie ausschließen. Dort steht zum Beispiel `Mensa-Rechnung` und darunter, ob die Betreuungsrechnung des Kindes im Lauf bleibt. Steht die Mensa auf der Rechnung der Betreuung, wird sie mit ausgeschlossen.",
          "Schreiben Sie bei `Grund`, warum, zum Beispiel „Kündigung wird noch geprüft“. Wählen Sie `Ausschließen`.",
          "Die Rechnung steht jetzt unter `Ausgeschlossen`. Sie zählt nicht zur Summe und bekommt bei der Freigabe keine Nummer. Die andere Rechnung des Kindes bleibt im Lauf.",
          "Mit `Wieder aufnehmen` nehmen Sie genau diese Rechnung zurück in den Lauf.",
          "Sind alle Rechnungen ausgeschlossen, wählen Sie trotzdem `Lauf freigeben`. Der Lauf schließt ohne Rechnungen und gibt die Schulen wieder frei. Die Gründe bleiben im Verlauf. Ein neuer Lauf kann diese Kinder abrechnen.",
          "Das Rechnungsdatum ändern Sie bei `Rechnungsdatum` mit `Ändern`.",
        ],
      },
      {
        title: "Leeren Monatslauf verwerfen",
        description:
          "Hat ein offener Lauf nichts abzurechnen, lässt er sich nicht freigeben. Das passiert zum Beispiel, wenn alle Verträge des Monats geendet haben. Dann verwerfen Sie ihn.",
        steps: [
          "Öffnen Sie den Lauf. Steht dort `Dieser Lauf hat nichts abzurechnen`, wählen Sie oben `Leeren Lauf verwerfen`.",
          "Wählen Sie im Fenster `Verwerfen`.",
          "Der Lauf steht jetzt als `Verworfen` in der Liste. Er hat nichts ausgestellt. Wer ihn wann verworfen hat, steht im Verlauf.",
          "Der Monat ist danach frei. Starten Sie bei Bedarf einen neuen Monatslauf.",
        ],
      },
      {
        title: "Eine Rechnung vor der Freigabe ändern",
        description:
          "Sie ändern nur diese eine Rechnung in diesem Lauf. Die Familie und andere Rechnungen bleiben, wie sie sind. Jede Änderung braucht einen Grund und steht im Verlauf der Rechnung.",
        steps: [
          "Wählen Sie in der Zeile der Rechnung das Menü mit den drei Punkten und dann `Rechnung ändern`. Die Seite der Rechnung öffnet sich.",
          "Wählen Sie oben `Ändern` und dann `Betrag ändern`, `Position hinzufügen`, `Empfänger ändern` oder `Anschrift ändern`.",
          "Tragen Sie den neuen Wert ein und schreiben Sie bei `Grund`, warum. Wählen Sie den Knopf unten im Fenster.",
          "Einen neuen Betrag zeigt die Rechnung als eigene Zeile `Korrektur` mit dem Unterschied. Die berechneten Zeilen bleiben stehen.",
          "Eine Position nehmen Sie heraus: Wählen Sie bei der Position das Menü mit den drei Punkten und dann `Position entfernen`. Die Position bleibt durchgestrichen sichtbar, die Rechnung bekommt eine Zeile mit Minus.",
          "Eine Änderung nehmen Sie zurück: Wählen Sie im `Verlauf der Änderungen` bei der Änderung `Zurücknehmen` und schreiben Sie einen Grund. Der Eintrag bleibt im Verlauf.",
          "Prüfen Sie danach die Vorschau. Haben Sie die Seite vorher geprüft, prüfen Sie sie noch einmal. Erst dann geben Sie frei.",
        ],
      },
      {
        title: "Monatslauf freigeben",
        steps: [
          "Wählen Sie `Lauf freigeben`.",
          "Prüfen Sie Zahl und Summe der Rechnungen und wählen Sie `Freigeben`.",
          "Jede Rechnung bekommt ihre Nummer und das Rechnungsdatum. Danach ändert sie sich nicht mehr.",
          "Hat sich seit Ihrer Prüfung etwas geändert, zum Beispiel ein Vertrag, ein Preis oder eine Änderung einer Rechnung, gibt moto nicht frei. Die Seite zeigt den neuen Stand. Prüfen Sie ihn und geben Sie dann frei.",
          "Passt eine Änderung einer Rechnung nicht mehr, zum Beispiel weil sich der Preis geändert hat, gibt moto nicht frei. Wählen Sie in der Meldung `Rechnung öffnen` und nehmen Sie die Änderung zurück.",
          "Die Familien sehen davon noch nichts. Sie bekommen erst das Schreiben, das Sie senden.",
        ],
      },
      {
        title: "Schreiben erstellen",
        description:
          "Jede Familie bekommt ein Schreiben im Briefkopf des Trägers. Es ist ihre Rechnung. Erstellen sendet noch nichts.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Schreiben an die Familien` den Knopf `Schreiben erstellen`. Die Seite mit den Schreiben des Laufs öffnet sich.",
          "Wählen Sie dort bei `Schreiben an die Familien` noch einmal `Schreiben erstellen`.",
          "Prüfen Sie die `Fälligkeiten`. Sie stehen auf den Rechnungen fest. Nur ältere Rechnungen haben noch das Feld `Zahlen bis`.",
          "Schreiben Sie bei `Hinweis für die Familien` etwas, wenn alle Familien es lesen sollen. Das ist freiwillig.",
          "Wählen Sie `Vorschau ansehen`. Sie sehen das Schreiben der ersten Familie.",
          "Wählen Sie `Schreiben erstellen`. Danach lassen sich die Schreiben nicht mehr ändern.",
        ],
      },
      {
        title: "Schreiben senden und drucken",
        description:
          "Jede Familie bekommt ihr Schreiben per E-Mail. Hat sie mit dieser Adresse ein Konto in der Eltern-App, liegt es auch dort unter `Dokumente`. Das geht nur, wenn das Kind einen Schülerdatensatz hat. Sonst steht bei der Rechnung `Nicht in der Eltern-App`, und die Familie bekommt das Schreiben per E-Mail oder gedruckt.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Schreiben an die Familien` den Knopf `Schreiben öffnen`. Dort steht schon, wie viele Schreiben gesendet sind.",
          "Wählen Sie bei `Schreiben an die Familien` den Knopf `An alle senden`. Prüfen Sie die Zahl und bestätigen Sie, zum Beispiel mit `3 Schreiben senden`.",
          "Die Karte zeigt, wie viele Schreiben gesendet sind, gerade gesendet werden, nicht angekommen sind, noch nicht gesendet sind und keine E-Mail-Adresse haben.",
          "In jeder Rechnung steht unter `An die Familie`, ob ihr Schreiben gesendet ist. `Ausgestellt am` sagt nur, dass die Rechnung ihre Nummer hat.",
          "Ist ein Schreiben nicht angekommen, wählen Sie bei der Rechnung das Menü mit den drei Punkten und dann `Erneut senden`. Mit `Senden` schicken Sie ein einzelnes Schreiben zum ersten Mal.",
          "Mit `Schreiben ansehen` im Menü sehen Sie genau das Schreiben, das die Familie bekommt.",
          "Für Familien ohne E-Mail wählen Sie `Drucken` und dann `Nur Familien ohne E-Mail`. Geben Sie ihnen das gedruckte Schreiben. `Alle Schreiben drucken` druckt alle Schreiben, die noch gelten.",
        ],
      },
      {
        title: "Rechnung stornieren und neu ausstellen",
        description:
          "Eine ausgestellte Rechnung ändert sich nie. Stimmt sie nicht, stornieren Sie sie. Der Storno bekommt eine eigene Nummer.",
        steps: [
          "Wählen Sie im freigegebenen Lauf `Schreiben öffnen`. Gibt es noch keine Schreiben, heißt der Knopf `Schreiben erstellen`.",
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten und dann `Stornieren`.",
          "Schreiben Sie bei `Begründung`, warum Sie stornieren. Wählen Sie `Stornieren`.",
          "Hatte die Familie die Rechnung schon bekommen, bekommt sie ein Storno-Schreiben. Es steht als eigene Zeile unter der Rechnung. Senden Sie es über das Menü dieser Zeile mit `Senden`.",
          "Haben Sie die Rechnung zum Drucken heruntergeladen, bekommt der Storno auch ein Schreiben. Laden Sie es über `Drucken` und `Alle Schreiben drucken` herunter. Geben Sie es der Familie auf Papier.",
          "Wurde die Rechnung weder gesendet noch zum Drucken heruntergeladen, bekommt die Familie kein Storno. Sie sieht nichts davon.",
          "Wählen Sie in der Meldung über den Rechnungen `Neu ausstellen`. Ein passender Prüflauf öffnet sich, sonst ein neuer Lauf. Ein neuer Lauf enthält nur Schulen mit noch offenen Stornos. Geben Sie ihn frei und senden Sie das neue Schreiben. Verteilen sich die Stornos auf mehrere Prüfläufe, wiederholen Sie `Neu ausstellen`. Sind alle Rechnungen neu ausgestellt, verschwindet die Meldung im alten Lauf.",
          "Die neue Rechnung übernimmt die Änderungen der stornierten Rechnung, zum Beispiel einen geänderten Betrag oder Empfänger. Im Verlauf steht dann `Übernommen aus` mit der alten Nummer. Passt eine Änderung nicht mehr, nehmen Sie sie dort zurück.",
        ],
      },
    ],
    result:
      "Die Rechnungen des Monats sind ausgestellt, und die Familien haben ihr Schreiben per E-Mail oder auf Papier. Ein späterer Lauf desselben Monats rechnet nur Kinder ab, die noch keine gültige Rechnung für diesen Monat haben.",
    differences: [
      "Beginnt, endet oder ändert sich ein Vertrag im Monat, zählt standardmäßig der Vertrag am 1. des Monats für den ganzen Monat. Beginnt die Betreuung erst im Monat, zählt der erste Vertrag den ganzen Monat. Ein weiterer Vertrag, der später im Monat beginnt, zählt erst ab dem nächsten Monat. Anteilig nach Kalendertagen stellen Sie unter `Einstellungen`, `Allgemein`, Bereich `Monatsbeitrag` ein.",
      "Mit einem Preis, für den es schon eine Rechnung gibt, rechnet der Monat weiter ab. Ändern Sie ihn ab einem späteren Monat. Fehlende Preise ergänzen Sie auch für einen schon abgerechneten Monat. Der nächste Lauf dieses Monats rechnet die Kinder dann ab.",
    ],
    notes: [
      "Welche Änderungen einer Rechnung möglich sind, stellt Ihr Träger unter `Einstellungen`, `Allgemein`, Bereich `Monatsbeitrag` ein. Standardmäßig sind alle vier an.",
      "Ferienpreise legen Sie im Ferienzeitraum fest. Die Preisliste gilt nur für die Monatsbeiträge.",
      "Die Preise für weitere Kinder gelten erst mit einer Geschwister-Regel mit der Wirkung `Preis laut Preisliste`. Die Regel legen Sie unter `Ermäßigungen` an.",
      "In einem beitragsfreien Monat steht im Monatslauf bei `Beitragsfreier Monat`, wie viele Verträge keine Rechnung bekommen.",
    ],
    troubleshootingDetails: [
      "Eine Schule lässt sich beim Start nicht wählen: Sie ist schon in einem offenen Lauf dieses Monats. Öffnen Sie diesen Lauf in der Liste und geben Sie ihn frei. Hat er nichts abzurechnen, wählen Sie `Leeren Lauf verwerfen`.",
      "`Leeren Lauf verwerfen` meldet, dass der Lauf etwas abzurechnen hat: Seit dem Öffnen ist ein Vertrag oder Preis dazugekommen. Prüfen Sie die Vorschau und geben Sie den Lauf frei.",
      "`Schreiben erstellen` geht nicht: Es fehlt eine Bankverbindung. Tragen Sie sie unter `Einstellungen`, `Abrechnungsvorgaben` ein.",
      "Ein Schreiben geht nicht raus, weil die Person nicht mehr im Vertrag steht: Stornieren Sie die Rechnung. Wählen Sie dann `Neu ausstellen`. Die neue Rechnung geht an die Person im Vertrag.",
      "`Erneut senden` fehlt: Die Mail ist noch unterwegs, oder die Adresse ist gesperrt. Der Hinweis bei der Rechnung sagt, was zu tun ist. Drucken Sie das Schreiben, wenn die Adresse gesperrt ist.",
    ],
    related: [
      HELP_TOPICS.traegerCareContracts,
      HELP_TOPICS.traegerBillingRun,
      HELP_TOPICS.traegerBillingTerms,
    ],
  };
}
