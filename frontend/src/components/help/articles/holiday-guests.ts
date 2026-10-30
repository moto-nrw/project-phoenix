import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Feriengäste im Alltag (#4666): Kinder anderer Schulen in der Feriengruppe
 * der ausrichtenden OGS. Die Kindakte zeigt nur Betreuung, Notfall und
 * Abholung und die Antworten, die der Träger freigibt (#5202).
 */
export function holidayGuestsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.holidayGuests,
    title: "Feriengäste betreuen",
    question: "Wo sehe ich Notfall und Abholung eines Feriengasts?",
    summary:
      "In den Ferien kommen Kinder anderer Schulen in Ihre OGS. Sie stehen in der Feriengruppe.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "SunHorizon",
    image: "/help/screens/feriengast-angaben-aus-der-anmeldung.webp",
    imageAlt:
      "Notfall, Abholung und Angaben aus der Ferienanmeldung bei einem Feriengast.",
    requirements: [
      "Ihre OGS richtet eine Ferienwoche für den Träger aus.",
      "Die Leitung hat unter `Ferienwochen` eine Feriengruppe gewählt.",
      "Die Ferienwoche hat begonnen. Vorher stehen Feriengäste noch nicht in der Gruppe.",
    ],
    steps: [
      "Öffnen Sie die Feriengruppe. Unter dem Namen jedes Feriengasts steht `Feriengast`.",
      "Melden Sie Feriengäste an und ab wie jedes andere Kind.",
      "Öffnen Sie die Karte eines Feriengasts.",
      "Lesen Sie unter `Betreuung` die Woche und die gebuchte Zeit.",
      "Unter `Notfall` stehen die Eltern und weitere Kontakte. Unter `Abholung` steht, wer das Kind außer den Eltern abholen darf.",
      "Unter `Angaben aus der Ferienanmeldung` steht zum Beispiel, ob das Kind allein nach Hause gehen darf.",
    ],
    result:
      "Sie wissen, wann das Kind gehen darf und wen Sie im Notfall anrufen.",
    notes: [
      "Mehr als Betreuung, Notfall, Abholung und die freigegebenen Angaben sehen Sie von einem Feriengast nicht. Gesundheitsangaben und die übrigen Angaben bleiben beim Träger.",
      "Jeder Blick auf Notfall und Abholung steht im Zugriffsprotokoll Ihrer OGS.",
    ],
    differences: [
      "Nach dem letzten Tag der Woche steht der Feriengast nicht mehr in der Gruppe und nicht mehr im Check-in. Seine Angaben bleiben noch 7 Tage zum Nachlesen.",
    ],
    troubleshootingDetails: [
      "Fehlt `Angaben aus der Ferienanmeldung`? Dann gibt der Träger keine Frage frei.",
      "Fehlt ein Feriengast? Anmeldungen bestätigt der Träger. Neu bestätigte Kinder kommen innerhalb einer Stunde in die Gruppe.",
      "Sehen Sie `Notfall` und `Abholung` nicht? Dafür brauchen Sie das Recht, die Angaben der Kinder zu lesen. Fragen Sie Ihre Leitung.",
    ],
    related: [HELP_TOPICS.webAttendance, HELP_TOPICS.ownGroups],
  };
}
