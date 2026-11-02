import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Eigene Einwilligung zur Nutzungsanalyse (#4901): der Dialog nach dem Login
 * und der Schalter zum Ändern. Eine Frage für alle Rollen, je Portal ein Weg.
 */
export function analyticsConsentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.analyticsConsent,
    title: "Nutzungsanalyse erlauben oder ablehnen",
    question: "Darf moto zählen, wie ich die App nutze?",
    summary:
      "Das entscheiden nur Sie. Wenn Sie ablehnen, können Sie moto genauso nutzen.",
    group: "einstieg",
    // Das Träger-Portal (#4649) erfasst nichts und fragt deshalb nicht.
    audience: ["caregiver", "lead", "parent", "teacher"],
    icon: "ChartLine",
    steps: [],
    instructionGroups: [
      {
        title: "Nach der Anmeldung",
        description:
          "moto fragt einmal, ob es zählen darf, welche Seiten und Schaltflächen Sie nutzen. Namen und Eingaben sieht moto dabei nicht.",
        steps: ["Wählen Sie `Erlauben` oder `Ablehnen`."],
      },
      {
        title: "Später ändern im OGS-Portal",
        steps: [
          "Öffnen Sie oben rechts Ihr `Profil`.",
          "Gehen Sie zu `Nutzungsanalyse`.",
          "Schalten Sie `Nutzungsanalyse erlauben` ein oder aus.",
        ],
      },
      {
        title: "Später ändern in der Eltern-App",
        steps: [
          "Tippen Sie unten auf `Mehr` und dann auf `Einstellungen`.",
          "Schalten Sie bei `Nutzungsanalyse` den Schalter ein oder aus.",
        ],
      },
      {
        title: "Später ändern in moto schule",
        steps: [
          "Öffnen Sie `Einstellungen`.",
          "Schalten Sie bei `Nutzungsanalyse` den Schalter ein oder aus.",
        ],
      },
    ],
    result:
      "Ihre Wahl gilt für Ihr Konto, in jeder Schule. Schalten Sie aus, endet die Zählung sofort.",
    notes: [
      "Ihre Schule kann die Nutzungsanalyse für alle ausschalten. Dann fragt moto Sie nicht. Ihre Wahl gilt wieder, sobald die Schule einschaltet.",
      "Im OGS-Portal gilt Ihre Wahl auch für Aufzeichnungen, wenn Ihre Schule sie erlaubt. Texte, Eingaben und Bilder sind darin unkenntlich.",
      "Sie haben die Frage über das Kreuz oben rechts geschlossen? Dann zählt moto nichts. moto fragt beim nächsten Öffnen noch einmal.",
    ],
    differences: [
      "Am Computer steht `Einstellungen` in der Eltern-App in der Navigation statt hinter `Mehr`.",
    ],
    related: [],
  };
}
