import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function appOverviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.appOverview,
    title: "Sich in moto zurechtfinden",
    question: "Wie ist moto aufgebaut?",
    summary:
      "Über die Navigation öffnen Sie die Bereiche, die Sie für Ihren Arbeitstag brauchen.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Am Computer",
        steps: [
          "Nutzen Sie die Seitenleiste links, um einen Bereich zu öffnen.",
          "Wählen Sie einen Eintrag mit Pfeil, um die zugehörigen Bereiche ein- oder auszublenden.",
          "Öffnen Sie oben rechts Ihr Profil, um Ihre Angaben anzusehen oder sich abzumelden.",
        ],
      },
      {
        // Beschreibt den Zweck jeder Gruppe, nicht ihre Eintraege. Welche
        // Seiten in einer Gruppe stehen, haengt von Rolle und Einstellungen
        // ab -- eine Aufzaehlung waere in der Haelfte der OGS falsch.
        // Quelle der Ordnung: STAFF_NAV_GROUPS in lib/staff-navigation.ts.
        title: "Wie die Seitenleiste geordnet ist",
        description:
          "Ganz oben steht die `Startseite`. Darunter bündeln Gruppen, was zusammengehört.",
        ordered: false,
        steps: [
          "`Tagesbetrieb`: alles für den laufenden Betreuungstag. Diese Gruppe ist schon aufgeklappt.",
          "`Eltern`: was Sie mit den Familien austauschen.",
          "`Team`: Ihre Arbeitszeit, Ihr Kalender und Ihre Kolleginnen und Kollegen.",
          "`Planung`: die Pläne Ihrer OGS.",
          "`Verwaltung`: Stammdaten, Auswertungen und Dateien.",
          "In jeder Gruppe steht oben, was im Alltag am häufigsten gebraucht wird.",
          "Sie sehen nur Gruppen, in denen etwas für Sie dabei ist.",
          "Ganz unten stehen `Notfall` und `Hilfe`. Wer die OGS verwaltet, findet dort auch `Einstellungen`.",
        ],
      },
      {
        title: "Auf dem Handy oder Tablet",
        steps: [
          "Nutzen Sie die Leiste unten, um häufige Bereiche zu öffnen.",
          "Wählen Sie `Mehr`, um die übrigen Bereiche zu sehen.",
          "Öffnen Sie oben rechts Ihr Profil, um Ihre Angaben anzusehen oder sich abzumelden.",
        ],
      },
      {
        // Labels aus home/page.tsx (Knopf „Anpassen", „Fertig") und
        // home-board.tsx („Bausteine", „Standardansicht wiederherstellen").
        title: "Die Startseite anpassen",
        description:
          "Die `Startseite` besteht aus Karten. Welche dort stehen, legen Sie selbst fest.",
        steps: [
          "Wählen Sie auf der `Startseite` oben rechts das Symbol `Anpassen`.",
          "Wählen Sie `Bausteine` und dann eine Karte, zum Beispiel `Kinder`.",
          "Die Karte steht jetzt unten. Am Computer ziehen Sie sie an ihren Platz.",
          "Wählen Sie `Fertig`.",
        ],
      },
      {
        title: "Hilfe öffnen",
        steps: [
          "Sehen Sie oben rechts ein Fragezeichen? Es öffnet die passende Anleitung.",
          "Am Handy oder Tablet steht es rechts im Seitenkopf.",
          "Oder wählen Sie `Hilfe`: am Computer unten in der Seitenleiste, auf dem Handy unter `Mehr`.",
        ],
      },
    ],
    result:
      "Sie wissen, wo Sie die wichtigsten Bereiche und die passende Hilfe finden.",
    differences: [
      "Welche Bereiche Sie sehen, hängt von Ihrer Rolle und den Einstellungen Ihrer OGS ab.",
      "Unter `Bausteine` fehlt eine Karte? Dann steht sie schon auf Ihrer Startseite. Oder Ihre Rolle oder Ihre OGS sieht sie nicht vor.",
      // Kopfzeile aus BrandTraegerLine (#4656): nur bei einem Träger mit
      // mehreren Schulen, und nur am Computer, wo auch der Schulname steht.
      "Steht am Computer oben links unter dem Namen Ihrer Schule `Träger:` und ein Name? Das ist der Träger Ihrer Schule. Die Zeile gibt es nur, wenn Ihr Träger mehrere Schulen hat.",
    ],
    notes: [
      "Die Karte `Kinder` zählt die Kinder nach `Anwesend`, `Schule`, `Zuhause`, `Krank` und `Entschuldigt`. Ein Klick auf eine Zahl öffnet `Alle Kinder` mit diesem Filter. `Entschuldigt` zeigt auch Kinder auf Klassenfahrt. `Zuhause` zeigt auch Kinder, die heute nicht kommen.",
      "Zu viel verändert? Wählen Sie in `Anpassen` den Knopf `Standardansicht wiederherstellen`.",
    ],
    related: [
      HELP_TOPICS.login,
      HELP_TOPICS.mySchedule,
      HELP_TOPICS.studentSearch,
    ],
  };
}
