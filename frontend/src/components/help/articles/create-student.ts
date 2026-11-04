import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Kinder anlegen: einzeln im Slide-over `Neues Kind` auf
 * `/database/students`, viele auf einmal unter `/database/students/import`.
 * Beides an keine Einstellung gebunden.
 *
 * Live geprueft: `Neues Kind` ist ein einziges Fenster. Pflicht sind nur
 * `Vorname`, `Nachname` und `Klasse`; darunter folgen die Abschnitte
 * `Erziehungsberechtigte`, `Betreuungszeiten`, `Gesundheitsinformationen`,
 * `Elternnotizen`, `Datenschutz` und `Erlaubte Heimwege`
 * -- alle vor `Erstellen`, alle freiwillig. Deshalb tragen die Schritte nur
 * den Weg, nicht jedes Feld.
 *
 * Die Weiche `Nur Klassenliste` erscheint nur mit `users:create`
 * (students/page.tsx:638), fuer die Leitung also immer. Der Einstieg
 * `Importieren` steht nur auf breiten Bildschirmen (`!isMobile`,
 * page.tsx:723).
 */
export function createStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCreateStudent,
    title: "Kinder anlegen",
    question: "Wie lege ich Kinder an?",
    summary: "Einzeln über ein Fenster. Viele auf einmal über eine Vorlage.",
    group: "einrichten",
    audience: "lead",
    icon: "User",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind einzeln anlegen",
        description:
          "Alles steht in einem Fenster. Pflicht sind nur drei Felder, alles andere können Sie später nachtragen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie oben rechts `+ Kinder`.",
          "Bleiben Sie oben auf `Mit OGS-Betreuung`.",
          "Tragen Sie `Vorname`, `Nachname` und `Klasse` ein.",
          "Füllen Sie weiter unten aus, was Sie schon wissen.",
          "Wählen Sie unten `Erstellen`.",
        ],
      },
      {
        title: "Viele Kinder aus einer Liste übernehmen",
        description:
          "Für ein ganzes Schuljahr ist die Vorlage schneller als das Fenster.",
        steps: [
          "Öffnen Sie `Kinderdaten` und wählen Sie oben `Importieren`.",
          "Wählen Sie bei `Format wählen` `Excel (.xlsx)` oder `CSV (Komma-getrennt)`.",
          "Wählen Sie `Vorlage herunterladen`.",
          "Tragen Sie die Kinder ein. Beim Geburtstag geht zum Beispiel `14.03.2018`.",
          "Bleiben Sie unter `Was soll der Import tun?` bei `Nur neue anlegen`.",
          "Ziehen Sie die Datei unter `Schritt 3: Datei hochladen` in das Feld.",
          "Prüfen Sie die `Datenvorschau`.",
          "Wählen Sie unten den grünen Knopf, zum Beispiel `12 Kinder importieren`.",
        ],
      },
    ],
    result:
      "Ein einzeln angelegtes Kind steht sofort unter `Kinderdaten`. Nach einem Import meldet moto, wie viele Kinder angelegt und wie viele aktualisiert wurden.",
    notes: [
      "Im Fenster stehen weiter unten `Erziehungsberechtigte`, `Betreuungszeiten`, `Gesundheitsinformationen`, `Elternnotizen`, `Datenschutz` und `Erlaubte Heimwege`.",
      "Bei einem Geschwisterkind wählen Sie `Vorhandene/n suchen`. Dann teilen beide dieselben Erziehungsberechtigten.",
      "Ein Auge neben einer Angabe heißt: Eltern sehen sie.",
      "In der Vorlage erklärt das Blatt `Hinweise` jede Spalte.",
      "`Nur bestehende aktualisieren` und `Beides` brauchen Sie erst, wenn Sie Angaben nachträglich ändern. Leere Zellen ändern nie etwas.",
      "Hat Ihre Schule ein Kinderkontingent, steht es oben in `Kinderdaten`. Zum Beispiel `Kinderkontingent: 48 von 50 belegt`. Es zählen aktive Kinder und Kinder, deren Betreuung später beginnt. Das Info-Symbol daneben erklärt es auch.",
    ],
    differences: [
      "`Vorname`, `Nachname` und `Klasse` tragen einen roten Stern. Sie sind Pflicht.",
      "Oben im Fenster können Sie auf `Nur Klassenliste` wechseln. Das ist für Kinder ohne OGS-Betreuung.",
      "Auf dem Handy steht statt `+ Kinder` ein rundes Plus unten rechts. `Importieren` fehlt dort ganz.",
    ],
    troubleshootingDetails: [
      "Fehlt die passende `Gruppe`? Legen Sie sie zuerst in der `Datenverwaltung` unter `Gruppendaten` an.",
      "Der grüne Knopf ist grau? Dann hat die Vorschau noch Fehler. Beheben Sie sie in der Datei und laden Sie erneut hoch.",
      "Über der Vorschau steht rot, der Import würde mehr Kinder hinzufügen, als im Kinderkontingent frei sind? Dann startet der Import gar nicht. Nehmen Sie Kinder aus der Datei heraus oder melden Sie sich beim moto-Team. Kinder, die der Import nur aktualisiert, zählen dabei nicht.",
      "moto erkennt eine Zeile an Vorname, Nachname und Klasse. Bei einem Klassenwechsel an der Spalte `RFID-Karte` oder am Geburtstag.",
      "moto meldet, das Kinderkontingent ist voll? Dann ist die Kontingentzahl erreicht. Wie viel belegt ist, steht oben in `Kinderdaten`. Ihre Eingaben bleiben im Fenster. Für weitere Kinder melden Sie sich beim moto-Team.",
    ],
    related: [
      HELP_TOPICS.leadCareTimes,
      HELP_TOPICS.leadGroups,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
