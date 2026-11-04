import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Rollen unter `/database/roles`, Rechteliste unter `/database/permissions`.
 * Vorgegebene Rollen (`role.isSystem`) haben keine Kopfaktionen
 * (roles-master-detail.tsx). Eine neue Rolle startet mit den Rechten ihrer
 * Vorlage; gesperrte Rechte bietet der Reiter nicht an (#4574).
 */
export function staffPermissionsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffPermissions,
    title: "Mitarbeitende und Rechte verwalten",
    question: "Wie lege ich fest, was eine Person sehen darf?",
    summary: "Jede Person hat eine Rolle. Die Rolle trägt die Rechte.",
    group: "personal",
    audience: "lead",
    icon: "ShieldCheck",
    steps: [],
    instructionGroups: [
      {
        title: "Eine eigene Rolle anlegen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Rollen`.",
          "Wählen Sie oben rechts `+ Rolle`.",
          "Tragen Sie `Name` und `Beschreibung` ein.",
          "Wählen Sie eine `Vorlage`: `Wie Betreuung`, `Wie Verwaltung` oder `Wie Lehrkraft`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Die Rechte einer Rolle ändern",
        steps: [
          "Öffnen Sie `Rollen` und wählen Sie die Rolle aus der Liste.",
          "Öffnen Sie den Reiter `Berechtigungen` und wählen Sie `Bearbeiten`.",
          "Haken Sie an, was die Rolle dürfen soll.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Die Rolle einer Person ändern",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
          "Wählen Sie die Person aus der Liste.",
          "Öffnen Sie den Reiter `Konto` und wählen Sie `Bearbeiten`.",
          "Wählen Sie bei `Rolle` die neue Rolle und dann `Speichern`.",
        ],
      },
    ],
    result:
      "Die neue Rolle gilt, sobald sich die Person das nächste Mal anmeldet.",
    notes: [
      "Eine neue Rolle bekommt einmal die Rechte ihrer Vorlage. Danach ändern Sie die Rechte nur noch an der Rolle selbst.",
      "`Wie Verwaltung` gibt fast alle Rechte der Verwaltung. Einiges kann nur die Rolle `Verwaltung` selbst, zum Beispiel Mitteilungen an Eltern schreiben.",
      "Eine Rolle aus `Wie Lehrkraft` arbeitet wie die Rolle `Lehrkraft` in moto schule. Wer sie hat, meldet sich dort an. Später lässt sie sich nicht gegen eine andere Rolle tauschen.",
      "Bei manchen Rechten steht ein Hinweis. Damit kann eine Person Rollen, Konten oder Einstellungen ändern. Geben Sie diese Rechte nur der Leitung.",
      "`Position` im Reiter `Konto` ist nur ein Text für die Personalliste. Sie ändert keine Rechte.",
      "Unter `Datenverwaltung` → `Berechtigungen` sehen Sie alle verfügbaren Rechte. Diese Übersicht können Sie nur ansehen. Die Rechte einer eigenen Rolle ändern Sie unter `Rollen`.",
      "Die Rolle entscheidet auch darüber, welche Menüpunkte jemand sieht.",
      // ADR 0091: die zwei Zugangsstufen externer Personen sind feste Rollen.
      "`AG-Leitung` ist für Personen, die eigene Termine betreuen. Sie sieht unter `Heute` nur ihre eigenen Termine. Dort startet sie den Termin, checkt die Kinder ein und aus und beendet ihn.",
      "`Gast` ist für Personen, die nur zuschauen. Sie sieht Kinder, Räume und AGs. Elterndaten, Nachrichten und Telefonnummern der Eltern sieht sie nicht. Ändern kann sie nichts.",
      "Mit `AG-Leitung` oder `Gast` sieht die Person keine Zeiterfassung, keine Einstellungen und kein Personal.",
    ],
    differences: [
      "moto bringt einige Rollen schon mit: `Verwaltung`, `Betreuung`, `Lehrkraft`, `Schulverwaltung`, `AG-Leitung` und `Gast`. Sie sind als `Vorgegeben` markiert.",
      "Bei einer vorgegebenen Rolle fehlen `Bearbeiten` und `Löschen`. Zum Ändern von Rechten legen Sie deshalb eine eigene Rolle an.",
      "Einige Rechte stehen nicht zur Auswahl. Sie gehören nur der Rolle `Verwaltung` oder dem moto-Team.",
    ],
    troubleshootingDetails: [
      "Eine Person sieht einen Bereich nicht? Prüfen Sie die Rechte ihrer Rolle.",
    ],
    related: [
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadMissingMenu,
      HELP_TOPICS.leadTeacherAccess,
    ],
  };
}
