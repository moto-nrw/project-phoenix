import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/ferien` (#4659): Ferienzeitraum mit Wochen und Zeitmodellen.
 * Mittagessen je Woche mit Preis für ein Essen (#5631).
 */
export function traegerHolidayPeriodTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayPeriod,
    title: "Ferienzeitraum anlegen",
    question: "Wie plane ich Ferienbetreuung für mehrere Schulen?",
    summary:
      "Ein Ferienzeitraum hat buchbare Wochen. Jede Woche findet an einer Schule Ihres Trägers statt. Dieselben Tage kann es an mehreren Schulen geben.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "SunHorizon",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Ferienzeitraum anlegen",
        steps: [
          "Wählen Sie in der Navigation `Ferienbetreuung`.",
          "Wählen Sie `Ferienzeitraum anlegen`.",
          "Tragen Sie den `Name des Ferienzeitraums` ein, zum Beispiel Osterferien 2027.",
          "Tragen Sie `Anmeldebeginn` und `Anmeldeschluss` ein.",
          "Ist die Anmeldung verbindlich? Dann lassen Sie den Haken bei `Die Anmeldung ist verbindlich.`.",
          "Schreiben Sie unter `Hinweise für Eltern`, was Eltern wissen müssen. Zum Beispiel, ob ein Lunchpaket nötig ist.",
          "Soll ein Kind nur eine bestimmte Zahl an Tagen kommen? Tragen Sie die Zahl bei `Höchstens Tage je Kind` ein, zum Beispiel 5. Ohne Grenze lassen Sie das Feld leer.",
          "Wählen Sie bei `Wie werden die Plätze vergeben?` die Karte `Nach Eingang` oder `Nach Kriterien`. Nach Eingang bekommt zuerst einen Platz, wer sich zuerst anmeldet. Nach Kriterien entscheidet Ihr Büro.",
        ],
      },
      {
        title: "Wochen und Zeitmodelle planen",
        steps: [
          "Tragen Sie bei jeder Woche `Erster Tag` und `Letzter Tag` ein.",
          "Wählen Sie die `Ausrichtende OGS`. Dort findet die Betreuung in dieser Woche statt.",
          "Liegt ein Feiertag oder Schließtag der Schule in der Woche, zeigt moto einen Hinweis. Bei `Ganze Woche` wählen Sie dann andere Tage oder `Einzelne Tage`.",
          "Wählen Sie bei `Buchung`, wie Eltern die Woche buchen: `Ganze Woche` oder `Einzelne Tage`. Bei einzelnen Tagen wählen Eltern Tage von Montag bis Freitag. Unter `Buchbare Tage` sehen Sie, welche Tage Eltern buchen können. Ein Tag ohne Betreuung zeigt „–“ und den Grund.",
          "Tragen Sie die Zeitmodelle ein, zum Beispiel „bis 13 Uhr“ und „bis 16 Uhr“. Jedes Zeitmodell hat `Beginn`, `Ende` und einen Preis. Bei `Ganze Woche` ist es der `Preis in Euro` für die Woche, bei `Einzelne Tage` der `Preis je Tag in Euro`.",
          "Gibt es in der Woche Mittagessen? Schalten Sie `Mittagessen anbieten` ein. Tragen Sie den `Preis für ein Essen in Euro` ein, zum Beispiel 4,30.",
          "Weitere Wochen fügen Sie mit `Woche hinzufügen` hinzu.",
          "Dieselben Tage an mehreren Schulen? Legen Sie je Schule eine Woche mit denselben Tagen an. Eltern wählen dann für ihr Kind eine Schule. An derselben Schule dürfen sich Wochen nicht überschneiden.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Annahme-Mail schreiben",
        description:
          "Bestätigen Sie eine Anmeldung, bekommen die Eltern eine E-Mail. Kinder, Wochen und Zeitmodell setzt moto selbst ein.",
        steps: [
          "Öffnen Sie den Ferienzeitraum in der Liste `Ferienbetreuung`.",
          "Schreiben Sie unter `Text der Annahme-Mail`, was Eltern noch wissen müssen. Zum Beispiel: Mittagessen mitgeben, Wechselkleidung, Sportsachen, Regenkleidung.",
          "Rechts unter `Vorschau` sehen Sie die E-Mail so, wie Eltern sie lesen.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Veröffentlichen",
        steps: [
          "Öffnen Sie den Ferienzeitraum in der Liste `Ferienbetreuung`.",
          "Wählen Sie oben `Veröffentlichen`.",
          "Bestätigen Sie mit `Veröffentlichen`.",
        ],
      },
    ],
    result:
      "Der Ferienzeitraum ist für die Anmeldung freigegeben. Sie können ihn danach weiter ändern.",
    notes: [
      "Der `Text der Annahme-Mail` ist freiwillig. Ohne Text bekommen Eltern die E-Mail ohne Hinweise. Sie können den Text bis zum Bestätigen jederzeit ändern. Schon verschickte E-Mails ändern sich nicht.",
      "Eltern mit Elternkonto sehen in der Annahme-Mail den Knopf `In der Eltern-App ansehen`.",
      "Mit `Ferienwunsch aus der OGS-Anmeldung berücksichtigen` warten Kinder Ihrer Schulen ohne Ferienwunsch auf der Warteliste hinter den anderen.",
      "Ändern Sie die Zeiten eines Zeitmodells? Die neuen Zeiten gelten ab dem Änderungstag. Vergangene gebuchte Tage behalten ihre bisherigen Zeiten. Auch nach Umbuchung oder Archivierung bleiben diese Zeiten erhalten.",
      "Ein gespeicherter Ferienzeitraum ist zuerst ein Entwurf. Ein Entwurf ist nicht für die Anmeldung freigegeben.",
      "Ein veröffentlichter Ferienzeitraum wird nicht wieder zum Entwurf.",
      "Ein Zeitmodell mit Anmeldungen lässt sich nicht entfernen. Wählen Sie dann `Archivieren`. Neu buchen kann es danach niemand mehr. Gebuchte Tage behalten es. Zurückholen lässt es sich nicht.",
      "`Höchstens Tage je Kind` gilt für alle Wochen zusammen. Es gilt auch, wenn Eltern dasselbe Kind ein zweites Mal anmelden. moto erkennt das Kind an der E-Mail-Adresse der Eltern, am Namen und am Geburtsdatum. Eine ganze Woche zählt mit allen Tagen von Montag bis Freitag. Eltern dürfen die Tage auf mehrere Wochen verteilen. Senken Sie die Zahl später, behalten Kinder ihre gebuchten Tage.",
      "Bei `Einzelne Tage` zahlt die Familie den Preis je Tag mal die gebuchten Tage.",
      "Mit `Mittagessen anbieten` wählen Eltern das Essen für jedes Kind und jede Woche dazu. Das Kind isst an jedem gebuchten Tag. Bei einer ganzen Woche sind das die Tage von Montag bis Freitag. Unter dem Preis steht, was das zusammen kostet.",
      "Das Mittagessen steht als eigener Betrag im Beitrag und auf der Rechnung. Geschwisterermäßigung, Befreiung und Kostenübernahme gelten nur für die Betreuung, nicht für das Essen.",
      "Ändern Sie den Preis für ein Essen, gilt er für neue Anmeldungen. Bestätigte Anmeldungen behalten ihren Preis.",
      "Eltern sehen den Preis jedes Zeitmodells bei der Wahl der Wochen. Aus den Preisen rechnet moto den Beitrag der Familien. Geschwister und Befreiung legt die Abrechnung unter `Abrechnungsvorgaben` fest.",
    ],
    differences: [
      "`Nach Eingang` und `Nach Kriterien` lassen sich nicht wählen? Es gibt schon Anmeldungen. Die Platzvergabe bleibt dann so.",
      "`Ganze Woche` und `Einzelne Tage` lassen sich nicht wählen? Für die Woche gibt es schon Anmeldungen. Die Buchungsart bleibt dann so. Bei einzelnen Tagen bleiben auch die Tage der Woche.",
      "`Mittagessen anbieten` lässt sich nicht ausschalten? Für die Woche ist schon Mittagessen gebucht. Das Angebot bleibt dann. Den Preis können Sie ändern.",
      "Sie sehen in der Navigation kein `Ferienbetreuung`? Ihre Rolle darf keine Anmeldungen verwalten. Fragen Sie im Träger-Büro nach der Rolle Verwaltung.",
    ],
    troubleshootingDetails: [
      "`Veröffentlichen` lässt sich nicht wählen? Speichern Sie zuerst Ihre Änderungen. Jede Woche braucht mindestens ein Zeitmodell, das nicht archiviert ist.",
      "Eine Schule fehlt bei `Ausrichtende OGS`? Das moto-Team ordnet die Schulen Ihrem Träger zu.",
      "Die Woche lässt sich nicht speichern, weil keine Betreuung ist? Eine ganze Woche mit Feiertag oder Schließtag kann niemand buchen. Wählen Sie bei `Buchung` `Einzelne Tage`. Dann fällt nur dieser Tag weg. Oder lassen Sie den Tag aus der Woche.",
      "Eine Woche lässt sich nicht verlängern? Ein Kind ist schon für die ganze Woche angemeldet. Mit mehr Tagen hätte es mehr als `Höchstens Tage je Kind`. Lassen Sie die Woche so oder erhöhen Sie die Zahl.",
    ],
    related: [
      HELP_TOPICS.traegerLogin,
      HELP_TOPICS.traegerHolidayForm,
      HELP_TOPICS.traegerHolidayContribution,
      HELP_TOPICS.traegerClosingDays,
      HELP_TOPICS.traegerHolidayWish,
      HELP_TOPICS.traegerHolidayAllocation,
    ],
  };
}
