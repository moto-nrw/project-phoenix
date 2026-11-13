import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function webAttendanceTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const result =
    presenceMode === "detailed"
      ? "Mit `Anwesend (ohne Raum)` steht das Kind nach dem Anmelden unter `Unterwegs`. Mit einem Raum unter `Anmelden in` ist es sofort in diesem Raum. Nach dem Abmelden steht das Kind unter `Zuhause`, ein Raumbesuch endet dabei."
      : presenceMode === "binary"
        ? "Nach dem Anmelden steht das Kind auf `Anwesend`. Nach dem Abmelden steht es auf `Zuhause`."
        : "moto zeigt nach der Aktion den neuen Anwesenheitsstatus des Kindes.";

  return {
    id: HELP_TOPICS.webAttendance,
    title: "Ein Kind an- oder abmelden",
    question: "Wie melde ich ein Kind an oder ab?",
    summary: "Tragen Sie ein, ob ein Kind heute in der OGS ist.",
    group: "kinder",
    audience: "caregiver",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      // #6953: das Ziel der Anmeldung gibt es nur bei der Erfassung mit
      // Räumen. Fällt ein Terminal aus, meldet das Team in der App an.
      ...(presenceMode === "binary"
        ? []
        : [
            {
              title: "Kinder direkt in einen Raum anmelden",
              description:
                "So machen Sie es, wenn ein Terminal ausfällt. Das Kind ist sofort im Raum, wie nach einem Scan am Terminal.",
              steps: [
                "Öffnen Sie `Alle Kinder` und wählen Sie `An- & Abmelden`.",
                "Wählen Sie unter `Anmelden in` den Raum oder die Aktivität.",
                "Auf den Karten steht jetzt `In … anmelden` mit dem Namen des Raums.",
                "Tippen Sie die Kinder an. Jedes Kind ist sofort im Raum.",
                "Die Wahl bleibt, bis Sie sie ändern. Sie gilt auch nach dem Neuladen auf diesem Gerät.",
              ],
            },
          ]),
      {
        title: "Über die Angaben des Kindes",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Anmelden` oder `Abmelden`.",
          "Prüfen Sie den Namen im Fenster.",
          "Bestätigen Sie mit `Anmelden`. Beim Abmelden wählen Sie `Geht nach Hause` oder `Geht zum Unterricht`.",
        ],
      },
      {
        title: "Direkt in der Kinderliste",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Wählen Sie `An- & Abmelden`.",
          "Wählen Sie die Karte des Kindes.",
          "Prüfen Sie den neuen Status auf der Karte.",
          "Wählen Sie danach `Fertig`.",
        ],
      },
      {
        // #4762: „Alle N auswählen“ nimmt nur die sichtbaren Karten und
        // lässt Kinder aus, die heute krank, entschuldigt oder nicht
        // erwartet sind.
        title: "Eine ganze Klasse anmelden",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Wählen Sie im Filter `Klasse` die Klasse.",
          "Wählen Sie `An- & Abmelden` und danach `Mehrere`.",
          "Tippen Sie auf den Knopf mit `Alle` und der Zahl der Kinder.",
          "Die Zahl nennt die ausgewählten Kinder.",
          "Tippen Sie Kinder an, die nicht da sind. So nehmen Sie sie wieder heraus.",
          "Wählen Sie `Anmelden`.",
          "Lesen Sie den Hinweis `Noch nicht angemeldet`. Er nennt die Kinder, die noch fehlen.",
        ],
      },
    ],
    result,
    notes: [
      "Möchten Sie mehrere Kinder ändern? Wählen Sie `Mehrere`.",
      ...(presenceMode === "binary"
        ? []
        : [
            "Unter `Anmelden in` stehen Räume mit laufender Aufsicht, offene Räume und laufende Aktivitäten. Einen Raum ohne Aufsicht können Sie nicht wählen. Die App startet keine Aufsicht.",
            "Mit `Mehrere` und einem Raum kommen nur abwesende Kinder in den Raum. Kinder, die schon da sind, bleiben, wo sie sind.",
          ]),
      "Der Knopf mit `Alle` lässt kranke und entschuldigte Kinder aus. Das gilt auch für Kinder auf Klassenfahrt oder mit „Kommt heute nicht“. Die Leiste nennt ihre Zahl.",
      "Im Hinweis `Noch nicht angemeldet` zeigt `Nur „Schule“ zeigen` die Kinder, die laut Tagesplan noch in der Schule sind und heute noch nicht da waren.",
      "In der Listenansicht kreuzen Sie die Kinder links an und wählen oben `Anmelden` oder `Abmelden`.",
      // Live geprueft: in `Direkt` schreibt jeder Tipp sofort, ohne Rueckfrage.
      // Ausnahme seit #3324: frühes Gehen öffnet vorher ein Fenster.
      "In `Direkt` meldet jeder Tipp sofort an oder ab. Auf der Karte steht `Tippen zum Anmelden`.",
      "Geht ein Kind zum Unterricht und kommt danach wieder? Öffnen Sie die Kindakte und wählen Sie `Abmelden`. Wählen Sie dann `Geht zum Unterricht`. Das Kind steht danach auf `Im Unterricht`, nicht auf `Zuhause`.",
      "Ein Tipp in `Direkt` und die Abmeldung über `Mehrere` zählen sonst als `Geht nach Hause`. Nur wenn ein Kind in einem Raum ist, fragt moto auch dort nach dem Ziel.",
      "Kommt das Kind zurück, melden Sie es ganz normal an. Kommt es heute nicht zurück, steht es am Ende des Betreuungstags wieder auf `Zuhause`.",
      "Geht ein Kind früher als seine Abholzeit, fragt moto nach dem Grund. Der Grund ist freiwillig. Das Fenster bietet auch `Geht zum Unterricht` an.",
      "Den Grund sehen Sie danach beim Kind unter `Heutige Abholung`. Mit Anwesenheitsprotokoll steht er auch in der `Tagesauswertung` und im Verlauf.",
    ],
    differences: [
      "Fehlt `Anmelden in`? Dann erfasst Ihre OGS nur „Anwesend oder abwesend“ ohne Räume, oder Ihnen fehlt das Recht, Kinder zu verschieben.",
      "moto zeigt nur die Aktion, die zum aktuellen Status passt.",
      "Fehlt `Anmelden` oder `Abmelden`? Fragen Sie Ihre Leitung nach dem Recht zum An- und Abmelden.",
      "moto fragt nie nach einem Grund? Dann hat Ihre Schule die Frage ausgeschaltet oder das Kind hat heute keine Abholzeit.",
    ],
    troubleshootingDetails: [
      "Steht unter `Anmelden in` plötzlich `Anwesend (ohne Raum)` und ein Hinweis? Dann läuft im gewählten Raum keine Aufsicht mehr. Wählen Sie ein neues Ziel.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.changeLocation,
      HELP_TOPICS.absences,
    ],
  };
}
