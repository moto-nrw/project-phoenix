import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function roomsTopic(presenceMode: HelpPresenceMode): HelpTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.rooms,
      title: "Räume und ihre aktuelle Belegung ansehen",
      question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
      summary: "Bei einfacher Anwesenheit ordnet moto Kinder keinen Räumen zu.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "DoorOpen",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Prüfen Sie dort, welche Kinder anwesend oder abwesend sind.",
      ],
      result:
        "Der Bereich `Räume` wird nicht angezeigt. moto speichert dann keinen Aufenthaltsort.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.studentSearch],
    };
  }
  if (presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.rooms,
      title: "Räume und ihre aktuelle Belegung ansehen",
      question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
      summary:
        "Der Bereich ist nur sichtbar, wenn Ihre OGS Aufenthaltsorte erfasst.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "DoorOpen",
      steps: [
        "Prüfen Sie, ob `Räume` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie dort einen Raum und prüfen Sie die Belegung.",
        "Fehlt der Bereich? Öffnen Sie `Alle Kinder`, um die Anwesenheit zu prüfen.",
      ],
      result:
        "Raumbelegungen werden nur bei detaillierter Anwesenheit angezeigt.",
      related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.rooms,
    title: "Räume und ihre aktuelle Belegung ansehen",
    question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
    summary:
      "Prüfen Sie, welche Räume frei oder belegt sind und wer sich dort befindet.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "DoorOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Raum und seine Belegung ansehen",
        // #4372: die drei Begriffe einheitlich erklärt.
        description:
          "`Belegt` heißt: Im Raum läuft gerade eine Aktivität. Ein offener Raum ist für Ortswechsel freigegeben, auch wenn dort nichts läuft. Unter `Aktuelle Aufsicht` stehen nur Aufsichten, die gerade laufen.",
        steps: [
          "Öffnen Sie `Räume` in der Seitenleiste.",
          // #4372 (01.14): offene Räume stehen als eigene Gruppe oben.
          "Gibt es offene Räume, stehen sie oben unter `Offene Räume`. Alle anderen stehen darunter unter `Weitere Räume`.",
          "Klappen Sie eine Gruppe über den Pfeil rechts ein oder aus.",
          "Suchen Sie den Raum über `Raum suchen...`.",
          // Live geprueft: kein Knopf `Filter`, sondern ein Umschalter
          // `Alle` / `Belegt` / `Frei` und eine Auswahl `Alle Gebäude`.
          "Oder wählen Sie oben `Belegt` oder `Frei`.",
          "Grenzen Sie über `Alle Gebäude` auf ein Gebäude ein.",
          "Wählen Sie einen Raum aus.",
          "Prüfen Sie die aktuelle Aktivität, die Aufsicht und die Kinder im Raum.",
          // #4372: offene Räume sind nur noch über `Räume` erreichbar.
          "Bei einem offenen Raum wählen Sie oben `Zur Aufsicht`. Dort sehen Sie alle Kinder im Raum.",
          "Wählen Sie bei Bedarf ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      // #4768: Reiter `Plan` mit den Terminen des Raums laut Betreuungsplan.
      {
        title: "Sehen, wann ein Raum laut Plan frei ist",
        description:
          "Der Reiter `Plan` zeigt die Termine des Betreuungsplans in diesem Raum. Ändern können Sie den Plan dort nicht.",
        steps: [
          "Öffnen Sie `Räume` und wählen Sie einen Raum aus.",
          "Wählen Sie den Reiter `Plan`.",
          "Wählen Sie oben `Tag` oder `Woche`. Mit den Pfeilen wechseln Sie den Zeitraum.",
          "`Frei` heißt: In dieser Zeit ist im Raum nichts geplant.",
          "Ein durchgestrichener Termin mit `Abgesagt` belegt den Raum nicht.",
          "Wählen Sie `Im Betreuungsplan ansehen`, um den Plan dieses Raums dort zu öffnen. Planende sehen dort `Im Betreuungsplan bearbeiten`.",
          "Für den PDF-Aushang brauchen Sie auch das Recht, die Kinderliste zu sehen.",
          "Für einen Aushang öffnen Sie oben rechts das Menü mit den drei Punkten. Wählen Sie `Raumplan als PDF`. Sie erhalten die angezeigte Woche als PDF.",
        ],
      },
      {
        title: "Kinder ohne Raum in einen Raum setzen",
        description:
          "Führen Sie selbst die Aufsicht, stehen nur Ihre Räume zur Wahl. Mit dem Recht für alle Räume steht die ganze Liste da. Dann stehen auch offene Räume in der Liste, mit dem Zusatz `(offener Raum)`. So bleibt ein Kind nach seinem Angebot im Raum, ohne an einem Angebot teilzunehmen. Dieselbe Karte steht auch unter `Alle Kinder`, wenn Sie dort nach `Unterwegs` filtern.",
        steps: [
          "Wählen Sie oben die Karte `Unterwegs`.",
          "Wählen Sie die gewünschten Kinder aus.",
          "Wählen Sie unter `Zielraum` den Raum aus.",
          "Wählen Sie `In Raum setzen`.",
        ],
      },
    ],
    result:
      "Sie sehen die aktuelle Belegung des Raums. Unter `Unterwegs` stehen anwesende Kinder, die noch keinem Raum zugeordnet sind. Die Anzeige ändert sich, sobald Kinder den Raum wechseln.",
    troubleshootingDetails: [
      "Steht unter `Zielraum` `Keine aktiven Räume`? Dann läuft gerade keine Aufsicht, in die Sie Kinder setzen können.",
      "Fehlt unter `Unterwegs` die Auswahl der Kinder? Dann erfasst Ihre OGS die Anwesenheit nicht über die App. Fragen Sie Ihre Leitung.",
      "Fehlt der Reiter `Plan` oder `Raumplan als PDF`? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
    ],
    related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.changeLocation],
  };
}
