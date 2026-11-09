import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/einstellungen` (#4652, ADR 0086, #5685): Träger-Werte zwischen Standard
 * und Schule. Bezeichnungen aus `settings-origin.ts` und `traeger-nav-items.ts`.
 */
export function traegerSettingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSettings,
    title: "Einstellungen für alle Schulen festlegen",
    question: "Wie lege ich Einstellungen für alle Schulen fest?",
    summary: "Was Sie hier festlegen, gilt für alle Schulen Ihres Trägers.",
    group: "konfiguration",
    audience: "traeger",
    icon: "Settings",
    requirements: ["Sie dürfen im Träger-Büro Einstellungen ändern."],
    steps: [
      "Wählen Sie unten in der Navigation `Einstellungen`.",
      "Wählen Sie `Allgemein`.",
      "Wählen Sie den Bereich, zum Beispiel `Anmeldung` oder `Betrieb`.",
      "Ändern Sie den Wert. moto speichert die Änderung von selbst.",
    ],
    result:
      "Der Wert gilt sofort für alle Schulen Ihres Trägers. Auch neue Schulen übernehmen ihn.",
    notes: [
      "Bei Ihrem Wert steht `Vom Träger festgelegt` und das Datum der letzten Änderung.",
      "`Standard` heißt: Sie haben nichts festgelegt. Es gilt der Wert von moto.",
      "`Zurücksetzen` entfernt Ihren Wert. Dann gilt wieder der Standard.",
      "Eine Schule kann einen eigenen Wert eintragen. Dann gilt dort ihr Wert.",
      "Den eigenen Wert einer Schule kann nur die Schule selbst zurücksetzen.",
      "Im Bereich `Kontakt` tragen Sie den `Ansprechpartner` Ihres Trägers ein. Er steht auf allen Briefen und Rechnungen.",
      "Darunter steht `Kontakt für Widerrufe`. Diese Telefonnummer und E-Mail-Adresse nennt die Widerrufsbelehrung jedes Betreuungsvertrags. Ein leeres Feld nimmt die Angabe des Ansprechpartners. Die Vorschau zeigt den Satz so, wie Eltern ihn lesen. Nach einer Änderung kopiert die Schule die Widerrufsbelehrung neu ins Anmeldeformular.",
      "Mit `Zurück zur App` oben in der Navigation kommen Sie zur zuletzt geöffneten Seite zurück.",
      "Im Bereich `Ferienbetreuung` legen Sie fest, ob Sie beim Bestätigen einer Ferienanmeldung die Debitorennummer der Familie abfragen. Diese Einstellung gilt nur im Träger-Büro.",
      "Ebenfalls unter `Ferienbetreuung` schreiben Sie bei `Zahlungstext im Schreiben` einen eigenen Satz, der um die Zahlung bittet. Erlaubte Platzhalter sind {Betrag}, {Zahlungsfrist}, {Rechnungsnummer} und {Debitorennummer}. Darunter zeigt die `Vorschau des Zahlungstexts` den Satz mit Beispielwerten. Ist das Feld leer, steht im Schreiben der eingebaute Satz.",
      "Der eigene Zahlungstext gilt für die Schreiben, die Sie danach erstellen. Fertige Schreiben bleiben, wie sie sind.",
      "Im Bereich `Dashboard` legen Sie fest, ab wann das Dashboard eine Schule unter `Auffälligkeiten` zeigt: bei zu wenig Personal, bei vielen Krankmeldungen und bei offenen Anmeldungen. 0 schaltet einen Hinweis aus.",
      "Unter `Ferienbetreuung` legen Sie bei `Ferienbeitrag` auch fest, welches Kind das erste Kind ist und was ein Kostenträger bezahlt. Mehr dazu steht im Artikel `Ferienbeitrag festlegen`.",
    ],
    differences: [
      "Auf dem Handy steht `Einstellungen` in der Leiste unten, meist unter `Mehr`. Dort führt `Zurück zur App` wieder zurück.",
      "Sie können die Werte nur ansehen? Ihnen fehlt das Recht zum Ändern. Wenden Sie sich an das moto-Team.",
    ],
    related: [HELP_TOPICS.traegerLogin, HELP_TOPICS.errorMessages],
  };
}
