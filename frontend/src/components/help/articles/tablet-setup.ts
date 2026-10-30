import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Das Aufstellen des Tablets, vom Karton bis zum Startbildschirm.
 *
 * Eigener Artikel, weil die NFC-Themen der Betreuung ein laufendes Geraet
 * voraussetzen: sie fangen beim Anmelden an. Das gedruckte Blatt im
 * Versandkarton (`/help/nfc/erste-schritte`) deckt dieselben drei Schritte
 * kuerzer ab.
 */
export function tabletSetupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadTabletSetup,
    title: "Ein neues NFC-Tablet aufstellen",
    question: "Wie stelle ich ein neues NFC-Tablet auf?",
    summary: "Standort wählen, Kabel einstecken, den Startbildschirm abwarten.",
    group: "einrichten",
    audience: "lead",
    icon: "PlugZap",
    requirements: ["Eine Steckdose in der Nähe des Standorts"],
    steps: [
      "Wählen Sie einen Ort, an dem die Kinder täglich vorbeikommen.",
      "Stellen Sie das Tablet auf einen Tisch oder hängen Sie es an die Wand.",
      "Achten Sie darauf, dass der NFC-Sensor am Fuß frei bleibt.",
      "Legen Sie das Kabel so, dass niemand darüber stolpert.",
      "Stecken Sie das Netzkabel in die Steckdose.",
      "Warten Sie ein bis zwei Minuten.",
      "Auf dem Bildschirm erscheint `Willkommen bei moto!`.",
    ],
    result:
      "Das Tablet ist bereit. Ihr Team kann sich mit der Geräte-PIN anmelden.",
    notes: [
      "Einen Einschaltknopf gibt es nicht. Das Tablet startet mit dem Strom.",
      "Auf der Rückseite sind Löcher für eine Wandhalterung.",
      "Das Tablet ist schon mit dem WLAN Ihrer Schule verbunden.",
      "Ein Netzwerkkabel können Sie hinten anschließen.",
    ],
    differences: [
      "Auf dem Tablet läuft nur moto. Andere Apps finden Sie dort nicht.",
      "Ein rotes Zeichen unten rechts heißt: die Internetverbindung fehlt.",
    ],
    troubleshootingDetails: [
      "Der Bildschirm bleibt dunkel? Prüfen Sie die Steckdose und warten Sie zwei Minuten.",
      "Das rote Zeichen bleibt? Prüfen Sie Ihr WLAN oder fragen Sie das moto-Team.",
    ],
    related: [
      HELP_TOPICS.tabletLogin,
      HELP_TOPICS.tagAssignment,
      HELP_TOPICS.leadNfcSettings,
      HELP_TOPICS.leadDevices,
    ],
  };
}
