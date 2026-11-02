import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { unavailableNfcTopic, unknownNfcTopic } from "../help-article-shared";

export function nfcCheckInTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      "Scan",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      "Scan",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }

  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcCheckIn,
      title: "Kinder mit dem Armband ein- und auschecken",
      question: "Wie checken sich Kinder mit dem Armband ein und aus?",
      // Der Server schaltet bei einfacher Anwesenheit nur die Anwesenheit um
      // (`scanBinary`) und antwortet mit "Willkommen, ..." bzw. "Tschuess,
      // ...". Raum und taegliche Abmeldung sind in der Antwort leer bzw.
      // aus. Das Scannen selbst funktioniert also unveraendert.
      summary:
        "Kinder melden sich mit ihrem Armband an und ab. moto hält fest, ob ein Kind da ist.",
      group: "nfc",
      audience: ["caregiver", "lead"],
      icon: "Scan",
      requirements: [
        "Das Armband ist dem Kind zugewiesen.",
        "Am Tablet ist der Scan-Bildschirm geöffnet.",
      ],
      steps: [],
      instructionGroups: [
        {
          title: "Anmelden",
          steps: [
            "Das Kind hält sein Armband an den NFC-Sensor.",
            "moto bestätigt mit dem Namen des Kindes.",
          ],
        },
        {
          title: "Abmelden",
          steps: [
            "Das angemeldete Kind hält sein Armband erneut an den Sensor.",
            "moto meldet das Kind ab und bestätigt mit seinem Namen.",
          ],
        },
      ],
      result:
        "moto zeigt `Anwesend` oder `Zuhause`. Räume werden nicht erfasst.",
      differences: [
        "Die Frage `Wohin geht ...?` erscheint nicht. Das zweite Scannen meldet das Kind direkt ab.",
        "Eine Auswahl von Raum oder Aktivität am Tablet ändert nichts an der Anwesenheit.",
      ],
      troubleshooting: HELP_TOPICS.nfcProblem,
      related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.nfcCheckIn,
    title: "Kinder mit dem Armband ein- und auschecken",
    question: "Wie checken sich Kinder mit dem Armband ein und aus?",
    summary:
      "Kinder melden sich mit ihrem Armband bei einer Aufsicht an. Danach wählen sie ihr nächstes Ziel.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Scan",
    requirements: [
      "Das Armband ist dem Kind zugewiesen.",
      "Am Tablet läuft eine Aufsicht.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Einchecken",
        steps: [
          "Das Kind hält sein Armband an den NFC-Sensor.",
          "Prüfen Sie die grüne Bestätigung mit Name, Abholzeit und Raum.",
          "Warten Sie, bis sich die Anzeige automatisch schließt.",
        ],
      },
      {
        title: "Auschecken oder den Ort wechseln",
        steps: [
          "Das bereits eingecheckte Kind hält sein Armband erneut an den Sensor.",
          "Das Kind wählt unter `Wohin geht ...?` das passende Ziel.",
          "Wählen Sie `Raumwechsel` für einen anderen betreuten Raum. Dort hält das Kind sein Armband erneut an.",
          "Offene Räume stehen mit ihrem Namen da, zum Beispiel `Turnhalle`. Ein Tipp trägt das Kind sofort dort ein.",
          "Wählen Sie `nach Hause`, wenn das Kind die OGS verlässt.",
          "Je nach Einstellung können auch `Schulhof` oder `Toilette` erscheinen.",
        ],
      },
    ],
    result:
      "moto aktualisiert den Aufenthaltsort. Bei einem Ortswechsel bleibt das Kind angemeldet. Erst nach `nach Hause` ist es abgemeldet.",
    differences: [
      "Welche Ziele angezeigt werden, legt Ihre OGS fest.",
      "Ein offener Raum erscheint nur, wenn die Leitung ihn unter `Räume` als `Offener Raum` freigegeben hat. Dort braucht es kein Tablet und keine Aufsicht.",
      "Eine tägliche Abmeldezeit kann verhindern, dass `nach Hause` zu früh gewählt wird.",
      "Nach dem Abmelden kann ein freiwilliges Tages-Feedback erscheinen.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.nfcSupervision],
  };
}
