import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/stichtagszahlen` (#4673): die festgehaltenen Zahlen, nach denen moto jede Schule berechnet. */
export function traegerKeyDateCountsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerKeyDateCounts,
    title: "Stichtagszahlen der Schulen",
    question: "Nach welchen Zahlen berechnet moto meine Schulen?",
    summary:
      "Die Stichtagszahlen zeigen je Monat die aktiven Kinder und Terminals jeder Schule am Stichtag.",
    group: "einstieg",
    audience: "traeger",
    icon: "CalendarCheck",
    steps: [
      "Wählen Sie in der Navigation `Stichtagszahlen`.",
      "Wählen Sie oben einen Monat. Zu Beginn sehen Sie den neuesten Monat.",
      "Lesen Sie oben die Summe für alle Schulen.",
      "Lesen Sie darunter die Zahlen jeder Schule.",
      "Wählen Sie unter `Alle Monate` einen Monat, um seine Zahlen zu sehen.",
    ],
    result:
      "Sie sehen für jede Schule, wie viele aktive Kinder und aktive Terminals moto am Stichtag gezählt hat.",
    notes: [
      "Suchen Sie oben nach einer Schule oder einem Ort.",
      "moto hält die Zahlen am Stichtag einmal fest. Danach ändern sie sich nicht mehr.",
      "Das moto-Team sieht in seinem Bericht dieselben Zahlen.",
      "Neben jeder Zahl steht, wie sie sich zum Monat davor verändert hat.",
      "Sie sehen nur Zahlen, keine Namen.",
    ],
    differences: [
      "Bei unterschiedlichen Stichtagen stehen unter `Alle Monate` mehrere Daten. Das genaue Datum finden Sie bei jeder Schule unter `Stichtag`.",
      "Steht bei einer Schule `Keine Zahlen zu diesem Stichtag`, gab es die Schule am Stichtag noch nicht in moto.",
      "Steht bei einer Schule `Nachgeholt`, hat moto die Zahlen erst nach dem Stichtag festgehalten. Sie gelten trotzdem für den Stichtag.",
    ],
    related: [HELP_TOPICS.traegerDashboard, HELP_TOPICS.traegerSchoolView],
  };
}
