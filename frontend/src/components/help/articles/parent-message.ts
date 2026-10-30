import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function parentMessageTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentMessage,
    title: "Eltern eine Nachricht schreiben",
    question: "Wie schreibe ich Eltern eine Nachricht?",
    summary:
      "Starten Sie eine Unterhaltung mit einer Bezugsperson oder antworten Sie auf eine Nachricht.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "MessageSquareText",
    steps: [],
    instructionGroups: [
      {
        title: "Eine neue Unterhaltung beginnen",
        steps: [
          "Öffnen Sie unter `Eltern` den Bereich `Nachrichten`.",
          "Wählen Sie `Neue Nachricht`.",
          "Wählen Sie das Kind aus der Liste. Über das Suchfeld finden Sie es schneller.",
          "Wählen Sie `Weiter` und dann die passende Bezugsperson.",
          "Schreiben Sie Ihre Nachricht in das Textfeld.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Eine Unterhaltung nur für die Leitung beginnen",
        description:
          "Das geht nur mit dem Recht `Vertrauliche Elternnachrichten lesen` und wenn die Schule `Nachrichten nur an die Leitung (Eltern)` eingeschaltet hat.",
        steps: [
          "Wählen Sie unter `Nachrichten` den Knopf `Neue Nachricht`.",
          "Wählen Sie das Kind und dann `Weiter`.",
          "Wählen Sie bei `Wer liest auf Ihrer Seite mit?` den Punkt `Nur Leitung`.",
          "Wählen Sie die Bezugsperson.",
        ],
      },
      {
        title: "Auf eine Nachricht antworten",
        steps: [
          "Öffnen Sie die Unterhaltung im Posteingang.",
          "Lesen Sie den bisherigen Verlauf.",
          "Schreiben Sie Ihre Antwort in `Nachricht an die Eltern schreiben…`.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Eine Unterhaltung wieder als ungelesen markieren",
        description:
          "So sieht das ganze Team: Hier ist noch etwas offen. Die Eltern merken davon nichts.",
        steps: [
          "Öffnen Sie die Unterhaltung im Posteingang.",
          "Öffnen Sie oben das Menü mit den drei Punkten.",
          "Wählen Sie `Als ungelesen markieren`.",
          "moto bringt Sie zurück zum Posteingang.",
        ],
      },
      {
        title: "Alle Nachrichten für sich als gelesen markieren",
        description:
          "Ihre Zahl bei `Nachrichten` kann bleiben. Vom Team als ungelesen markierte Unterhaltungen bleiben ungelesen. Für Ihr Team und die Eltern ändert sich nichts.",
        steps: [
          "Öffnen Sie den Posteingang unter `Nachrichten`.",
          "Öffnen Sie oben das Menü mit den drei Punkten.",
          "Wählen Sie `Alle als gelesen markieren`.",
        ],
      },
      {
        title: "Festlegen, was Ihre Zahl bei Nachrichten zählt",
        description:
          "Beantworten andere die Elternnachrichten? Dann können Sie die Zahl für sich ändern. Für Ihr Team ändert sich nichts.",
        steps: [
          "Öffnen Sie oben rechts Ihr `Profil`.",
          "Gehen Sie zu `Zahl bei Nachrichten`.",
          "Wählen Sie `Alle Nachrichten`, `Nur Kinder aus meinen Gruppen` oder `Keine Zahl anzeigen`.",
          "moto speichert Ihre Wahl sofort.",
        ],
      },
    ],
    result:
      "Die Bezugsperson sieht die Nachricht in der Eltern-App. Sie wird dort im Namen der OGS angezeigt.",
    differences: [
      "Sehen Sie `Nachrichten` nicht? Vielleicht ist die Funktion ausgeschaltet.",
      "Möglicherweise fehlt Ihnen auch der Zugriff auf die betroffenen Kinder.",
      "Fehlen bei einer Anfrage `Anfrage ansehen` oder `Anfrage bearbeiten`? Dann dürfen Sie die Anfrage für dieses Kind nicht prüfen.",
      "Steht beim Kind `Kein Eltern-Zugang`? Dann kann niemand die Nachricht in der Eltern-App lesen. Den Zugang der Eltern legen Sie beim Kind fest.",
      "Steht bei einer Unterhaltung `Nur Leitung`? Dann lesen sie nur die Bezugsperson und die Leitung. Begonnen hat sie die Bezugsperson oder jemand aus der Leitung. Ohne das Recht `Vertrauliche Elternnachrichten lesen` sehen Sie solche Unterhaltungen nicht.",
      "Eltern können nur an die Leitung schreiben, wenn unter `Einstellungen` der Schalter `Nachrichten nur an die Leitung (Eltern)` an ist und jemand das Recht `Vertrauliche Elternnachrichten lesen` hat. Das Recht vergeben Sie unter `Rollen`.",
    ],
    notes: [
      "Über dem Schreibfeld steht, wer die Unterhaltung lesen kann: die eine Bezugsperson und alle Mitarbeitenden oder nur die Leitung. Andere Bezugspersonen des Kindes lesen nicht mit.",
      "Schreiben Sie persönliche Angaben nur in die Unterhaltung der richtigen Bezugsperson.",
      "Mit `Nur ungelesen` sehen Sie nur neue Unterhaltungen.",
      "Eine als ungelesen markierte Unterhaltung bleibt für alle ungelesen, bis jemand aus dem Team sie öffnet oder antwortet.",
      "Die Eltern sehen `Gelesen` erst, wenn jemand aus dem Team die Unterhaltung öffnet.",
      "Die Nachrichten stehen immer im Posteingang, auch wenn Ihre Zahl sie nicht zählt.",
      "Über `Zum Kinderprofil` wechseln Sie direkt zu den Angaben des Kindes.",
    ],
    related: [HELP_TOPICS.parentRequests, HELP_TOPICS.studentSearch],
  };
}
