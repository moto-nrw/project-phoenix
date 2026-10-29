import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Regeltermine, deren Kinderliste von den Anmeldungen abweicht (#5103):
 * Hinweis an der Karte, „Zu prüfen“, „Mit Angebot verbinden“ mit Vorschau
 * und „Bleibt von Hand gepflegt“; mehrere Angebote zusammen (#5139). Beschriftungen aus
 * roster-deviation-hint.tsx, template-list.tsx, connect-offering-modal.tsx.
 */
export function leadRosterDeviationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadRosterDeviation,
    title: "Regeltermin mit Angeboten verbinden",
    question:
      "Warum passt die Kinderliste eines Regeltermins nicht zu den Anmeldungen?",
    summary:
      "moto zeigt, wenn die Kinderliste eines Regeltermins nicht zu den Anmeldungen passt. Mit Angeboten verbunden, folgt die Kinderliste den Anmeldungen.",
    group: "planung",
    audience: "lead",
    icon: "ListChecks",
    requirements: [
      "Sie dürfen den Betreuungsplan bearbeiten.",
      "Ihre Schule nutzt Betreuungsangebote mit Anmeldungen.",
    ],
    steps: [
      "Öffnen Sie `Betreuungsplan` und wählen Sie oben `Serien`.",
      "Wählen Sie `Zu prüfen`. Dann sehen Sie nur Regeltermine mit dem Hinweis `Weicht von Anmeldungen ab`.",
      "Lesen Sie den Hinweis. Er nennt die passenden Angebote und die Kinder, die dort nicht angemeldet sind.",
      "Wählen Sie `Mit Angebot verbinden`.",
      "Prüfen Sie unter `Angebote` die gewählten Angebote. Sie können weitere Angebote dazu wählen oder abwählen.",
      "Prüfen Sie die Vorschau. Sie zeigt für jeden Wochentag, wer dazukommt und wer wegfällt.",
      "Wählen Sie `Verbinden`.",
    ],
    result:
      "Der Regeltermin zeigt `Automatisch`. An- und Abmeldungen im Angebot kommen ab dem nächsten Termin von selbst in die Kinderliste.",
    notes: [
      // #5139: mehrere Angebote zusammen als Quelle.
      "Passen mehrere Angebote zusammen, etwa `Randstunde` und `Ganztag`? Dann schlägt moto beide vor. Ein Kind kommt, wenn es für eines davon angemeldet ist. So fällt kein Ganztagskind aus der Randstunde.",
      "Pflegen Sie die Kinder bewusst selbst? Wählen Sie `Bleibt von Hand gepflegt`. Dann verschwindet der Hinweis für diesen Regeltermin.",
      "Den Hinweis holen Sie zurück: Öffnen Sie einen Termin des Regeltermins und wählen Sie `Hinweis wieder zeigen`.",
      "`Manuell` heißt: An- und Abmeldungen werden nicht übernommen. Die Kinder pflegen Sie selbst.",
      "Die Startseite zeigt die Aufgabe `Regeltermine prüfen`, solange ein Regeltermin abweicht.",
      "Wechseln Sie beim Bearbeiten die `Zielgruppe` auf `Angebot`, zeigt moto dieselbe Vorschau pro Wochentag.",
      "Beim Anlegen eines Regeltermins schlägt moto ein passendes Angebot vor. Mit `Angebot übernehmen` verbinden Sie den neuen Regeltermin gleich mit dem Angebot. Schlägt moto mehrere Angebote zusammen vor, heißt der Knopf `Angebote übernehmen`.",
      "Passen beim Anlegen mehrere Angebotskombinationen gleich gut? Wählen Sie einen der Knöpfe mit den Angebotsnamen. Er übernimmt alle dort genannten Angebote zusammen.",
      "Haben Kinder ein Angebot gebucht, aber kein Regeltermin übernimmt sie? Dann steht über den Regelterminen die Liste `Angebote ohne Regeltermin`. Jede Zeile nennt Angebot, Klassen und Wochentage. Wählen Sie `Regeltermin anlegen`: Der neue Regeltermin übernimmt die Kinder dieses Angebots. Oder verbinden Sie einen bestehenden Regeltermin mit dem Angebot.",
      "Brauchen diese Kinder bewusst keinen eigenen Regeltermin? Wählen Sie `Kein Regeltermin nötig`. Das gilt für den gewählten Zeitraum. Der Hinweis kommt wieder, sobald Kinder einer anderen Klasse betroffen sind. Das gilt auch für Ferienangebote.",
      "Diesen Hinweis holen Sie zurück: Klappen Sie `Ohne eigenen Regeltermin` auf und wählen Sie `Hinweis wieder zeigen`.",
      // #5137: Weg aus der Rückmeldung nach einer Buchungsänderung.
      "Nach einer Abmeldung führt `Mit Angebot verbinden` in der Rückmeldung direkt zu diesem Schritt. Findet moto kein passendes Angebot, öffnet sich der Regeltermin zum Bearbeiten. Wechseln Sie im Schritt `Personal und Kinder` die `Zielgruppe` auf `Angebot`.",
    ],
    differences: [
      "Hat der Regeltermin mehrere Abschnitte, etwa nach einer Änderung `Ab jetzt dauerhaft`? `Mit Angebot verbinden` stellt alle Termine ab dem nächsten Termin um. Das Bearbeiten ändert nur einen Abschnitt; die Vorschau nennt dann die Tage, die gleich bleiben.",
      // #5138: Passen Angebote gleich gut, wählt die Schule.
      "Nennt der Hinweis Angebote mit „oder“ und keine Zahl? Dann passen mehrere Angebote gleich gut. moto wählt keins vor. Wählen Sie in `Mit Angebot verbinden` unter `Angebote` die passenden aus.",
      "Sie sehen keine Knöpfe am Hinweis? Dann dürfen Sie den Plan nur ansehen.",
    ],
    related: [HELP_TOPICS.leadCarePlan, HELP_TOPICS.leadEnrollmentSetup],
  };
}
