import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { parentAnnouncementSteps } from "../help-article-shared";

/**
 * Einverständnis (#3430): Eltern stimmen je Kind zu oder lehnen ab. Einfache
 * Erklärung per Knopfdruck, mit Fassung und Nachweis, auf Wunsch mit
 * gezeichneter Unterschrift (#4785). Eine reine Lesebestätigung ist ein
 * Elternbrief.
 */
export function parentDeclarationTopic(): HelpTopic {
  const steps = parentAnnouncementSteps(
    "Einverständnisse",
    "Einverständnis",
    "Titel",
  );
  return {
    id: HELP_TOPICS.leadParentDeclaration,
    title: "Ein Einverständnis einholen",
    question: "Wie hole ich das Einverständnis der Eltern ein?",
    summary:
      "Eltern stimmen für jedes Kind zu oder lehnen ab, zum Beispiel für einen Ausflug.",
    group: "elternarbeit",
    audience: "lead",
    icon: "ShieldCheck",
    steps: [
      // Ohne die letzten zwei Schritte (Weiter, Empfänger): die Einstellungen
      // stehen noch in Schritt 1, der Empfängerschritt heißt hier
      // `Wer soll gefragt werden?`.
      ...steps.slice(0, -2),
      "Wählen Sie unter `Wer muss antworten?`, ob eine sorgeberechtigte Person genügt oder alle antworten müssen.",
      "Tragen Sie bei Bedarf ein Datum bei `Frist (optional)` ein.",
      "Wählen Sie unten `Weiter`.",
      "Wählen Sie in `Schritt 2 von 2` unter `Wer soll gefragt werden?` die Empfänger.",
      "Wählen Sie `Veröffentlichen`.",
    ],
    result:
      "Die Eltern sehen die Anfrage im Eltern-Portal. Den Stand je Kind sehen Sie beim Öffnen unter `Stand der Antworten`.",
    notes: [
      "Mit `Widerruf erlauben` können Eltern eine Zustimmung später zurücknehmen, auch nach der Frist.",
      "Mit `Mit Unterschrift` unterschreiben Eltern ihre Zustimmung mit dem Finger oder der Maus. Ablehnen geht ohne Unterschrift. Die Unterschrift steht im Nachweis der Eltern und in `Bericht als PDF`.",
      "Mit `Passwort vor dem Antworten abfragen` geben Eltern vor jeder Antwort ihr Passwort ein.",
      "`Offene erinnern` schickt eine Erinnerung an alle, die noch nicht geantwortet haben.",
      "Unter `Herunterladen` lädt `Bericht als PDF` den Stand und alle Antworten als PDF herunter. `Verlauf als CSV` enthält dieselben Antworten als Tabelle.",
      "Das ist eine einfache Erklärung per Knopfdruck. Verlangt ein Gesetz eine Erklärung auf Papier, reicht sie nicht aus.",
    ],
    differences: [
      "Nach dem Veröffentlichen stehen Text und Dateien fest.",
      "Ziehen Sie ein Einverständnis zurück und ändern den Text, müssen die Eltern neu antworten. Die alten Antworten bleiben im Verlauf.",
    ],
    troubleshootingDetails: [
      "Sollen Eltern nur bestätigen, dass sie etwas gelesen haben? Schreiben Sie dann einen Elternbrief mit Lesebestätigung.",
      "Ein Kind steht auf `Niemand kann antworten`? Dann hat keine sorgeberechtigte Person ein Eltern-Konto. Laden Sie die Eltern ein.",
      "`Löschen` klappt nicht? Auf das Einverständnis gibt es schon Antworten. Ziehen Sie es stattdessen zurück.",
    ],
    related: [
      HELP_TOPICS.leadParentLetter,
      HELP_TOPICS.leadParentSurvey,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}
