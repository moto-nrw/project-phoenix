import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/login`. Eigenes Portal; ein Konto ohne Rolle wird abgewiesen. */
export function teacherLoginTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherLogin,
    title: "Bei moto schule anmelden",
    question: "Wie melde ich mich bei moto schule an?",
    summary: "moto schule hat eine eigene Adresse und eine eigene Anmeldung.",
    group: "einstieg",
    audience: "teacher",
    icon: "LogIn",
    steps: [
      "Öffnen Sie die Adresse von moto schule.",
      "Tragen Sie Ihre `E-Mail-Adresse` ein.",
      "Tragen Sie Ihr `Passwort` ein.",
      "Wählen Sie `Anmelden`.",
      "Fragt moto nach einem Code? Öffnen Sie die E-Mail von moto.",
      "Geben Sie den sechsstelligen Code ein.",
    ],
    result: "moto öffnet die `Klassenansicht` mit dem heutigen Tag.",
    notes: [
      "Legen Sie sich die Adresse als Lesezeichen an.",
      "Passwort vergessen? Wählen Sie `Passwort vergessen?` auf der Anmeldeseite.",
    ],
    differences: [
      "Oben steht `Willkommen im Schulportal`. So erkennen Sie, dass Sie richtig sind.",
      "Für Verwaltung ist ein zweiter Nachweis immer Pflicht. Ebenso für Personalstammdaten, Personaldokumente und Personalfinanzen. Das gilt auch für Einkommensnachweise der Eltern. Ebenso für Gesundheitsdokumente und das Verwalten fremder Arbeitszeiten. Ebenso, wenn Sie Urlaub genehmigen dürfen. Gesundheitshinweise der Kinder allein lösen diese Pflicht nicht aus.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Konto habe keinen Zugang zum Schul-Portal? Dann fehlt Ihnen die Rolle. Wenden Sie sich an die OGS-Verwaltung.",
      "moto sagt, Ihre Schule nutze moto schule zurzeit nicht? Dann bietet Ihre Schule moto schule nicht an. Wenden Sie sich an die OGS-Verwaltung.",
      "moto sagt `Die E-Mail-Adresse oder das Passwort stimmt nicht.`? Prüfen Sie beides und versuchen Sie es erneut.",
    ],
    related: [
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherLoginProblem,
    ],
  };
}
