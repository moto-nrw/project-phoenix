import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { unavailableNfcTopic, unknownNfcTopic } from "../help-article-shared";

export function nfcSupervisionTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      "TabletSmartphone",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      "TabletSmartphone",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcSupervision,
      title: "Eine Aufsicht am Tablet starten und beenden",
      question: "Wie starte oder beende ich eine Aufsicht am Tablet?",
      // Das Tablet fragt auch bei einfacher Anwesenheit nach Aktivitaet und
      // Raum, weil es die Einstellung noch nicht liest (PyrePortal kennt
      // `presence_mode` nur als Typ). Der Server verwirft die Auswahl:
      // `scanBinary` bekommt das Kommando mit der RoomID gar nicht erst.
      // Der Ablauf funktioniert also -- er hat nur einen Zwischenschritt
      // ohne Wirkung. Frueher stand hier, das Tablet sei unzuverlaessig und
      // man solle die App nehmen; das legte ein funktionierendes Geraet still.
      summary:
        "Am Tablet wählen Sie zuerst Aktivität und Raum. Bei einfacher Anwesenheit ändert diese Auswahl nichts.",
      group: "nfc",
      audience: ["caregiver", "lead"],
      icon: "TabletSmartphone",
      steps: [
        "Melden Sie sich am Tablet mit Ihrer PIN an.",
        "Wählen Sie eine Aktivität und einen Raum.",
        "Starten Sie die Aufsicht. So kommen Sie zum Scannen.",
        "Scannen Sie danach die Armbänder wie gewohnt.",
      ],
      result:
        "Die Kinder sind am Tablet an- und abgemeldet. moto hält nur fest, ob ein Kind da ist.",
      differences: [
        "Aktivität und Raum sind nur ein Zwischenschritt. moto speichert sie bei einfacher Anwesenheit nicht.",
      ],
      related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.tabletLogin],
    };
  }

  return {
    id: HELP_TOPICS.nfcSupervision,
    title: "Eine Aufsicht am Tablet starten und beenden",
    question: "Wie starte oder beende ich eine Aufsicht am Tablet?",
    summary:
      "Wählen Sie Aktivität, Betreuungsteam und Raum. Starten Sie danach die Aufsicht.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "TabletSmartphone",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aufsicht starten",
        steps: [
          "Melden Sie sich am Tablet an.",
          "Wählen Sie im `Menü` `Aufsicht starten`.",
          "Wählen Sie unter `Was machen wir?` die Aktivität und danach `Weiter`.",
          "Wählen Sie unter `Wer ist dabei?` mindestens eine Betreuungskraft und danach `Weiter`.",
          "Wählen Sie unter `Wo machen wir das?` den Raum und danach `Weiter`.",
          "Prüfen Sie die Zusammenfassung und wählen Sie `Aufsicht starten`.",
        ],
      },
      {
        title: "Eine Aufsicht beenden",
        steps: [
          "Wählen Sie oben rechts `Anmelden` und geben Sie die Geräte-PIN ein.",
          "Wählen Sie im `Menü` `Aufsicht beenden`.",
          "Bestätigen Sie mit `Ja, beenden`.",
          "Wählen Sie `Abmelden`, um das Menü wieder zu sperren.",
        ],
      },
    ],
    result:
      "Das Tablet zeigt Aktivität, Raum und Kinderzahl. Beim Beenden werden die Kinder abgemeldet.",
    notes: [
      "Unter `Letzte Aufsichten` starten Sie eine frühere Aufsicht neu. Aktivität, Team und Raum werden übernommen.",
      "Unter `Team anpassen` ändern Sie während der Aufsicht das Betreuungsteam.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.activeSupervision],
  };
}
