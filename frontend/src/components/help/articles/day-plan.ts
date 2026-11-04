import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Tagesplan (#2383) ist in der echten App der oberste Eintrag im
 * Tagesbetrieb und hatte bisher keine Hilfeseite. Inhalt belegt aus
 * `components/timetable/tagesplan-view.tsx`: chronologische Blockliste,
 * Jetzt-Linie, Tippen auf einen laufenden Block oeffnet dessen Kinderliste,
 * ein startbarer Block startet ueber `Starten` (#3622). Bei einfacher Anwesenheit greift
 * der BinaryModeGuard, deshalb steht das Thema in DETAILED_ONLY_TOPIC_IDS.
 */
export function dayPlanTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.dayPlan,
    title: "Den Betreuungstag von heute verfolgen",
    question: "Wo sehe ich den Betreuungstag von heute?",
    summary:
      "Die Seite `Heute` zeigt die Betreuungsblöcke des Tages nach Uhrzeit.",
    group: "tagesplanung",
    audience: "caregiver",
    icon: "ClockAfternoon",
    requirements: [
      "Ihre OGS hat den Betreuungsplan eingeschaltet.",
      "Sie dürfen den Betreuungsplan sehen.",
    ],
    steps: [
      "Öffnen Sie `Heute` in der Seitenleiste.",
      "Sie sehen die Blöcke des Tages von früh nach spät.",
      "Blöcke zur selben Zeit stehen nebeneinander, unter `Gleichzeitig`.",
      "Eine Linie zeigt, wo Sie gerade im Tag stehen.",
      "Tippen Sie auf einen laufenden Block. Das öffnet seine Kinderliste.",
      "Hat Ihr Block noch nicht begonnen? Wählen Sie `Starten`.",
    ],
    result:
      "Nach `Starten` öffnet moto die Kinderliste des Blocks in `Aktuelle Aufsicht`.",
    differences: [
      "`Läuft` heißt: der Block ist gerade aktiv. `Beendet` und `Nicht gestartet` können Sie nur ansehen.",
      "`Fällt aus` zeigt zusätzlich den Grund.",
      "Laufen mehr als zwei Blöcke gleichzeitig? Dann wischen Sie auf dem Handy zur Seite, um alle zu sehen.",
      "Ein `Dienst` ist eine Aufgabe ohne Kinder, etwa die Busaufsicht. Er hat keinen Knopf `Starten`.",
      "Blöcke, für die Sie eingeteilt sind, sind in ihrer Farbe hinterlegt. Das gilt auch für eine Vertretung.",
      "Welche Blöcke Sie sehen, legt Ihre OGS fest. Manche sehen den ganzen Tag der Schule, andere nur die eigene Einteilung.",
      "Ob Sie nur eigene oder alle Blöcke starten dürfen, legt Ihre OGS fest. Wer einen fremden Block startet, wird nicht zur Aufsicht.",
      ...(presenceMode === "unknown"
        ? ["Bei einfacher Anwesenheit gibt es die Seite `Heute` nicht."]
        : []),
    ],
    notes: [
      "Mit den Pfeilen oben rechts sehen Sie einen anderen Tag.",
      "Betreut Ihre OGS am Wochenende? Dann zeigt die Seite `Heute` auch Samstag und Sonntag. Dort gilt der Freitagsplan.",
    ],
    troubleshootingDetails: [
      // Live geprueft: der haeufigste Leerzustand einer Betreuungskraft.
      "Steht dort `Heute ist keine Betreuung geplant`? Dann sind Sie für diesen Tag nicht eingeteilt. Die Einteilung macht Ihre Leitung.",
      "Steht dort `Der Betreuungsplan ist ausgeschaltet`? Dann nutzt Ihre OGS diese Funktion nicht.",
      "Steht dort `Kein Zugriff auf den Betreuungsplan`? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
    ],
    related: [HELP_TOPICS.carePlan, HELP_TOPICS.activeSupervision],
  };
}
