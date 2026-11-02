import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function findStaffTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.findStaff,
    title: "Eine Person aus dem Team finden",
    question: "Wie finde ich eine Person aus dem Team?",
    summary: "Suchen Sie eine Person und sehen Sie, ob sie gerade da ist.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "UserRoundSearch",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mitarbeiter`.",
      "Geben Sie den Namen in das Suchfeld ein.",
      // Live geprueft: die Auswahl traegt ihren aktuellen Wert als
      // Beschriftung, anfangs `Alle`. Ein Knopf `Filter` existiert nicht.
      "Öffnen Sie bei Bedarf rechts neben der Suche die Auswahl `Alle`.",
      "Wählen Sie zum Beispiel `Anwesend`, `Unterwegs` oder `Krank/Urlaub`.",
      "Prüfen Sie auf der Karte, ob die Person gerade da ist.",
    ],
    // Ohne Aufsichten steht auf der Karte auch keine.
    result:
      presenceMode === "binary"
        ? "Sie sehen, ob die Person gerade arbeitet."
        : "Sie sehen, ob die Person gerade arbeitet. Manchmal steht dort auch ihre Aufsicht.",
    differences: [
      "Auch ohne Recht für Personalakten sehen Sie, wo die Person gerade ist. Die Karte öffnet dann keine Personalakte. Fragen Sie bei Bedarf Ihre Leitung.",
      "Möchten Sie der Person schreiben? Öffnen Sie den `Team-Chat`.",
      "Unter `Ehemalige` können Sie frühere Beschäftigungen nur lesen. Die Personalakte zeigt den Zeitraum, Stammdaten und frühere Arbeitszeiten. Welche Angaben Sie sehen, hängt von Ihren Rechten ab.",
      "Personaldokumente wurden beim Austritt endgültig gelöscht. Sie stehen auch in der früheren Personalakte nicht mehr bereit.",
      "Mit `Zurück zu den Mitarbeitenden` kommen Sie wieder zur Auswahl `Ehemalige`. Eine Wiederaufnahme erfolgt beim Einladen.",
    ],
    notes: [
      "Hat jemand früher bei Ihnen gearbeitet? Wählen Sie in der Auswahl `Ehemalige`. Die Karte zeigt den Zeitraum in moto. Öffnen Sie die Karte für die früheren Angaben.",
    ],
    image: "/help/screens/personal-ehemalige.webp",
    imageAlt:
      "Frühere Beschäftigung mit Zeitraum. Die Angaben sind nur lesbar.",
    related: [HELP_TOPICS.teamChat, HELP_TOPICS.activeSupervision],
  };
}
