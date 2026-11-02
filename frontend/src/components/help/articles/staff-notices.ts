import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Tagesinformationen unter `/tagesinformationen`: lesen alle, anlegen nur
 * Admins (page.tsx:183). Die Zielgruppen stehen in AUDIENCE_LABELS
 * (lib/staff-notices-api.ts:239).
 */
export function staffNoticesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffNotices,
    title: "Tagesinformationen an das Team schreiben",
    question: "Wie erreiche ich das ganze Team mit einem Hinweis?",
    summary: "Eine Tagesinformation sehen alle, die heute arbeiten.",
    group: "planung",
    audience: "lead",
    icon: "Megaphone",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf.",
      "Öffnen Sie `Tagesinformationen`.",
      "Wählen Sie `Neue Tagesinformation`.",
      "Tragen Sie `Titel` und `Hinweis` ein.",
      "Wählen Sie bei `Für wen` `Alle`, `Nur Betreuung` oder `Nur Lehrkräfte`.",
      "Wählen Sie bei `Wichtigkeit`, wie dringend der Hinweis ist.",
      "Wählen Sie bei `Woche`, ob der Hinweis sich wiederholt.",
      "Tragen Sie `Gilt ab` und bei Bedarf `Gilt bis (optional)` ein.",
      "Haken Sie unter `Wochentage` die Tage an, an denen der Hinweis erscheint.",
      "Wählen Sie `Speichern`.",
    ],
    result: "Der Hinweis steht sofort unter `Heute` für die gewählte Gruppe.",
    notes: [
      "Mit `Kenntnisnahme verlangen` sehen Sie später, wer den Hinweis bestätigt hat.",
      "`Nur Lehrkräfte` erreicht die Lehrkräfte in moto schule. Unter `Für wen` steht das auch im Fenster.",
    ],
    differences: [
      "Ein wichtiger Hinweis trägt `Wichtig`, ein abgeschalteter `Abgeschaltet`.",
      "Lesen können alle im Team. Anlegen und ändern darf nur die Leitung.",
    ],
    troubleshootingDetails: [
      "Beim Löschen gehen die bereits erfassten Kenntnisnahmen mit verloren.",
      "Soll die Nachricht an Eltern gehen? Nutzen Sie stattdessen eine Elternmitteilung.",
    ],
    related: [
      HELP_TOPICS.leadTeacherAccess,
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}
