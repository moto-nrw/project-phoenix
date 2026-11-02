import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Problemartikel der Leitung. Bewusst getrennt vom Betreuer-Thema
 * `missingMenu`: dort fehlt einem selbst etwas, hier einer anderen Person.
 */
export function leadMissingMenuTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMissingMenu,
    title: "Eine Person sieht einen Menüpunkt nicht",
    question: "Warum fehlt jemandem ein Bereich in der Seitenleiste?",
    summary: "Meist fehlt der Rolle das Recht für diesen Bereich.",
    group: "probleme",
    audience: "lead",
    icon: "EyeSlash",
    steps: [
      "Fragen Sie, welcher Bereich genau fehlt.",
      "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
      "Prüfen Sie, welche Rolle die Person hat.",
      "Öffnen Sie `Rollen` und wählen Sie diese Rolle.",
      "Wählen Sie `Berechtigungen` und prüfen Sie die Haken.",
      "Ergänzen Sie das fehlende Recht und speichern Sie.",
    ],
    result:
      "Der Bereich erscheint, sobald sich die Person das nächste Mal anmeldet.",
    notes: [
      "moto blendet einen Bereich aus, statt ihn zu sperren. Ein fehlender Menüpunkt ist kein Fehler.",
    ],
    differences: [
      "Manche Bereiche hängen nicht an Rechten, sondern an Funktionen Ihrer OGS.",
      "Manche Bereiche gehören zu einem Zusatzmodul. Hat Ihre OGS es nicht gebucht, sieht Ihr Team den Bereich nicht. Sie selbst sehen ihn mit Schloss.",
      "Ohne NFC fehlen `Aktivitäten` und `Geräte`. Bei einfacher Anwesenheit fehlen `Räume` und `Heute`.",
      "Ohne feste Gruppen fehlt `Gruppen`.",
      "`Info-Displays` hängt an einem eigenen Schalter in den `Einstellungen`, nicht an einem Recht.",
    ],
    troubleshootingDetails: [
      "Die Rolle bringt moto selbst mit? Dann lassen sich ihre Rechte nicht ändern. Legen Sie eine eigene Rolle an.",
      "Fehlt Ihnen selbst ein Bereich? Dann fehlt Ihrem Konto das Recht. Fragen Sie das moto-Team.",
    ],
    related: [
      HELP_TOPICS.leadStaffPermissions,
      HELP_TOPICS.leadModules,
      HELP_TOPICS.settings,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}
