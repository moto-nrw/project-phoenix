import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function carePlanTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.carePlan,
    title: "Den Betreuungsplan ansehen",
    question: "Wie sehe ich den Betreuungsplan an?",
    summary:
      "Prüfen Sie, welche Betreuung für die Schule geplant ist und wo Sie selbst eingesetzt sind.",
    group: "tagesplanung",
    audience: ["caregiver", "lead"],
    icon: "Table",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mein Kalender`.",
      "Wählen Sie den Tab `Betreuungsplan`.",
      "Wählen Sie oben `Tag` oder `Woche`.",
      "Nutzen Sie die Pfeile oder `Heute`, um zum passenden Tag zu wechseln.",
      presenceMode === "binary"
        ? "Gruppe und Fachkräfte stehen im Block, wenn sie zugeordnet sind."
        : "Raum, Gruppe und Fachkräfte stehen im Block, wenn sie zugeordnet sind.",
      "Ein durchgestrichener Name heißt: abwesend. `(Ersatz)` steht hinter einer Ersatzkraft.",
      "Wählen Sie einen Block, um alle Kinder zu sehen.",
    ],
    result:
      "Sie sehen, was die OGS geplant hat und wo Sie eingesetzt sind. Als Betreuungskraft können Sie den Plan nicht verändern.",
    differences: [
      // Live geprueft: die Kopfzeile des Reiters traegt das Kennzeichen.
      "Steht oben rechts `Nur ansehen`? Dann dürfen Sie den Plan lesen, aber nicht ändern.",
    ],
    notes: [
      // #4766: Filter Gruppe und Person, auch in der Leseansicht.
      "Nur eine Gruppe oder eine Person sehen? Wählen Sie oben bei `Alle Gruppen` eine Gruppe oder bei `Alle Personen` eine Person. Die Person sieht auch ihre Vertretungen.",
      // #4768: Raumfilter.
      "Nur einen Raum sehen? Wählen Sie oben bei `Alle Räume` einen Raum.",
      "Die Adresse der Seite behält Ihre Auswahl. So können Sie den gefilterten Plan teilen.",
      // #5028: Sa/So nur in der Woche des heutigen Wochenendes.
      "Betreut Ihre OGS am Wochenende? Dann gilt dort der Freitagsplan. Am Wochenende zeigt die Wochenansicht dieser Woche auch Samstag und Sonntag.",
    ],
    troubleshootingDetails: [
      "Der Tab `Betreuungsplan` fehlt? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
      "Steht dort `Noch kein Planungszeitraum`? Dann hat Ihre Leitung den Zeitraum noch nicht angelegt.",
      "Steht dort `Für diese Auswahl ist nichts geplant`? Stellen Sie die Filter wieder auf `Alle Gruppen`, `Alle Personen` und `Alle Räume`.",
    ],
    related: [HELP_TOPICS.mySchedule, HELP_TOPICS.activeSupervision],
  };
}
