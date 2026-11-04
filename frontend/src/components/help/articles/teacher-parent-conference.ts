import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/elternsprechtag`: Elternsprechtag einer eigenen Klasse (#4801). */
export function teacherParentConferenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherParentConference,
    title: "Einen Elternsprechtag anbieten",
    question: "Wie biete ich den Eltern meiner Klasse Gesprächstermine an?",
    summary:
      "Sie legen einen Tag mit Zeitfenstern an. Die Eltern buchen in der Eltern-App ein Zeitfenster je Kind.",
    group: "nachrichten",
    audience: "teacher",
    icon: "CalendarCheck",
    requirements: ["Ihnen ist die Klasse zugeordnet."],
    steps: [
      "Öffnen Sie `Kalender`.",
      "Wählen Sie `Elternsprechtag`.",
      "Wählen Sie `Neuer Elternsprechtag`.",
      "Wählen Sie die Klasse und den Tag.",
      "Geben Sie `Beginn`, `Ende` und die `Dauer je Gespräch` an. Eine Pause und den Raum können Sie dazuschreiben.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die Eltern der Klasse sehen die freien Zeitfenster in der Eltern-App und buchen eines je Kind. Jede Buchung steht in Ihrem Kalender und im Kalender der Eltern.",
    notes: [
      "Wählen Sie den Tag in der Liste. Dort sehen Sie, welches Kind wann kommt. Mit `Drucken` drucken Sie die Liste.",
      "Eltern buchen, verschieben oder sagen ab, bis 24 Stunden vor dem Termin. Danach melden sie sich bei Ihnen.",
      "Unter `Einstellungen` schalten Sie `Buchungen am Elternsprechtag` ein. Dann bekommen Sie einen Hinweis, wenn Eltern buchen, verschieben oder absagen.",
    ],
    differences: [
      "Sie ändern den Tag mit `Ändern`. Eltern mit Buchung bekommen eine Nachricht. Fällt ein gebuchtes Zeitfenster weg, ist diese Buchung abgesagt.",
      "Wählen Sie eine andere Klasse, werden alle bisherigen Buchungen abgesagt. Die Eltern bekommen eine Nachricht.",
      "Sie löschen den Tag mit `Löschen`. Alle Buchungen sind dann abgesagt, und die Eltern bekommen eine Nachricht.",
      "`Neuer Elternsprechtag` ist grau? Ihnen ist keine Klasse zugeordnet.",
    ],
    troubleshootingDetails: [
      "Eltern finden keinen Termin? Die Eltern sehen den Tag nur, wenn ihr Kind in der Klasse ist und sie Zugang zur Eltern-App haben.",
      "Am Ende des Schuljahres löscht moto die Elternsprechtage und ihre Buchungen.",
    ],
    related: [
      HELP_TOPICS.teacherCalendar,
      HELP_TOPICS.teacherParentLetters,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}
