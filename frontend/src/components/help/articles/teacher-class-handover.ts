import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function teacherClassHandoverTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassHandover,
    title: "Klasse an die OGS übergeben",
    question: "Wie übergebe ich meine Klasse nach dem Unterricht?",
    summary:
      "Sie bestätigen die Übergabe und markieren Kinder, die nicht da sind.",
    group: "klasse",
    audience: "teacher",
    icon: "ArrowsLeftRight",
    steps: [
      "Öffnen Sie in der `Klassenansicht` Ihre Klasse für heute.",
      "Tippen Sie auf `Klasse übergeben`.",
      "Tippen Sie bei jedem Kind, das nicht da ist, auf `Nicht da`.",
      "Tippen Sie auf `Übergeben`.",
    ],
    result:
      "Oben steht, wann Sie übergeben haben. Die OGS sieht sofort bei jedem Kind, ob es da ist.",
    notes: [
      "In der Liste stehen nur die Kinder, die heute in die OGS gehen.",
      "Kinder, die am Morgen als `Fehlt` eingetragen sind, stehen schon auf `Nicht da`.",
      "Die Anwesenheit am Morgen ändert sich dadurch nicht.",
      "Mit `Übergabe ändern` korrigieren Sie einzelne Kinder. Die OGS sieht dann den neuen Stand.",
      "Mit `Übergabe zurücknehmen` nehmen Sie die Übergabe heute wieder zurück.",
    ],
    differences: [
      "Fehlt `Klasse übergeben`? Übergeben geht nur heute und an Schultagen. Oder Ihre OGS hat es nicht freigegeben.",
    ],
    troubleshootingDetails: [
      "Übergeben klappt nicht und ein Kind gehört nicht mehr zur Klasse? Laden Sie die Seite neu.",
    ],
    related: [
      HELP_TOPICS.teacherMorningAttendance,
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}
