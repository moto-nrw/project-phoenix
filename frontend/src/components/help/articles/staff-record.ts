import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/staff/[id]`, die sieben Reiter der Personalakte. Live geprueft an einer
 * Person mit Daten. Schwerpunkt ist das `Arbeitszeitmodell`: daran rechnet
 * jedes Stundenkonto, und es war in der Hilfe bisher nur eine Fehlerzeile.
 */
export function staffRecordTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffRecord,
    title: "Die Personalakte einer Person führen",
    question: "Wo pflege ich die Angaben zu einer Person?",
    summary:
      "Die Personalakte bündelt Arbeitszeit, Abwesenheiten und Unterlagen.",
    group: "personal",
    audience: "lead",
    icon: "FolderOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Die Akte öffnen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf.",
          "Öffnen Sie `Mitarbeiter`.",
          "Geben Sie den Namen in das Feld `Name suchen` ein.",
          "Wählen Sie die Karte der Person.",
        ],
      },
      {
        title: "Das Arbeitszeitmodell eintragen",
        description:
          "Ohne Modell hat die Person kein Wochensoll. Stundenkonto und Saldo haben dann nichts, womit sie vergleichen könnten.",
        steps: [
          "Wählen Sie oben `Arbeitszeitmodell`.",
          "Wählen Sie rechts `Bearbeiten`.",
          "Wählen Sie `Vorlage zuweisen` für ein Modell, das mehrere Personen teilen.",
          "Oder wählen Sie `Eigenes Modell` nur für diese Person.",
          "Wählen Sie bei `Rotation` zum Beispiel `Standard (1 Woche)` oder `A/B-Wochen (2 Wochen)`.",
          "Tragen Sie je Wochentag die Stunden ein, zum Beispiel `4,45`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Die weiteren Bereiche der Akte",
        ordered: false,
        steps: [
          "`Übersicht`: Stundenkonto, Urlaubstage und Krankheitstage des Jahres.",
          "`Zeiterfassung`: die einzelnen Buchungen dieser Person.",
          "`Abwesenheiten`: unter `Kontingente` der Resturlaub und die Tage eigener Abwesenheitsarten, dazu `Krank melden` und `Freizeitausgleich eintragen`.",
          "`Stammdaten`: Person, Kontakt, Arbeitsvertrag, Qualifikationen und Personalnummer.",
          "`Dokumente`: Dateien zur Akte, je mit Kategorie und Frist.",
          "`Klassen`: nur bei Lehrkräften.",
        ],
      },
    ],
    result:
      "Unter `Arbeitszeitmodell` zeigt die `Vorschau` das Wochensoll der nächsten vier Wochen.",
    notes: [
      "Qualifikationen sind Nachweise wie Erste Hilfe am Kind, Rettungsfähigkeit oder erweitertes Führungszeugnis, jeweils mit Ablaufdatum. Wie moto daran erinnert, steht unter `Nachweise`.",
      "Die Personalnummer braucht die spätere Abrechnung. Ohne sie fehlt die Person dort.",
      "In der `Übersicht` buchen Sie unter `Stundenkonto-Verwaltung` Auszahlung, Freizeitausgleich oder einen Eröffnungssaldo.",
    ],
    differences: [
      "Ändern Sie eine Vorlage, wirkt das künftig auf alle Personen mit dieser Zuordnung.",
      "Den Bereich `Klassen` gibt es nur bei Lehrkräften.",
      "Bankverbindung und Steuernummer sind verborgen. Jeder Abruf wird festgehalten.",
    ],
    troubleshootingDetails: [
      "Ein Saldo stimmt nicht? Prüfen Sie zuerst das `Arbeitszeitmodell`.",
      "Ein Bereich fehlt Ihnen? Dann fehlt Ihrer Rolle das Recht dafür.",
    ],
    related: [
      HELP_TOPICS.leadTargetOverride,
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadPayroll,
      HELP_TOPICS.leadStaffQualifications,
    ],
  };
}
