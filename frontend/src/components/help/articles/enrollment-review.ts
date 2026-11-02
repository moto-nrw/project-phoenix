import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Pruefung einer eingegangenen Anmeldung, `/admin/enrollments/[id]`. Die vier
 * Entscheidungen stehen in DECISION_ACTIONS (admin-enrollment-detail.tsx:100).
 */
export function enrollmentReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.enrollments,
    title: "Eingegangene Anmeldungen prüfen",
    question: "Wie prüfe ich eine neue Anmeldung?",
    summary: "Sie prüfen die Angaben und entscheiden je Kind.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "ListChecks",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Öffnen Sie eine Anmeldung aus der Liste.",
      "Prüfen Sie die `Angaben der Kinder` und die `Zusatzfragen (Eltern)`.",
      "Wählen Sie eine Entscheidung: `Bestätigen`, `Warteliste`, `Ablehnen` oder `Zur Prüfung`.",
    ],
    result:
      "Die Eltern sehen die Entscheidung bei ihrer Anmeldung. Bei `Bestätigen` wird das Kind übernommen.",
    notes: [
      "Sie entscheiden je Kind. Eine Anmeldung kann mehrere Kinder enthalten.",
      "Auf der Seite der Phase steht unter `Freie Plätze je Angebot`, wie viele Plätze noch frei sind. `Belegt` zählt die Kinder am vollsten Tag bis zum Ende der Phase.",
      "Hochgeladene Nachweise stehen bei `Zusatzfragen (Eltern)`. Mit `Öffnen` sehen Sie die Datei in einem neuen Tab.",
      "Hat die Vorlage `Unterschrift erforderlich`, steht die Unterschrift der Eltern im Abschnitt `Unterschrift`. Mit `Nachweis als PDF` laden Sie den Anmeldenachweis mit der Unterschrift herunter.",
      "Mit `Statusseite öffnen` sehen Sie, was die Eltern sehen.",
      // #5102: Auswirkung nach einer Korrektur unter `Nachbearbeitung`.
      "Angebote eines bestätigten Kindes ändern Sie unter `Nachbearbeitung` mit `Bearbeiten`. Danach steht dort `Auswirkung auf den Betreuungsplan`.",
      "Unter `Automatisch angepasst` stehen Regeltermine, die moto selbst geändert hat.",
      "Unter `Nicht angepasst, weil von Hand gepflegt` steht das Kind an abgemeldeten Tagen weiter im Plan. Mit `Dort auch entfernen` nehmen Sie es dort ab diesem Datum heraus. Vergangene Termine bleiben.",
      // #5497
      "Steht dort ein Enddatum, gibt es danach wieder eine passende Buchung. Das Kind wird dann nur bis zu diesem Tag herausgenommen.",
      "Haben Sie die Seite schon verlassen? Der Hinweis bleibt im `Betreuungsplan` unter `Ohne passende Buchung eingeplant`.",
      // #5137
      "Plant ein Regeltermin alle Kinder einer Klasse, eines Jahrgangs oder einer Gruppe, steht dort `Mit Angebot verbinden`. Entfernen hilft dort nicht: Bei der nächsten Planung steht das Kind wieder im Plan.",
      "Offene Rückfragen der Eltern stehen im Bereich `Anfragen`.",
      "Die Zahl an `Anmeldungen` zeigt, wie viele neue Anmeldungen Sie noch nicht gelesen haben.",
      "Gelesen gilt nur für Sie. Ihr Team hat eine eigene Zahl.",
      "Wenn Sie eine Anmeldung öffnen, gilt sie als gelesen.",
      "Offene Anmeldungen in aktiven Phasen markieren Sie im Menü `⋮` als gelesen oder ungelesen.",
      "Im Menü `⋮` oben im `Überblick` wählen Sie `Alle als gelesen markieren`.",
      "Ändern Eltern ihre Anmeldung, ist sie wieder ungelesen.",
      "Sie möchten bei jeder neuen Anmeldung eine E-Mail? Wählen Sie im Menü `⋮` oben im `Überblick` `E-Mail an mich bei neuer Anmeldung`.",
      "Ein Haken zeigt: Die E-Mail ist eingeschaltet. Noch einmal wählen schaltet sie aus. Das gilt nur für Sie.",
      "Die E-Mail geht an Ihre E-Mail-Adresse in moto.",
    ],
    differences: [
      "Der Überblick zählt je Phase, wie viele Eingänge offen, bestätigt und abgelehnt sind.",
      "Auf dem Handy erscheint nur ein Hinweis. Die Prüfung läuft am Computer.",
    ],
    troubleshootingDetails: [
      "Eine Anmeldung war versehentlich abgelehnt? Wählen Sie `Anmeldung wiederherstellen`.",
      "Bei einem Nachweis steht `Keine Berechtigung zum Öffnen`? Dann fehlt Ihnen das Recht `Nachweise aus Anmeldungen ansehen`. Fragen Sie Ihre Leitung.",
      "Bei einem Nachweis steht `Nachweis gelöscht (Löschfrist abgelaufen)`? Die Frist ist vorbei. Bitten Sie die Eltern bei Bedarf um einen neuen Nachweis.",
      "Bei der Unterschrift steht `Der gespeicherte Eintrag wurde nachträglich verändert`? Dann passt die Unterschrift nicht mehr zu ihrer Prüfsumme. Wenden Sie sich an das moto-Team.",
      "Beim Bestätigen erscheint `Das Kinderkontingent Ihrer Schule ist voll`? Dann bleibt die Anmeldung offen. Für weitere Kinder melden Sie sich beim moto-Team.",
      "Kinder, die schon aktiv oder vorgemerkt sind, können Sie auch bei vollem Kinderkontingent verlängern.",
      "Bei einer Anmeldung steht `Wegen Kinderkontingent offen`? Beim Schuljahreswechsel war das Kinderkontingent voll. Das Kind wurde nicht automatisch verlängert. Entscheiden Sie selbst.",
      "Trotz Haken kommt keine E-Mail? Schauen Sie im Spam-Ordner nach.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentSetup,
      HELP_TOPICS.leadEnrollmentCleanup,
      HELP_TOPICS.parentRequests,
    ],
  };
}
