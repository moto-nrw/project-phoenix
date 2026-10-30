import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/parents/children/<id>/elternsprechtag`: Termin buchen (#4801). */
export function parentConferenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentConference,
    title: "Einen Termin am Elternsprechtag buchen",
    question: "Wie buche ich einen Termin bei der Lehrkraft?",
    summary:
      "Bietet die Lehrkraft einen Elternsprechtag an, buchen Sie eine freie Zeit für Ihr Kind.",
    group: "nachrichten",
    audience: "parent",
    icon: "CalendarPlus",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Haben Sie mehrere Kinder, wählen Sie oben das Kind.",
      "Wählen Sie bei `Elternsprechtag` die Schaltfläche `Termin buchen`.",
      "Tippen Sie unter `Freie Zeiten` auf eine Uhrzeit.",
    ],
    result:
      "Der Termin ist gebucht. Er steht in Ihrem Kalender, und Sie bekommen eine Nachricht.",
    notes: [
      "Sie buchen einen Termin je Kind. Haben Sie zwei Kinder in der Klasse, buchen Sie für jedes Kind einzeln.",
      "Tippen Sie auf eine andere Zeit, um Ihren Termin zu verschieben. Mit `Termin absagen` sagen Sie ab.",
    ],
    differences: [
      "Sie sehen nur freie Zeiten. Was andere Familien gebucht haben, sehen Sie nicht.",
      "Ändert die Lehrkraft den Tag oder sagt ihn ab, bekommen Sie eine Nachricht.",
    ],
    troubleshootingDetails: [
      "Sie sehen keinen `Elternsprechtag`? Dann bietet die Lehrkraft der Klasse zurzeit keinen an.",
      "Sie können nicht mehr buchen oder absagen? Bis 24 Stunden vor dem Termin ist das in der App möglich. Danach wenden Sie sich direkt an die Lehrkraft.",
      "Die Buchung wird abgelehnt? Vielleicht dürfen Sie für dieses Kind keine Termine buchen. Fragen Sie die Schule.",
    ],
    related: [HELP_TOPICS.parentCalendar, HELP_TOPICS.parentChildOverview],
  };
}
