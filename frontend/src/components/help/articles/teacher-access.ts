import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** Einladung annehmen: geteilte Strecke, danach geht es nach moto schule. */
export function teacherAccessTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherAccess,
    title: "Zugang zu moto schule einrichten",
    question: "Wie bekomme ich Zugang zu moto schule?",
    summary: "Sie öffnen den Link aus der E-Mail und legen Ihr Passwort fest.",
    group: "einstieg",
    audience: "teacher",
    icon: "KeyRound",
    requirements: ["Sie haben eine Einladungs-E-Mail der OGS bekommen."],
    steps: [
      "Öffnen Sie den Link aus der E-Mail.",
      "Legen Sie Ihr Passwort fest.",
      "Nehmen Sie die Einladung an.",
      "Öffnen Sie danach moto schule und melden Sie sich an.",
    ],
    result: "Sie landen in der `Klassenansicht` mit Ihren Klassen.",
    notes: [
      "Haben Sie schon ein Konto? Dann melden Sie sich an und kehren zur Einladung zurück. Ihr Passwort bleibt unverändert.",
      "Der Link aus der E-Mail führt schon zur richtigen Adresse.",
      "Zur Einladung bekommen Sie eine zweite E-Mail: `Willkommen bei moto`. Sie führt zu dieser Anleitung. Den Link zum Annehmen finden Sie nur in der Einladungs-Mail.",
    ],
    differences: [
      "moto schule hat eine eigene Adresse. Über die Adresse der OGS kommen Sie nicht hinein.",
    ],
    troubleshootingDetails: [
      "Der Link funktioniert nicht mehr? Bitten Sie die OGS um eine neue Einladung.",
    ],
    related: [
      HELP_TOPICS.teacherLogin,
      HELP_TOPICS.teacherLoginProblem,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}
