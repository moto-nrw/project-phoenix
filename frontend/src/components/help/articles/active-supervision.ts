import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function activeSupervisionTopic(
  presenceMode: HelpPresenceMode,
): HelpTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.activeSupervision,
      title: "Eine Aufsicht starten, führen und beenden",
      question: "Wie arbeite ich mit einer laufenden Aufsicht?",
      summary:
        "Bei einfacher Anwesenheit führt moto keine Aufsichten für einzelne Räume oder Aktivitäten.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Eye",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Nutzen Sie `An- & Abmelden`, um die Anwesenheit zu erfassen.",
      ],
      result:
        "Der Bereich `Aktuelle Aufsicht` wird nicht angezeigt. moto speichert nur, ob ein Kind anwesend ist.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.rooms],
    };
  }
  if (presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.activeSupervision,
      title: "Eine Aufsicht starten, führen und beenden",
      question: "Wie arbeite ich mit einer laufenden Aufsicht?",
      summary:
        "Der Bereich ist nur sichtbar, wenn Ihre OGS Räume und Aktivitäten erfasst.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Eye",
      steps: [
        "Prüfen Sie, ob `Aktuelle Aufsicht` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie dort einen Raum oder geplanten Termin.",
        "Fehlt der Bereich? Erfassen Sie die Anwesenheit unter `Alle Kinder`.",
      ],
      result: "Diese Aufsicht gibt es nur bei detaillierter Anwesenheit.",
      related: [HELP_TOPICS.rooms, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.activeSupervision,
    title: "Eine Aufsicht starten, führen und beenden",
    question: "Wie arbeite ich mit einer laufenden Aufsicht?",
    summary:
      "Übernehmen Sie einen Raum oder einen geplanten Termin und halten Sie die Kinderliste aktuell.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "Eye",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aufsicht starten",
        steps: [
          "Öffnen Sie `Aktuelle Aufsicht`.",
          "Wählen Sie unter `Als Nächstes` einen geplanten Termin.",
          "Starten Sie ihn, sobald die Schaltfläche freigegeben ist.",
          "Öffnen Sie unter `Räume` einen offenen Raum, zum Beispiel den Schulhof.",
          "Wählen Sie `Zur Aufsicht` und dann `Beaufsichtigen`.",
          // Live geprueft: der gruene Streifen steht immer oben auf der
          // Seite, auch ohne geplanten Block und ohne NFC.
          "Oder wählen Sie `Spontane Aktivität starten`, wenn nichts geplant ist.",
          "Prüfen Sie Raum, Aktivität und Betreuungsteam.",
          "Nach dem Start einer spontanen Aktivität öffnet sich `Anwesende Kinder hinzufügen`. Wählen Sie dort die Kinder für die Aktivität aus.",
        ],
      },
      {
        // #3823, ADR 0091: externe Kräfte mit einer von drei Zugangsstufen.
        title: "Weitere Betreuer eintragen",
        steps: [
          "Wählen Sie in der laufenden Aufsicht `Betreuer hinzufügen`.",
          "Wählen Sie die Person unter `Betreuer auswählen` und dann `Hinzufügen`.",
          "Fehlt die Person, zum Beispiel jemand von der Musikschule? Wählen Sie `Externe Person eintragen`.",
          "Tragen Sie `Vorname` und `Nachname` ein.",
          "Wählen Sie den `Zugang`: `Kein Zugang`, `Nur ansehen` oder `Eigene Termine betreuen`.",
          "Bei `Nur ansehen` und `Eigene Termine betreuen` tragen Sie die `E-Mail-Adresse` ein. Die Person bekommt dann eine Einladung.",
          "Wählen Sie `Eintragen`. Sie können die Person sofort auswählen.",
          "Beim Start einer spontanen Aktivität geht das genauso unter `Weitere Betreuer`.",
          "Wurde eine AG-Leitung hinzugefügt? Sie öffnet die Kinderliste unter `Aktuelle Aufsicht`.",
        ],
      },
      {
        // #4763: ändert nur die laufende Aufsicht, nie die Planung.
        title: "Die Aufsicht übergeben oder abgeben",
        steps: [
          "Wählen Sie in Ihrer laufenden Aufsicht `Aufsicht übergeben`.",
          "Übernimmt eine andere Person? Wählen Sie `An Person übergeben`, dann die Person unter `An wen übergeben Sie?` und `Übergeben`.",
          "Bleibt schon jemand anderes als Aufsicht im Raum? Wählen Sie `Aufsicht abgeben` und dann `Abgeben`.",
          "Die Kinder bleiben im Raum und die Aktivität läuft weiter. Die Planung ändert sich nicht.",
        ],
      },
      {
        title: "Die Kinderliste führen",
        steps: [
          "Suchen Sie ein Kind in der laufenden Aufsicht.",
          "Wählen Sie `Hinzufügen`, wenn das Kind anwesend ist und zur Aufsicht kommen soll.",
          "Mehrere Kinder auf einmal? Wählen Sie `Anwesende Kinder`. Dort stehen die Kinder, die gerade in der OGS sind und laut Gehzeit noch bleiben. Haken Sie die Kinder an und wählen Sie `Kinder hinzufügen`.",
          // #4571: live geprüft im Dialog `Anwesende Kinder hinzufügen`.
          "Sie kennen die Kinder an ihrem Betreuungsangebot, zum Beispiel „bis 16 Uhr“? Wählen Sie `Nach Angebot` und darunter das Angebot. Dann stehen dort nur Kinder, die dieses Angebot heute gebucht haben. Kinder, die heute früher gehen oder abgemeldet sind, fehlen. moto merkt sich das Angebot für das nächste Mal.",
          "Prüfen Sie geplante Abholzeiten und Hinweise in der Liste.",
          "Wechselt ein Kind den Raum oder geht nach Hause? Ändern Sie seinen Aufenthaltsort.",
        ],
      },
      {
        title: "Die Aufsicht beenden",
        steps: [
          "Wählen Sie `Beenden`.",
          "Prüfen Sie im Fenster, ob noch Kinder anwesend sind.",
          "Bestätigen Sie das Ende der Aufsicht.",
        ],
      },
    ],
    result:
      "Die Aufsicht ist abgeschlossen. Ein selbst beendeter Termin kann für kurze Zeit über `Rückgängig` wieder geöffnet werden.",
    differences: [
      "Bei einem laufenden Termin können weitere Betreuungskräfte der Aufsicht beitreten.",
      "Ein geplanter Termin hat feste Zeiten. Vorher sind Start oder Ende möglicherweise gesperrt.",
      "Ihre OGS kann erlauben, einen Termin einige Minuten vor dem Ende zu beenden.",
      "Steht am Termin `Nur für Eingeplante`? Dann dürfen hier nur eingeplante Kräfte starten. Ihre OGS kann das Starten für das ganze Team freigeben.",
      "Sie starten einen Termin, für den Sie nicht eingeplant sind? Dann werden Sie nicht zur Aufsicht. Wer beenden darf, legt Ihre OGS fest.",
      "Fehlt `Beenden`? Dann dürfen hier nur eingeplante Kräfte beenden. Ihre OGS kann das Beenden für das ganze Team freigeben.",
      // #4372: offene Räume ohne eigene Aufsicht stehen unter `Räume`.
      "Läuft Ihre Aufsicht in einem offenen Raum? Dann steht der Raum unter `Aktuelle Aufsicht`. Alle anderen offenen Räume finden Sie unter `Räume`.",
      "Haben Sie mehrere Aufsichten? Auf dem Handy stehen sie oben auf der Seite `Aufsicht` als Reiter.",
      // #5035: mit dem Überblick über alle Aufsichten stehen auch fremde da.
      "Sehen Sie auch Aufsichten von anderen? Ihre eigene steht zuerst und öffnet sich zuerst. Bei einer fremden steht oben `Nicht Ihre Aufsicht`.",
      "Welche Räume offen sind, legt Ihre OGS fest.",
      "`Abgeben` ist gesperrt? Dann sind Sie die einzige Aufsicht. Übergeben Sie die Aufsicht an eine Person oder beenden Sie die Aktivität.",
      "Eine externe Person steht danach unter `Personal` in der Gruppe `Extern`, mit jedem Zugang. In Zeiterfassung und Dienstplan erscheint sie nicht.",
      "`Nur ansehen` fehlt? Diesen Zugang darf nur vergeben, wer Personal verwalten darf.",
      "Fehlt `Spontane Aktivität starten`? Dann hat Ihre OGS in den `Einstellungen` unter `Betrieb` den Schalter `Spontane Aktivitäten erlauben` ausgeschaltet.",
      "Fehlt ein Kind unter `Anwesende Kinder`? Ist seine Gehzeit schon erreicht, steht es nur unter `Alle anwesenden`. Ist es noch nicht angemeldet, suchen Sie es über `Kind hinzufügen`.",
      "Fehlt `Nach Angebot`? Dann hat Ihre OGS kein aktives Betreuungsangebot, das als Betreuungstag zählt.",
      // Einstellung „Kinder nur über ‚Alle Kinder‘ finden“ (#1321).
      "Fehlt `Aktuelle Aufsicht` in der Navigation? Öffnen Sie `Alle Kinder`. Wählen Sie im Menü mit den drei Punkten `Zur Aufsicht`.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [HELP_TOPICS.rooms, HELP_TOPICS.ownGroups],
  };
}
