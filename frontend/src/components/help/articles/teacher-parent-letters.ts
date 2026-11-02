import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function teacherParentLettersTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherParentLetters,
    title: "Einen Elternbrief schreiben",
    question: "Wie schreibe ich den Eltern meiner Klasse einen Brief?",
    summary:
      "Sie schreiben den Eltern Ihrer Klasse. Die Eltern lesen den Brief in moto und bekommen eine E-Mail.",
    group: "nachrichten",
    audience: "teacher",
    icon: "EnvelopeOpen",
    requirements: [
      "Ihnen ist eine Klasse zugeordnet. Sekretariat und Schulleitung schreiben an jede Klasse.",
    ],
    steps: [
      "Öffnen Sie `Elternbriefe`.",
      "Wählen Sie `Neuer Elternbrief`.",
      "Wählen Sie die Klasse.",
      "Geben Sie `Betreff` und `Text` ein.",
      "Wählen Sie unter `Was sollen die Eltern tun?`: `Nur zur Information`, `Mit Lesebestätigung` oder `Einverständnis mit Unterschrift`.",
      "Wählen Sie `Senden`.",
    ],
    result:
      "Die Eltern der Klasse sehen den Brief sofort. Als Absender sehen sie Ihren Namen, die Klasse und die Schule.",
    notes: [
      "Wählen Sie den Brief in der Liste. Dort sehen Sie, wer gelesen, bestätigt oder zugestimmt hat.",
      "Ein gesendeter Brief lässt sich nicht mehr ändern. Sie können ihn über `Zurückziehen` zurücknehmen.",
      "Die OGS sieht Ihre Briefe in ihrer Übersicht. Ändern kann sie nur Sie.",
    ],
    differences: [
      "Sie sehen nur Ihre eigenen Briefe. Briefe anderer Lehrkräfte stehen nicht in Ihrer Liste.",
      "`Neuer Elternbrief` ist grau? Warten Sie, bis die Klassen geladen sind. Ohne zugeordnete Klasse können Sie keinen Brief schreiben.",
    ],
    troubleshootingDetails: [
      "Die Klassen laden nicht? Wählen Sie `Wiederholen` bei der Fehlermeldung.",
      "Das Senden ist fehlgeschlagen? Wählen Sie `Senden` oder `Wiederholen`. Dabei wird derselbe gespeicherte Brief verwendet.",
      "Der Brief geht nicht raus? Vielleicht hat die OGS Elternbriefe ausgeschaltet. Fragen Sie die OGS-Leitung.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherCalendar,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}
