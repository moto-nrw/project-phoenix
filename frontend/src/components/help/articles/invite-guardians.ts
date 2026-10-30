import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Das Einladen ins Elternportal. Der Weg des Teams laeuft ueber das Kind:
 * Reiter `Erziehungsberechtigte`, dann `Einladen` bei der Person
 * (student-guardian-manager.tsx). Ohne E-Mail-Adresse tut der Knopf nichts --
 * `handleInviteGuardian` bricht bei `!guardian.email` still ab.
 *
 * Die Seite `Elternzugänge` (`/admin/guardian-approvals`) ist NICHT die
 * Warteschlange fuer Einladungen des Teams. Sie sammelt nur, was Eltern
 * selbst anstossen, gesteuert von `guardians.parent_invite_mode`. Beides
 * steht hier zusammen, weil es dieselbe Frage beantwortet: wer bekommt
 * Zugang zu einem Kind. Deshalb zeigt auch das Fragezeichen dieser Seite
 * hierher.
 *
 * Eine eingeschraenkte `Portalrolle` (`Notfallkontakt`, `Nur Abholung`,
 * `Sozialdienst`) hat keinen Portalzugriff. Eine Einladung stuft sie hoch
 * und fragt vorher `Vollen Zugriff gewähren?`.
 */
export function inviteGuardiansTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadInviteGuardians,
    title: "Eltern ins Elternportal einladen",
    question: "Wie lade ich Eltern ein?",
    summary:
      "Sie laden jede Person beim Kind ein. Dafür brauchen Sie ihre E-Mail-Adresse.",
    group: "elternarbeit",
    audience: "lead",
    icon: "KeyRound",
    requirements: [
      "Das Kind ist in moto angelegt.",
      "Sie kennen die E-Mail-Adresse der Person.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Eine eingetragene Person einladen",
        description:
          "Meistens stehen die Eltern schon beim Kind. Dann sind es vier Schritte.",
        steps: [
          "Öffnen Sie in der Seitenleiste `Alle Kinder`.",
          "Wählen Sie das Kind aus der Liste.",
          "Wechseln Sie auf den Reiter `Erziehungsberechtigte`.",
          "Wählen Sie rechts neben der Person `Einladen`.",
        ],
      },
      {
        title: "Eine Person zuerst eintragen",
        description:
          "Steht die Person noch nicht beim Kind, tragen Sie sie vorher ein. Pflicht sind nur zwei Felder.",
        steps: [
          "Wählen Sie im Reiter `Erziehungsberechtigte` oben rechts `+ Hinzufügen`.",
          "Tragen Sie `Vorname` und `Nachname` ein.",
          "Tragen Sie unter `Kontaktdaten` die `E-Mail` ein. Ohne sie geht keine Einladung raus.",
          "Lassen Sie `Portalrolle` auf `Erziehungsberechtigte/r`.",
          "Wählen Sie unten `Hinzufügen`.",
          "Wählen Sie danach bei der Person `Einladen`.",
        ],
      },
      {
        title: "Eine Anfrage von Eltern freigeben",
        description:
          "Eltern können weitere Bezugspersonen einladen, zum Beispiel eine Großmutter. Je nach Einstellung landet das erst bei Ihnen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Elternzugänge`.",
          "Prüfen Sie bei der Anfrage Name und Kind.",
          "Wählen Sie `Freigeben`.",
        ],
      },
    ],
    result:
      "Die Person bekommt eine E-Mail mit einem Link. Sobald sie ihr Passwort festlegt, sieht sie ihr Kind im Elternportal.",
    notes: [
      "Hinter dem Namen steht, wie weit die Person ist: `Kein Konto`, `Einladung offen` oder `Konto aktiv`.",
      "Bei `Einladung offen` heißt der Knopf `Erneut einladen`. Damit geht die E-Mail noch einmal raus.",
      "Eine Person mit mehreren Kindern laden Sie nur einmal ein. Sie sieht danach alle ihre Kinder.",
    ],
    differences: [
      "Steht bei der Person `Konto aktiv, kein Portalzugriff`, heißt der Knopf `Zugriff gewähren`.",
      "Bei einer eingeschränkten `Portalrolle` wie `Notfallkontakt` oder `Nur Abholung` fragt moto zuerst `Vollen Zugriff gewähren?`. Erst `Zugriff gewähren` erteilt den Zugang.",
      "Unter `Elternzugänge` steht `Einladungen gehen ohne Freigabe raus`? Dann laden Eltern direkt ein. Über `Zu den Einstellungen` ändern Sie das.",
      "Dort steht `Eltern können derzeit niemanden einladen`? Dann dürfen nur Sie einladen.",
    ],
    troubleshootingDetails: [
      "`Einladen` bewirkt nichts? Dann fehlt die `E-Mail`. Tragen Sie sie über `Bearbeiten` nach.",
      "Die Person ist schon bei einem anderen Kind eingetragen? Wählen Sie `Vorhandene/n suchen` statt `+ Hinzufügen`. Sonst entstehen zwei Einträge für dieselbe Person.",
      "Fehlt `Elternzugänge` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
      "Eine Anfrage passt nicht? Wählen Sie `Ablehnen` und bestätigen Sie mit `Ablehnen`. Es wird kein Zugang gewährt.",
    ],
    related: [
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.parentMessage,
    ],
  };
}
