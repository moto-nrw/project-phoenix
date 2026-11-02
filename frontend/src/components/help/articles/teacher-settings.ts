import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/school/einstellungen`: Auswahl der Arten, danach das Geraet einrichten. */
export function teacherSettingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSettings,
    title: "Benachrichtigungen einstellen",
    question: "Wie werde ich über Neues informiert?",
    summary:
      "Erst wählen, worüber Sie informiert werden, dann das Gerät einrichten.",
    group: "einstieg",
    audience: "teacher",
    icon: "Settings",
    steps: [
      "Öffnen Sie `Einstellungen`.",
      "Wählen Sie unter `Benachrichtigungen` die Arten aus.",
      "Erlauben Sie danach die Benachrichtigungen auf Ihrem Gerät.",
    ],
    result: "moto meldet sich, sobald etwas Neues für Sie ansteht.",
    notes: [
      "`Krankmeldungen meiner Klassen`: moto meldet sich, wenn ein Kind aus Ihren Klassen für heute krank oder entschuldigt ist. Name und Grund stehen nicht im Hinweis.",
      "Tippen Sie auf den Hinweis. Dann öffnet sich die Liste der Klasse.",
      "Die Schulverwaltung bekommt diesen Hinweis für alle Klassen der Schule.",
      "`Fehlt ohne Meldung`: nur für die Schulverwaltung. moto meldet sich, wenn eine Lehrkraft ein Kind als fehlend einträgt und keine Meldung da ist. Im Hinweis steht nur die Zahl der Kinder. Diese Benachrichtigung ist von Anfang an eingeschaltet.",
      "`Nachrichten von Kolleginnen und Kollegen`: neue Nachrichten im Team-Chat der OGS.",
      "Benachrichtigungen kommen erst an, wenn moto schule auf dem Startbildschirm liegt.",
    ],
    differences: [
      "Nutzen Sie Samsung Internet? Öffnen Sie moto schule stattdessen in Chrome.",
    ],
    troubleshootingDetails: [
      "Es kommt nichts an? Prüfen Sie, ob Sie moto auf dem Gerät erlaubt haben, Benachrichtigungen zu senden.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherNotices,
      HELP_TOPICS.teacherLogin,
    ],
  };
}
