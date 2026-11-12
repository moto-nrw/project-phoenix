import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Reiter `Notizen` der Kindakte (#3632). Sichtbar fuer jeden, der das
 * Kind lesen darf; WELCHE Notizen erscheinen, entscheidet der Server je
 * Notiz. Deshalb tragen die Schritte die Reichweite als Auswahl, nicht als
 * Recht.
 *
 * Live geprueft: `Neue Notiz` oeffnet ein Formular im Reiter, kein Fenster.
 * Unter `Bezug` stehen `Nur das Kind`, die OGS-Gruppe und die Angebote, in
 * denen das Kind heute angemeldet ist (#4268). Die Leitungs-Reichweite
 * erscheint erst mit einem Bezug: `Gruppenleitung` bei der OGS-Gruppe,
 * `Angebotsleitung` bei einem Angebot. Der Bezug bleibt beim Bearbeiten fest.
 * An keine Einstellung gebunden.
 *
 * Team informieren (#6854): `Speichern und Team informieren` steht neben
 * `Speichern` und geht nur mit `Ganzes Team`. Wer informiert wurde, sieht
 * oben in der Kopfzeile unter `Neu für Sie` mit rotem Zähler; das Öffnen
 * der Notizen des Kindes hakt sie ab.
 */
export function studentNotesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.studentNotes,
    title: "Notizen zu einem Kind",
    question: "Wie halte ich etwas zu einem Kind fest?",
    summary:
      "Im Reiter `Notizen` der Kindakte. Sie wählen bei jeder Notiz, wer sie liest.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "NotePencil",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Notiz schreiben",
        steps: [
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie den Reiter `Notizen`.",
          "Wählen Sie `Neue Notiz`.",
          "Schreiben Sie kurz und sachlich, was war.",
          "Wählen Sie unter `Bezug`, worum es geht: `Nur das Kind`, die OGS-Gruppe oder ein Angebot.",
          "Wählen Sie unter `Wer sieht die Notiz?` die Reichweite.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine wichtige Notiz mit dem Team teilen",
        steps: [
          "Schreiben Sie die Notiz wie oben.",
          "Wählen Sie unter `Wer sieht die Notiz?` die Reichweite `Ganzes Team`.",
          "Wählen Sie `Speichern und Team informieren`.",
          "moto sagt Ihnen danach, wie viele Personen informiert sind.",
        ],
      },
      {
        title: "Eine geteilte Notiz lesen",
        steps: [
          "Oben in der Kopfzeile erscheint ein Zeitungssymbol mit rotem Zähler.",
          "Tippen Sie darauf. Die Liste `Neu für Sie` zeigt die Notiz.",
          "Tippen Sie auf das Kind. Die Notizen des Kindes öffnen sich.",
          "Die geteilte Notiz trägt `Neu`. Der Zähler geht zurück.",
        ],
      },
      {
        title: "Eine Notiz zu einem Angebot",
        steps: [
          "Wählen Sie unter `Bezug` das Angebot, zum Beispiel eine AG.",
          "Ein Eintrag mit Tag beschreibt dann diesen einen Termin.",
          "In der Liste steht das Angebot neben dem Datum.",
        ],
        ordered: false,
      },
      {
        title: "Die drei Reichweiten",
        steps: [
          "`Ganzes Team`: alle, die das Kind sehen dürfen.",
          "`Betreuungsteam des Kindes`: wer das Kind wirklich hat, über seine Gruppe, seine Klasse oder ein Angebot.",
          "`Gruppenleitung`: nur die Leitung der OGS-Gruppe, zu der die Notiz gehört.",
          "`Angebotsleitung`: nur die Leitung des Angebots, zu dem die Notiz gehört.",
        ],
        ordered: false,
      },
      {
        title: "Dauerhafte Hinweise",
        steps: [
          "Wählen Sie beim Schreiben unter `Art` den Eintrag `Dauerhafter Hinweis`.",
          "Der Hinweis steht danach auch im Reiter `Stammdaten`.",
        ],
      },
      {
        title: "Eine Notiz für die Schule freigeben",
        steps: [
          "Setzen Sie beim Schreiben oder Bearbeiten den Haken bei `Für die Schule freigeben`.",
          "Wählen Sie `Speichern`.",
          "An der Notiz steht danach `Für die Schule freigegeben`.",
          "Zum Zurücknehmen entfernen Sie den Haken wieder und wählen `Speichern`.",
        ],
      },
    ],
    result:
      "Die Notiz steht sofort in der Kindakte, mit Datum und Ihrem Namen.",
    notes: [
      "Eltern sehen Notizen nie.",
      "Gesundheitsangaben gehören nicht in eine Notiz, sondern in die `Gesundheitsinformationen` der Stammdaten.",
      "Ändern kann eine Notiz nur, wer sie geschrieben hat.",
      "`Speichern` allein informiert niemanden. Nur `Speichern und Team informieren` zeigt die Notiz allen im Team als neu. Auf dem Handy kommt zusätzlich eine Mitteilung, wenn Sie Mitteilungen eingeschaltet haben.",
      "Entfernen kann sie die Leitung der Gruppe oder des Angebots.",
      "Eine freigegebene Notiz liest nur die Lehrkraft der Klasse des Kindes, in moto schule. Die Schulverwaltung sieht sie nicht. Ohne Haken bleibt jede Notiz bei der OGS.",
      "Die `Bemerkung` an der Anwesenheit eines Angebots ist nur ein kurzer Satz zur Anwesenheit. Beobachtungen wie Verhalten oder einen Vorfall schreiben Sie als Notiz, mit dem Angebot als `Bezug`.",
    ],
    differences: [
      "Sie sehen weniger Notizen als eine Kollegin? Dann hat sie das Kind, Sie nicht. Die Reichweite entscheidet das je Notiz.",
      "Ein Hinweis trägt `Übernommen aus den Betreuernotizen`? Der stammt aus dem früheren Textfeld und hat deshalb keinen Verfasser.",
      "Auf dem Handy gibt es keine Kopfzeile. Dort trägt `Mehr` unten den roten Zähler, und im Menü steht die Liste `Neu für Sie`.",
    ],
    troubleshootingDetails: [
      "Es fehlt der Reiter `Notizen`? Dann darf Ihre Rolle keine Notizen sehen. Fragen Sie Ihre Leitung.",
      "Ein Angebot fehlt unter `Bezug`? Dort stehen nur Angebote, in denen das Kind heute angemeldet ist.",
      "Es fehlt `Gruppenleitung` oder `Angebotsleitung`? Wählen Sie zuerst unter `Bezug` die Gruppe oder das Angebot.",
      "`Speichern und Team informieren` lässt sich nicht wählen? Das geht nur mit `Ganzes Team` unter `Wer sieht die Notiz?`.",
      "moto meldet `Die Notiz ist gespeichert. Das Team ist noch nicht informiert.`? Wählen Sie in der Meldung `Wiederholen`.",
    ],
    related: [HELP_TOPICS.leadManageStudent, HELP_TOPICS.studentSearch],
  };
}
