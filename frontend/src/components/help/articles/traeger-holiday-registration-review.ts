import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/ferien/[id]/anmeldungen` (#4663): Anmeldungen prüfen. */
export function traegerHolidayRegistrationReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayRegistrationReview,
    title: "Ferienanmeldungen prüfen",
    question:
      "Wie bestätige, ändere, storniere und buche ich Ferienanmeldungen um?",
    summary:
      "Sie sehen alle Anmeldungen eines Ferienzeitraums in einer Liste. Dort bestätigen, ändern, stornieren und buchen Sie um. Geschwister aus zwei Anmeldungen führen Sie zusammen.",
    image: "/help/screens/ferienanmeldung-zweite-person-traeger.webp",
    imageAlt:
      "Die zweite sorgeberechtigte Person in der Einzelansicht beim Träger.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "ListChecks",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Liste öffnen und filtern",
        steps: [
          "Wählen Sie in der Navigation `Ferienbetreuung`.",
          "Öffnen Sie den Ferienzeitraum.",
          "Wählen Sie den Reiter `Anmeldungen`.",
          "Filtern Sie nach Status, `Schule`, `Woche` oder `Zeitmodell`. Unter `Schule` finden Sie auch `Feriengäste`.",
          "Wählen Sie eine Anmeldung, um sie zu öffnen.",
        ],
      },
      {
        title: "Bestätigen oder stornieren",
        steps: [
          "Prüfen Sie die Angaben der Eltern und der Kinder. Antworten auf eigene Fragen stehen beim Kind unter `Weitere Angaben`, allgemeine Antworten unter `Allgemeine Angaben`.",
          "Unter `Hinweise und Einwilligungen` sehen Sie, was die Eltern bestätigt haben und wann. Dort stehen die damaligen Texte und PDFs.",
          "Unter `Eltern` stehen `Sorgerecht bestätigt`, `Name als Erklärung` und `Fassung der Texte`. Den genauen Text öffnen Sie unter `Wortlaut der Erklärung`.",
          "Prüfen Sie unter `Ferienbeitrag`, was die Familie zahlt.",
          "Geben die Eltern an, dass sie Leistungen beziehen? Prüfen Sie den Nachweis. Haben die Eltern ihn hochgeladen, wählen Sie unter `Nachweis für den Leistungsbezug` den Dateinamen. Die Datei öffnet sich in einem neuen Tab.",
          "Bringt die Familie den Nachweis auf Papier, scannen Sie ihn. Wählen Sie unter `Nachweis für den Leistungsbezug` den Knopf `Nachweis hochladen` und dann die Datei. Erlaubt sind PDF, JPEG und PNG bis 10 MB.",
          "Wählen Sie unter `Wer bezahlt den Beitrag?`: `Die Familie bezahlt`, `Von der Zahlung befreien (Leistungsbezug)` oder `Kosten übernimmt ein Kostenträger`. Beim Kostenträger wählen Sie darunter, welcher bezahlt. Der `Ferienbeitrag` zeigt sofort, was festgehalten wird.",
          "Fragt Ihr Träger die Debitorennummer ab, steht über `Bestätigen` das Feld `Debitorennummer`. Tragen Sie dort die Nummer der Familie bei der Kasse der Gemeinde ein. Ohne Nummer geht `Bestätigen` nicht.",
          "Wählen Sie `Bestätigen`. Damit ist der Beitrag festgehalten.",
          "Oder wählen Sie `Stornieren` und bestätigen Sie mit `Stornieren`.",
        ],
      },
      {
        title: "Debitorennummer nachtragen oder ändern",
        description:
          "Das geht bei einer bestätigten Anmeldung, bis der Abrechnungslauf ihre Rechnung ausgestellt hat.",
        steps: [
          "Öffnen Sie die bestätigte Anmeldung.",
          "Wählen Sie unter `Status` den Knopf `Debitorennummer eintragen` oder `Debitorennummer ändern`.",
          "Das Feld `Debitorennummer` öffnet sich in der Karte. Tragen Sie die Nummer ein und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Angaben ändern",
        description:
          "Zum Beispiel eine neue Telefonnummer oder ein falsch geschriebener Name.",
        steps: [
          "Wählen Sie bei der Karte `Ändern`: bei `Eltern`, beim Kind, bei `Notfallkontakte und Abholung` oder bei `Allgemeine Angaben`.",
          "Ändern Sie die Angaben. Es gelten dieselben Regeln wie im Formular der Eltern. Fragt das Formular nach der Anschrift, ändern Sie sie bei `Eltern`.",
          "Wählen Sie `Änderung speichern`.",
        ],
      },
      {
        title: "Geschwister zusammenführen",
        description:
          "Eltern melden Geschwister manchmal in zwei Anmeldungen an. Führen Sie die Anmeldungen zusammen, zählen die Kinder beim Ferienbeitrag zusammen. moto führt nie von selbst zusammen.",
        steps: [
          "In der Spalte `Geschwister` steht ein Hinweis wie `Vielleicht Geschwister: gleiche E-Mail` oder `Vielleicht Geschwister: gleicher Nachname`. Der Hinweis ändert nichts.",
          "Prüfen Sie, ob die Kinder wirklich Geschwister sind.",
          "Setzen Sie in der Liste einen Haken bei jeder Anmeldung der Familie.",
          "Wählen Sie `Als Geschwister zusammenführen`.",
          "In der Spalte `Geschwister` steht jetzt `Geschwister` mit den Nummern der anderen Anmeldungen.",
        ],
      },
      {
        title: "Geschwister trennen",
        steps: [
          "Öffnen Sie die Anmeldung.",
          "Wählen Sie in der Karte `Geschwister` den Knopf `Trennen`.",
          "Bestätigen Sie mit `Trennen`.",
        ],
      },
      {
        title: "Umbuchen",
        steps: [
          "Wählen Sie beim Kind `Umbuchen`.",
          "Wählen Sie je Woche ein Zeitmodell oder `Nicht angemeldet`. Gibt es eine Woche an mehreren Schulen, wählen Sie auch die Schule.",
          "Wird die Woche tageweise gebucht, wählen Sie darunter die Tage. Mehr Tage als `Höchstens Tage je Kind` nimmt moto nicht an.",
          "Wählen Sie `Speichern`.",
          "Eine Woche, die das Kind nicht mehr bucht, gibt ihren Platz frei. Das nächste Kind der Warteliste rückt sofort nach. Eine neue Woche bekommt einen Platz, wenn einer frei ist. Sonst kommt das Kind dort auf die Warteliste.",
          "Kommt bei einer Woche mit einzelnen Tagen ein Tag dazu, braucht das Kind an jedem Tag einen Platz. Fehlt einer, wartet die ganze Woche wieder. Fallen nur Tage weg, behält das Kind seinen Platz.",
        ],
      },
      {
        title: "Einfrieren und wieder öffnen",
        steps: [
          "Nach dem Anmeldeschluss ist jede Anmeldung von selbst eingefroren.",
          "Wollen Sie eine Anmeldung vorher festhalten? Wählen Sie `Einfrieren`.",
          "Wollen Sie eine eingefrorene Anmeldung ändern? Wählen Sie `Wieder öffnen`.",
        ],
      },
      {
        title: "Exportieren",
        steps: [
          "Stellen Sie die Filter ein, zum Beispiel eine Schule.",
          "Wählen Sie `Herunterladen` und dann `Excel` oder `CSV`.",
        ],
      },
    ],
    result:
      "Unter `Verlauf` steht bei jeder Anmeldung, wann sie bestätigt, geändert, storniert, umgebucht, eingefroren oder wieder geöffnet wurde. Auch jede Debitorennummer steht dort, mit der Nummer vorher und nachher. Bei einer Änderung der Angaben steht dort, welche Angaben sich geändert haben, aber nicht die Werte. Auch jedes Zusammenführen und Trennen von Geschwistern steht dort. Storniert jemand eine Anmeldung der Geschwister, steht bei allen `Aus Geschwistergruppe gelöst (storniert)`.",
    notes: [
      "Berücksichtigt der Ferienzeitraum den Ferienwunsch, filtern Sie auch nach `Ferienwunsch`. Beim Kind steht dann `Mit Ferienwunsch`, `Ohne Ferienwunsch` oder `Ferienwunsch prüfen`.",
      "Beim Bestätigen bekommen die Eltern eine E-Mail mit den Kindern, Wochen und Zeitmodellen. Darin steht auch Ihr `Text der Annahme-Mail` aus dem Ferienzeitraum. Jede Anmeldung bekommt diese E-Mail nur einmal.",
      "Beim Stornieren bekommen Eltern keine Nachricht von moto. Rückt ein Kind von der Warteliste nach, bekommen seine Eltern eine E-Mail.",
      "Vergibt der Ferienzeitraum die Plätze nach Kriterien? Dann steht bei einer Woche zuerst `Platz offen`. Bestätigen geht erst, wenn Ihr Büro im Reiter `Vergabe` entschieden hat. Unter `Angaben für die Vergabe` sehen Sie die Antworten der Eltern und prüfen sie mit `Prüfen`.",
      "Bei jeder Woche eines Kindes steht `Platz` oder `Warteliste`. Auf der Warteliste steht der Rang dabei, zum Beispiel `Warteliste, Platz 2`. Stornieren Sie eine Anmeldung, rückt das nächste Kind der Warteliste sofort nach.",
      "Eine eingefrorene Anmeldung können Sie bestätigen. Stornieren und Umbuchen gehen erst nach `Wieder öffnen`.",
      "Der Export hat eine Zeile je Kind und Woche. Gesundheitsangaben stehen nicht darin. Antworten auf eigene Fragen ohne Gesundheitsangabe haben eigene Spalten.",
      "Gesundheitsangaben sieht in der Einzelansicht nur die Rolle Verwaltung. Jedes Öffnen steht im `Zugriffsprotokoll` des Trägers.",
      "Eine `Zweite sorgeberechtigte Person` steht in der Einzelansicht, nicht in der Liste. Im Export stehen Name und Telefon.",
      "Fragt das Formular die Anschrift ab, steht sie in der Einzelansicht bei `Eltern` und im Export. In der Liste und bei der ausrichtenden OGS steht sie nicht.",
      "Ein hochgeladener `Nachweis für den Leistungsbezug` steht nur in der Einzelansicht. Er steht nicht in der Liste, nicht im Export und nicht bei der ausrichtenden OGS. Er wird mit der Anmeldung gelöscht.",
      "Jedes Öffnen einer Anmeldung oder eines Nachweises steht im `Zugriffsprotokoll` des Trägers. Auch jeder Export steht dort.",
      "Ein festgehaltener Beitrag ändert sich nicht mehr. Auch nicht, wenn Sie später Preise oder Regeln ändern oder umbuchen. Nur wenn ein Kind von der Warteliste nachrückt, rechnet moto den Beitrag mit der neuen Woche neu.",
      "Die Debitorennummer steht im Bestätigungsschreiben als Verwendungszweck. Im Export steht sie in der Spalte `Debitorennummer`. In der Liste steht sie nur, wenn Ihr Träger sie abfragt.",
      "Eine Änderung der Angaben ändert den Ferienbeitrag nicht. Ein schon erstelltes Schreiben an die Familie bleibt, wie es ist. Auch eine schon ausgestellte Rechnung behält die bisherige Anschrift.",
      "Kinder hinzufügen oder entfernen geht nicht. Auch Wochen, Leistungsbezug, Nachweise, Einwilligungen, die Erklärung der Eltern und Gesundheitsangaben ändern Sie hier nicht. Die Wochen ändern Sie mit `Umbuchen`, die Debitorennummer unter `Status`.",
      "Die ausrichtende OGS sieht einen geänderten Namen sofort in ihrer Teilnehmerliste. Bei einem Feriengast steht der neue Name spätestens nach einer Stunde auch in der Gruppe.",
      "Bei Geschwistern ist das Kind mit dem höchsten Beitrag das 1. Kind. Ihr Träger kann in den Einstellungen unter `Allgemein` auch das älteste Kind wählen. Bei gleichem Rang zählt die früher eingegangene Anmeldung zuerst.",
      "Zusammenführen und Trennen rechnen auch einen schon bestätigten Beitrag neu. Der alte Beitrag bleibt gespeichert. Ein offener Abrechnungslauf zeigt den neuen Betrag.",
      "Stornieren Sie eine Anmeldung der Geschwister, gehört sie nicht mehr dazu. moto rechnet den Beitrag der anderen Geschwister sofort neu. Sie müssen nichts trennen.",
      "Gibt es für ein Geschwister-Kind schon eine Rechnung, bleibt die Rechnung, wie sie ist. moto rechnet den Beitrag trotzdem neu. Unter `Ferienbeitrag` steht dann `Rechnung prüfen`. Stornieren Sie die Rechnung. Der nächste Abrechnungslauf stellt dann den neuen Betrag aus.",
      "Jede Anmeldung bekommt weiter ihre eigene Rechnung und ihr eigenes Schreiben. Ein Schreiben nennt nie die Kinder oder Eltern der anderen Anmeldung.",
      "Alle bestätigten Anmeldungen der Geschwister bekommen dieselbe Debitorennummer. Beim Bestätigen steht die Nummer der Geschwister schon im Feld.",
    ],
    differences: [
      "Ein Kind ist als `Mögliche Doppelanmeldung` markiert? Ein Kind mit gleichem Namen und Geburtsdatum steht auch in einer anderen Anmeldung. Öffnen Sie beide und stornieren Sie die doppelte.",
      "Beim Bestätigen fehlt `Kosten übernimmt ein Kostenträger`? Ihr Träger hat noch keinen aktiven Kostenträger. Die Abrechnung legt ihn in den Einstellungen unter `Abrechnungsvorgaben` an.",
      "Beim Bestätigen gibt es kein Feld `Debitorennummer`? Ihr Träger fragt sie nicht ab. Das legen Sie in den Einstellungen unter `Allgemein` im Bereich `Ferienbetreuung` fest.",
    ],
    troubleshootingDetails: [
      "Eine Änderung führt zu zu vielen Tagen je Kind? Prüfen Sie die Anmeldungen vor dem Speichern. Bei gleicher Eltern-E-Mail, gleichem Namen und Geburtsdatum zählen die Tage zusammen. Das gilt auch, wenn Sie diese Angaben ändern.",
      "`Umbuchen` fehlt? Die Anmeldung ist eingefroren oder storniert. Eine eingefrorene Anmeldung öffnen Sie mit `Wieder öffnen`.",
      "Eine stornierte Anmeldung lässt sich nicht wieder öffnen. Die Eltern melden das Kind dann neu an.",
      "Die Debitorennummer lässt sich nicht mehr ändern? Für die Anmeldung gibt es schon eine Rechnung. Die Rechnung behält die Nummer, mit der sie ausgestellt wurde.",
      "`Ändern` fehlt? Die Anmeldung ist storniert. Dann lassen sich die Angaben nicht mehr ändern.",
      "`Trennen` fehlt oder Zusammenführen geht nicht? Für eine Anmeldung der Geschwister gibt es schon eine Rechnung. Geschwister können Sie dann nicht mehr ändern.",
      "Eine stornierte Anmeldung können Sie nicht zusammenführen. Beim Stornieren löst moto sie schon aus den Geschwistern.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayRegistration,
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayPlaces,
      HELP_TOPICS.traegerHolidayAllocation,
      HELP_TOPICS.traegerHolidayContribution,
      HELP_TOPICS.traegerHolidayWish,
    ],
  };
}
