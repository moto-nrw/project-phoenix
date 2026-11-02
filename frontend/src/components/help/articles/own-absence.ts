import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function ownAbsenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.ownAbsence,
    title: "Eine eigene Abwesenheit eintragen",
    question: "Wie trage ich eine eigene Abwesenheit ein?",
    summary:
      "Melden Sie Krankheit, Fortbildung oder eine andere Abwesenheit für den passenden Zeitraum.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "BellRing",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      "Wählen Sie in der `Stempeluhr` oben `Abwesend`.",
      "Wählen Sie den runden Knopf `Abwesenheit melden`.",
      "Wählen Sie unter `Art der Abwesenheit` den passenden Eintrag.",
      "Tragen Sie `Von` und `Bis` ein.",
      "Schalten Sie bei Bedarf `Halber Tag` ein.",
      "Tragen Sie bei Bedarf eine `Bemerkung` ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die Abwesenheit ist eingetragen und erscheint in Ihrer Zeiterfassung.",
    differences: [
      "Ihre OGS kann zusätzliche Abwesenheitsarten anbieten.",
      "Urlaub beantragen Sie in der Karte `Urlaub` und nicht über diese Aktion.",
      "Freizeitausgleich trägt nur ein, wer die Zeiten des Teams verwalten darf.",
    ],
    notes: [
      "Ihre Abwesenheit steht danach in der Tabelle `Zeiterfassung` unter `Status`. Über das Stift-Symbol am Ende der Zeile öffnen Sie den Tag.",
      "Hat eine Art Ihrer OGS feste Tage, sehen Sie den Stand in der Karte `Kontingente`. Den Anspruch trägt Ihre Leitung ein.",
    ],
    related: [HELP_TOPICS.vacation, HELP_TOPICS.trackWorkTime],
  };
}
