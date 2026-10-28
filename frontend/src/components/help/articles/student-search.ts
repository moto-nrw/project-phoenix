import type { HelpTopic, HelpPresenceMode, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function studentSearchTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  const currentStatusStep =
    presenceMode === "detailed"
      ? "Prüfen Sie oben den Aufenthaltsort sowie die heutige Ankunft und Abholung."
      : presenceMode === "binary"
        ? "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung."
        : "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung.";

  const presenceDifference =
    presenceMode === "binary"
      ? "Bei einfacher Anwesenheit sehen Sie keinen Raum. Sie sehen nur, ob das Kind da ist."
      : null;
  // #5026: ohne Räume (Binärmodus) gibt es keine Zuweisung.
  const transitNotes =
    presenceMode === "binary"
      ? []
      : [
          "Filtern Sie nach `Unterwegs`? Oder gruppieren Sie `Nach Status` oder `Nach Raum`? Dann steht oben die Karte `Unterwegs`. Mit `Zuweisen` setzen Sie diese Kinder in einen Raum. Die Karte sehen Sie nur, wenn Sie Kinder einem Raum zuweisen dürfen.",
        ];

  return {
    id: HELP_TOPICS.studentSearch,
    title: "Ein Kind finden und Angaben ansehen",
    question: "Wie finde ich ein Kind und seine Angaben?",
    summary: "Finden Sie ein Kind. Öffnen Sie danach die benötigten Angaben.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "Search",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind finden",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Geben Sie in `Name suchen...` den Namen ein.",
          "Sie können auch nur einen Namensteil eingeben.",
          // Der Filter traegt kein sichtbares Wort, nur ein Symbol mit drei
          // Reglern (FilterButton, aria-label "Filter").
          "Zu viele Treffer? Wählen Sie rechts neben der Suche das Filtersymbol.",
          // Ohne feste Gruppen gibt es auch keinen Gruppenfilter.
          groupMode === "open_care"
            ? "Wählen Sie zum Beispiel eine Klasse."
            : "Wählen Sie zum Beispiel eine Klasse oder Gruppe.",
          // Der Klick auf den eigenen Menüeintrag leert Suche und Filter
          // (#3374). Am Handy heißt der Eintrag unten "Suchen".
          "Neue Suche? Wählen Sie im Menü `Alle Kinder` oder unten `Suchen`. Suche und Filter sind dann leer.",
        ],
      },
      {
        title: "Angaben ansehen",
        steps: [
          "Wählen Sie die Karte des Kindes.",
          currentStatusStep,
          // #4761: die Zeilen stehen unter `Heutige Ankunft` und `Heutige
          // Abholung`, sobald das Kind an- oder abgemeldet ist.
          "Unter der Zeit steht, wer das Kind heute an- und abgemeldet hat. `am Gerät` heißt: am Tablet ohne Person. `Ehemalige Mitarbeitende` heißt: Die Person ist nicht mehr in moto. `automatisch` heißt: moto hat das Kind selbst abgemeldet.",
          // #4795: dieselbe Zeile steht in der Aufsicht und in der Gruppe.
          "Steht auf der Karte `Hinweis der Lehrkraft`? Den hat die Lehrkraft heute in moto schule geschrieben. Morgen ist er weg.",
        ],
      },
      {
        title: "Den passenden Bereich wählen",
        ordered: false,
        steps: [
          "Wählen Sie `Stammdaten` für persönliche Angaben.",
          "Wählen Sie `Nachrichten` für den Nachrichtenaustausch mit den Eltern.",
          "Wählen Sie `Erziehungsberechtigte` für Kontaktangaben.",
          "Wählen Sie `Betreuungsplan` für den geplanten Tages- oder Wochenablauf.",
          "Wählen Sie `Betreuungszeiten` für regelmäßige Ankunfts- und Abholzeiten.",
          // Live geprueft: mit Betreuer-Rechten fehlt der Reiter ganz. Er
          // bleibt trotzdem in der Liste -- die Leitung liest denselben
          // Artikel und sieht ihn (canViewEnrollments = `config:manage`).
          "Wählen Sie `Anmeldungen` für Angaben aus der Halbjahresanmeldung. Diesen Reiter sieht nur die Leitung. Über `Herunterladen` wählen Sie dort `PDF`, `Word` oder `Excel`.",
          "Wählen Sie `Dokumente` für Dateien zum Kind.",
          "Wählen Sie `Änderungsprotokoll` für Änderungen an den Angaben.",
          "Wählen Sie `Historie` für Anwesenheit, Feedback und weitere Verläufe.",
        ],
      },
      {
        title: "Ein Anwesenheitsprotokoll herunterladen",
        steps: [
          "Öffnen Sie beim Kind `Historie` und danach `Anwesenheitsprotokoll`.",
          // #4761: Tabelle und Export nennen dieselben Angaben.
          "Bei jedem Tag steht, wer das Kind an- und abgemeldet hat. Die Datei enthält dazu die Spalten `Angemeldet von` und `Abgemeldet von`.",
          "Wählen Sie im Drei-Punkte-Menü `Exportieren`.",
          "Wählen Sie unter `Von` und `Bis` den Zeitraum.",
          "Möglich sind nur die sichtbaren Tage bis heute.",
          "Wählen Sie das Format und danach `Herunterladen`.",
        ],
      },
      {
        // #3834: derselbe Umschalter steht auch in `Gruppen` und in der
        // aktuellen Aufsicht, auch auf dem Handy.
        title: "Als Liste ansehen",
        description:
          "Die Liste zeigt dieselben Angaben wie die Karten, ein Kind pro Zeile.",
        steps: [
          "Wählen Sie oben rechts das Listensymbol (drei Striche). Mit dem Kachelsymbol (vier Kästchen) kommen die Karten zurück.",
          "Spalten ein- oder ausblenden? Öffnen Sie oben rechts das Menü mit den drei Punkten. Wählen Sie unter `Spalten in der Liste` die Spalten.",
          "Auf dem Handy zeigt die Liste je Kind eine Zeile. Was unter dem Namen steht, wählen Sie im Menü mit den drei Punkten unter `In der Zeile zeigen`, zum Beispiel `Gehzeit` oder `Gruppe`.",
          "Kreuzen Sie links in der Zeile Kinder an. Oben erscheint eine Leiste mit `Exportieren`, `Anmelden` und `Abmelden`.",
          "Wählen Sie `Aufheben`, um alle Kreuze zu entfernen.",
        ],
      },
    ],
    result:
      "Sie sehen jetzt die Angaben des Kindes. Über dem Namen steht `Zurück zur Kinderübersicht`. Damit kommen Sie zur Liste zurück.",
    differences: [
      "Haben Sie einen anderen Tag gewählt? Dann sehen Sie die geplante Anwesenheit. Ein aktueller Aufenthaltsort wird nicht gezeigt.",
      "Ansicht und Spalten bleiben auf diesem Gerät so, wie Sie sie gewählt haben.",
      // Einstellung „Kinder nur über ‚Alle Kinder‘ finden“ (#1321).
      "Fehlen `Gruppen` und `Aktuelle Aufsicht` in der Navigation? Dann wählen Sie in `Alle Kinder` oben `Meine Gruppe` oder `Meine Aufsicht`.",
      "Einige Bereiche brauchen zusätzliche Rechte: `Betreuungsplan`, `Dokumente` und `Änderungsprotokoll`. Fehlt ein Bereich? Fragen Sie Ihre Leitung.",
      ...(presenceDifference ? [presenceDifference] : []),
    ],
    ...(transitNotes.length > 0 ? { notes: transitNotes } : {}),
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.editStudent,
      HELP_TOPICS.webAttendance,
      HELP_TOPICS.absences,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
