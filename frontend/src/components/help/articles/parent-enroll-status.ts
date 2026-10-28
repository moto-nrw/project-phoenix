import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Statusseite ueber den Link aus der Bestaetigungs-E-Mail. */
export function parentEnrollStatusTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentEnrollStatus,
    title: "Eine Anmeldung weiter bearbeiten",
    question: "Wo sehe ich, was aus meiner Anmeldung geworden ist?",
    summary: "Der Link aus der Bestätigungs-E-Mail führt zum Stand.",
    group: "anmeldung",
    audience: "parent",
    icon: "Eye",
    requirements: ["Sie haben die Bestätigungs-E-Mail zu Ihrer Anmeldung."],
    steps: [],
    instructionGroups: [
      {
        title: "Den Stand ansehen",
        steps: [
          "Öffnen Sie den Link aus der Bestätigungs-E-Mail.",
          "Lesen Sie unter `Aktueller Stand`, wie weit die Prüfung ist.",
          "Unter `Was jetzt passiert` steht, was als Nächstes kommt.",
        ],
      },
      {
        title: "Angaben ändern",
        steps: [
          "Öffnen Sie die Statusseite.",
          "Tippen Sie auf `Anmeldung bearbeiten`.",
          "Ändern Sie die Angaben.",
          "Fragt das Formular nach Ihrer `Unterschrift`? Unterschreiben Sie neu und tippen Sie auf `Unterschrift bestätigen`.",
          "Tippen Sie auf `Änderungen speichern`.",
        ],
      },
      {
        title: "Eine Anmeldung zurückziehen",
        steps: [
          "Öffnen Sie die Statusseite.",
          "Tippen Sie bei dem Kind auf `Dieses Kind zurückziehen`.",
          "Bestätigen Sie mit `Endgültig zurückziehen`.",
        ],
      },
    ],
    result:
      "Sobald die OGS entschieden hat, sehen Sie das unter `Kinder` und bekommen eine E-Mail.",
    notes: [
      "Zurückziehen können Sie nicht selbst rückgängig machen. Wenden Sie sich dann an die OGS.",
      "Bei Rückfragen meldet sich die OGS über die angegebene E-Mail-Adresse.",
      "Haben Sie die Anmeldung unterschrieben, steht auf der Statusseite der Abschnitt `Unterschrift`. Mit `Nachweis als PDF` laden Sie Ihren Anmeldenachweis herunter.",
    ],
    differences: [
      "Steht dort `Bestätigung erforderlich`? Dann müssen Sie mit `Anmeldung bestätigen` zusagen, sonst läuft die Anmeldung zur Frist ab.",
      "Steht dort `Anmeldung wurde verlängert`? Dann müssen Sie nichts tun.",
      "Bei einem Kind steht `Diese Anmeldung kann nicht mehr online geändert werden`? Dann wenden Sie sich an die OGS.",
      "Steht dort `Noch kein Zugang zur Eltern-App`? Dann öffnen Sie die E-Mail „Einladung zum Eltern-Portal“ und legen dort ein Passwort fest. Vorher klappt die Anmeldung nicht.",
      "Steht dort `Zugang zur Eltern-App nicht möglich`? Dann wenden Sie sich an die OGS.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Status-Link sei ungültig? Prüfen Sie die Adresse aus der E-Mail oder fragen Sie bei der OGS nach.",
    ],
    related: [
      HELP_TOPICS.parentEnroll,
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}
