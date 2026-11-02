import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Formular-Editor unter `/enrollment-form`. Zwei Dinge sind hier
 * folgenreich und stehen deshalb gross im Artikel:
 *
 * 1. Das `Basisformular` existiert immer und ist ein Systemformular. Die
 *    Seite raet selbst davon ab, ohne Not eine Vorlage anzulegen. Wer das
 *    nicht weiss, baut eine ueberfluessige Vorlage.
 * 2. `Stammdaten-Vorschläge` schreiben die Antwort in die Daten des Kindes
 *    (FIELD_TARGET_LABELS, enrollment-form-editor.tsx:139), eine
 *    `Freie Zusatzfrage` bleibt bei der Anmeldung. Gleiche Frage, anderes
 *    Ergebnis -- und im Formular sehen beide gleich aus.
 *
 * Die Vorlage wirkt erst, wenn eine Anmeldephase sie auswaehlt: dort im
 * Abschnitt `Formular` die Option `Eigene Vorlage wählen`
 * (phases-editor.tsx:1565).
 */
export function enrollmentFormTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentForm,
    title: "Ein eigenes Anmeldeformular anlegen",
    question: "Wie frage ich bei der Anmeldung mehr ab?",
    summary:
      "Nur nötig, wenn das Basisformular nicht reicht. Sie legen dann eine Vorlage an.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "FileText",
    requirements: [
      "Sie haben das `Basisformular` angesehen und etwas vermisst.",
    ],
    image: "/help/screens/anmeldung-pdf-anhaenge.webp",
    imageAlt:
      "Zwei PDF-Anhänge mit eigenen Bezeichnungen und Aktionen im Formular-Editor",
    steps: [],
    instructionGroups: [
      {
        title: "Erst prüfen, ob Sie eine Vorlage brauchen",
        description:
          "Das `Basisformular` ist immer da und fragt Elternteil, Kind, Klassenstufe und das gewünschte Betreuungsangebot ab. Für die meisten OGS reicht das.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Anmeldungen` und danach `Anmeldeformulare`.",
          "Wählen Sie beim `Basisformular` die `Vorschau`.",
          "Fehlt nichts? Dann sind Sie fertig. Legen Sie keine Vorlage an.",
        ],
      },
      {
        title: "Eine Vorlage anlegen",
        description:
          "Eine Vorlage ergänzt das Basisformular. Sie ersetzt es nicht.",
        steps: [
          "Wählen Sie oben rechts `+ Neue Vorlage`.",
          "Tragen Sie bei `Name der Vorlage` ein, wofür sie gilt, zum Beispiel `Ferienbetreuung Sommer 2026`.",
          "Gehen Sie `Pflichtstatus der Basisfelder` durch. Was auf `Optional` steht, können Sie zur Pflicht machen.",
          "Ergänzen Sie unter `Was Eltern zusätzlich beantworten sollen` Ihre Fragen.",
          "Sehen Sie rechts in der Vorschau nach, wie das Formular wirkt.",
          "Wählen Sie unten `Formularvorlage erstellen`.",
        ],
      },
      {
        title: "Die richtige Sorte Frage wählen",
        description:
          "Zwei Sorten sehen im Formular gleich aus, landen aber woanders. Das ist die wichtigste Entscheidung in diesem Fenster.",
        steps: [
          "Soll die Antwort später beim Kind stehen? Nehmen Sie einen `Stammdaten-Vorschlag` und wählen Sie `Hinzufügen`.",
          "Zur Auswahl stehen unter anderem `Abholzeiten`, `Ankunftszeiten`, `Erlaubte Heimwege` und `Gesundheitsinformationen`.",
          "Geht es nur um diese eine Anmeldung? Wählen Sie `+ Freie Zusatzfrage`.",
          "Fragen Sie etwas zum Kind, zum Beispiel zu Allergien oder zum Schwimmen? Schalten Sie bei der Frage `Pro Kind abfragen` ein. Dann beantworten Eltern sie für jedes Kind.",
          "Wollen Sie nur etwas erklären, ohne zu fragen? Wählen Sie `Infotext`.",
        ],
      },
      {
        title: "Eltern Dateien hochladen lassen",
        description:
          "Mit dem Typ `Datei` laden Eltern im Formular Nachweise hoch, zum Beispiel einen Steuerbescheid.",
        steps: [
          "Wählen Sie `+ Freie Zusatzfrage` und schreiben Sie die Frage.",
          "Stellen Sie bei `Typ` die Option `Datei` ein.",
          "Wählen Sie bei `Erlaubte Dateiarten` aus `PDF`, `JPG` und `PNG`.",
          "Wählen Sie bei `Höchstens je Datei` eine Größe von 1 bis 10 MB.",
          "Sollen Eltern mehrere Dateien schicken? Wählen Sie `Mehrere Dateien erlauben`. Dann sind bis zu 5 Dateien möglich.",
        ],
      },
      {
        title: "Die Einkommenserklärung hinzufügen",
        description:
          "Die Vorlage fragt die Einkommensstufe ab und verlangt einen Nachweis, wenn nicht der volle Beitrag gilt.",
        steps: [
          "Wählen Sie `+ Einkommenserklärung`.",
          "moto fügt zwei Fragen hinzu: `Einkommensstufe` und `Einkommensnachweis`.",
          "Passen Sie bei `Einkommensstufe` die `Auswahloptionen` an Ihre Beitragsstufen an.",
          "Der `Einkommensnachweis` erscheint nur, wenn Eltern nicht `Voller Beitrag` wählen.",
        ],
      },
      {
        title: "PDF-Dateien zu einem Hinweis hinzufügen",
        steps: [
          "Wählen Sie `Infotext` und schreiben Sie Ihren Hinweis.",
          "Wählen Sie beim Hinweis `PDFs hinzufügen`. Sie können mehrere Dateien gleichzeitig auswählen.",
          "Geben Sie jeder Datei unter `Bezeichnung für Eltern` einen verständlichen Namen.",
          "Fügen Sie weitere Dateien mit `Weitere PDFs hinzufügen` hinzu.",
          "Zum Austauschen wählen Sie bei der jeweiligen PDF `Ersetzen`.",
          "Zum Wegnehmen wählen Sie das Kreuz bei der jeweiligen PDF.",
          "Wählen Sie `Änderungen speichern` und danach `Vorschau öffnen`.",
        ],
      },
      {
        title: "Eltern unterschreiben lassen",
        description:
          "Die Eltern unterschreiben am Ende des Formulars mit dem Finger, dem Stift oder der Maus. Das Verfahren heißt einfache elektronische Signatur mit gezeichneter Unterschrift.",
        steps: [
          "Öffnen Sie Ihre Vorlage.",
          "Schalten Sie im Abschnitt `Unterschrift` den Schalter `Unterschrift erforderlich` ein.",
          "Wählen Sie `Änderungen speichern`.",
        ],
      },
      {
        title: "Die Vorlage in der Anmeldephase auswählen",
        description:
          "Eine Vorlage wirkt erst, wenn eine Phase sie benutzt. Ohne diesen Schritt sehen die Eltern weiter das Basisformular.",
        steps: [
          "Öffnen Sie `Anmeldephasen`.",
          "Öffnen Sie die Phase zum Bearbeiten.",
          "Wählen Sie im Abschnitt `Formular` die Option `Eigene Vorlage wählen`.",
          "Wählen Sie Ihre Vorlage aus der Liste.",
          "Speichern Sie die Phase.",
        ],
      },
      {
        title: "Eigene Texte für Eltern übersetzen",
        description:
          "Eltern wählen im Formular oben ihre Sprache. moto übersetzt nur die festen Texte. Ihre eigenen Fragen und Zustimmungen übersetzen Sie selbst.",
        steps: [
          "Öffnen Sie Ihre Vorlage und gehen Sie nach unten zu `Übersetzungen für Eltern`.",
          "Wählen Sie die Sprache, zum Beispiel `Russisch`.",
          "Links steht Ihr deutscher Text. Tragen Sie rechts bei `Übersetzung` den Text in der Sprache ein.",
          "Wählen Sie unten `Änderungen speichern`.",
          "Denselben Abschnitt finden Sie beim Bearbeiten einer Anmeldephase und eines Betreuungsangebots.",
        ],
      },
    ],
    result:
      "Eltern füllen in dieser Phase das Basisformular und Ihre zusätzlichen Fragen aus.",
    notes: [
      "Eine Vorlage gilt für alle Phasen, die sie auswählen. Sie brauchen nicht für jede Phase eine eigene.",
      "Mit `Vorschau öffnen` sehen Sie den Entwurf in einem neuen Tab, bevor Sie speichern.",
      "Die Zustimmungen `Datenschutzinformation`, `Fotoeinwilligung` und `E-Mail-Kontakt` tragen `Aus Einstellungen übernommen`. Sie kommen aus Ihren Einstellungen und lassen sich je Vorlage ausblenden.",
      "Über `+ Eigene Zustimmung hinzufügen` ergänzen Sie einen Text, der nur in dieser Vorlage steht.",

      "Jede PDF-Datei darf höchstens 10 MB groß sein.",
      "Eltern können alle Dateien ohne Elternkonto öffnen und herunterladen.",
      "Infotexte und Blöcke mit `Nur Hinweis` brauchen keine Zustimmung.",
      "Auch bei Zustimmungen können Sie mehrere PDF-Dateien hinzufügen.",
      "Beim Verwerfen werden neue, ungespeicherte Dateien entfernt. Frühere Formularfassungen behalten ihre Dateien.",
      "Eine Frage vom Typ `Datei` gilt immer für die ganze Anmeldung, nicht pro Kind.",
      "Mit `Unterschrift erforderlich` können Eltern die Anmeldung erst nach der Unterschrift absenden. Ändern sie die Anmeldung später, unterschreiben sie neu.",
      "Statt zu zeichnen können Eltern ihren Namen tippen. Das steht im Nachweis als `Name getippt`.",
      "Hochgeladene Nachweise öffnet nur, wer das Recht `Nachweise aus Anmeldungen ansehen` hat. Die Leitung hat es immer.",
      "Nachweise werden nach einer festen Frist gelöscht. Die Frist legt das moto-Team fest.",
    ],
    differences: [
      "Bei einem `Stammdaten-Vorschlag` können Sie Beschriftung und Typ nicht ändern. Beides ist fest, damit die Antwort beim Kind ankommt.",
      "Manche Basisfelder stehen auf `Immer Pflicht` und lassen sich nicht abschalten, zum Beispiel `E-Mail (Elternteil)`.",
      "Steht bei einer Zustimmung `Ist für diese Vorlage ausgeblendet`, fragt dieses Formular sie nicht ab.",
    ],
    troubleshootingDetails: [
      "Die Eltern sehen Ihre Fragen nicht? Dann steht die Phase noch auf `Basisformular`. Wählen Sie dort `Eigene Vorlage wählen`.",
      "In der Phase steht `Noch keine eigenen Formulare vorhanden`? Dann ist die Vorlage noch nicht gespeichert.",
      "Eine Antwort taucht beim Kind nicht auf? Dann war es eine `Freie Zusatzfrage`. Nur `Stammdaten-Vorschläge` schreiben in die Daten des Kindes.",
      "Eltern lesen einen Text auf Deutsch, obwohl Sie ihn übersetzt haben? Dann haben Sie den deutschen Text danach geändert. Bei der Übersetzung steht `Bitte prüfen`. Passen Sie sie an oder wählen Sie `Passt noch`, und speichern Sie.",
      "Fehlt `Anmeldungen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentSetup,
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
