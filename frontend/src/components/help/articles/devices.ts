import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/database/devices`. Nur mit NFC (NFC_ONLY_HREFS, sidebar.tsx:369). */
export function devicesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDevices,
    title: "NFC-Geräte verwalten",
    question: "Wie verwalte ich unsere NFC-Tablets?",
    summary: "Sie sehen alle Geräte Ihrer OGS und ihren Zustand.",
    group: "nfc",
    audience: "lead",
    icon: "Devices",
    steps: [],
    instructionGroups: [
      {
        title: "Die Geräte nachsehen",
        description:
          "Ein geliefertes Tablet steht schon in der Liste. Sie müssen es nicht anlegen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Geräte`.",
          "Wählen Sie ein Gerät aus der Liste.",
          "Lesen Sie bei `Verbindung`, ob das Gerät `Online` oder `Offline` ist.",
          "Lesen Sie bei `Letzter Standort`, wo es zuletzt benutzt wurde.",
          "Lesen Sie bei `Status`, ob es `Aktiv` ist.",
        ],
      },
      {
        title: "Ein Gerät von Hand anlegen",
        description:
          "Nur nötig, wenn ein Gerät nicht in der Liste steht. Fragen Sie vorher beim moto-Team nach.",
        steps: [
          "Wählen Sie oben rechts `+ Gerät`.",
          "Tragen Sie bei `Geräte-ID` die Kennung des Geräts ein.",
          "Wählen Sie bei `Gerätetyp` `Terminal` oder `Info-Point`.",
          "Tragen Sie bei `Gerätename` einen Namen ein, zum Beispiel `Eingang`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
    ],
    result:
      "Sie sehen für jedes Gerät, ob es erreichbar ist und wo es zuletzt stand.",
    notes: [
      "Steht bei `Letzter Standort` `Noch nicht verwendet`, wurde am Gerät noch nichts gescannt.",
      "Nach dem Anlegen zeigt moto einmalig einen Zugangscode. Nur das Gerät braucht ihn.",
      "Geben Sie diesen Code nicht weiter. Er ist danach nicht mehr zu sehen.",
    ],
    differences: [
      "Arbeitet Ihre OGS ohne NFC, fehlt `Geräte` in der `Datenverwaltung`.",
    ],
    troubleshootingDetails: [
      "Ein Gerät meldet sich nicht? Prüfen Sie `Verbindung` und `Status` in der Liste.",
      "`Offline` und der Strom stimmt? Prüfen Sie das WLAN am Standort des Geräts.",
    ],
    related: [
      HELP_TOPICS.leadNfcSettings,
      HELP_TOPICS.leadTabletSetup,
      HELP_TOPICS.nfcProblem,
      HELP_TOPICS.leadInfoDisplays,
    ],
  };
}
