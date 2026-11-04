import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Die wichtigsten Einstellungen je Reiter (#3727). Bewusst eine Auswahl, keine
 * vollständige Liste: Die vollständige Liste steht auf `/settings` selbst, mit
 * einem Satz unter jeder Einstellung. Bezeichnungen stammen aus der Registry
 * (`backend/modules/settings/internal/defaults/`) und aus `TAB_LABELS` in
 * `components/settings/settings-filter.ts`. Wer dort umbenennt, ändert hier mit.
 */
export function settingsGuideTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.settingsGuide,
    title: "Die wichtigsten Einstellungen im Überblick",
    question: "Welche Einstellungen gibt es, und was bewirken sie?",
    summary: "Was Sie in welchem Bereich der Einstellungen festlegen.",
    group: "konfiguration",
    audience: "lead",
    icon: "SlidersHorizontal",
    steps: [],
    instructionGroups: [
      {
        title: "Betrieb",
        description: "Wie Ihr Team im Alltag arbeitet.",
        ordered: false,
        steps: [
          "`Arbeit mit festen Gruppen`: Ob Kinder festen Gruppen angehören oder alle mit allen Kindern arbeiten.",
          "`Welche Gruppen und Blöcke sieht das Team?`: Was das Team im Überblick sieht. Was das Team bearbeiten darf, ändert das nicht.",
          "`Wo darf das Team Kinder an- und abmelden?`: Nur bei den eigenen Gruppen und Blöcken oder überall.",
          "`Wer darf Kinder krank oder entschuldigt melden?`: Nur OGS-Admins oder das ganze Team.",
          "`Übliche Ankunftszeit` und `Übliche Abholzeit`: Diese Zeiten tragen Sie im Wochenplan eines Kindes mit einem Klick ein.",
          "`Betreuungsplan nutzen`: Zeigt den Betreuungsplan im Menü.",
          "`Benachrichtigungen senden`: Hinweise in moto und auf dem Handy. Was ankommt, wählt jede Person selbst.",
          "`Team-Chat für Mitarbeitende`: Das Team schreibt sich Nachrichten. Eltern sehen davon nichts.",
          "`Kinderfotos`: Fotos bei den Kindern. Nur mit Einwilligung der Eltern. Beim Ausschalten löscht moto alle Fotos sofort. Sie können sie nicht wiederherstellen.",
          "`Automatisch ausstempeln`: moto wartet nach Dienstende die eingestellte Zeit. Dann stempelt es aus. Das gilt nur bei einem geplanten Dienst.",
        ],
      },
      {
        title: "Elternportal",
        description: "Steht im Bereich `Betrieb` unter `Elternportal`.",
        ordered: false,
        steps: [
          "`Nachrichten von Eltern`: Eltern schreiben dem Team zu ihrem Kind.",
          "`Krankmeldung über Elternportal`: Eltern melden ihr Kind selbst krank.",
          "`Speiseplan`: Eltern sehen, was es diese und nächste Woche zu essen gibt.",
          "Alle Schalter für Eltern erklärt die Anleitung „Festlegen, was Eltern in moto sehen“.",
        ],
      },
      {
        title: "Erinnerungen",
        description: "Hinweise rund um Abholung, Aktivitäten und Termine.",
        ordered: false,
        steps: [
          "`Anstehende Abholung`: Erinnert, wenn die Abholzeit eines Kindes näher rückt.",
          "`Überfällige Abholung`: Erinnert, wenn ein Kind nach seiner Abholzeit noch da ist.",
          "`AG dauert länger`: Erinnert, wenn eine gestartete Aktivität nach ihrem Ende noch läuft. Sie sehen den Raum und bei wie vielen Kindern die Abholzeit erreicht ist. Bei `Nachlauf in Minuten` legen Sie fest, wie lange moto wartet. Beendet moto Termine automatisch, wählen Sie dort weniger Minuten als den Puffer.",
          "Die Erinnerung verschwindet, sobald jemand die Aktivität beendet.",
          "`Terminerinnerung für Eltern`: Eltern bekommen vor einem Termin eine E-Mail. Das gilt nur, wenn Sie beim Anlegen `Eltern per E-Mail benachrichtigen` wählen.",
        ],
      },
      {
        title: "Geräte",
        description: "Nur wenn Ihre OGS NFC-Tablets nutzt.",
        ordered: false,
        steps: [
          "`Geräte-PIN`: Mit dieser PIN meldet sich Ihr Team am Tablet an.",
          "`„Nach Hause“ ab Uhrzeit`: Ab dieser Uhrzeit können Kinder am Tablet „Nach Hause“ wählen.",
          "`„Toilette“ am Tablet anbieten`: Kinder können beim Auschecken „Toilette“ wählen. Für „Schulhof“ und „Raumwechsel“ gibt es eigene Schalter.",
        ],
      },
      {
        title: "Module",
        description: "Welche Zusatzmodule Ihre OGS gebucht hat.",
        ordered: false,
        steps: [
          "`Gebuchte Module`: Zeigt die Zusatzmodule Ihrer OGS.",
          "Die Schalter sind nur zur Information. Sie können sie nicht ändern.",
          "Die gebuchten Module stehen im Vertrag mit moto. Wenn Sie etwas ändern möchten, wenden Sie sich an das moto-Team.",
          "Mehr dazu steht in der Anleitung „Welche Module hat meine Schule?“.",
        ],
      },
      {
        title: "Anmeldung",
        description: "Wie Eltern ihr Kind bei Ihnen anmelden.",
        ordered: false,
        steps: [
          "`Online-Anmeldung`: Eltern melden ihr Kind über ein Formular im Internet an.",
          "`Empfänger für neue Anmeldungen`: Diese Adressen bekommen bei jeder neuen Anmeldung eine E-Mail.",
          "`Rechtstexte`: Was Eltern im Formular lesen und bestätigen, zum Beispiel die Datenschutzinformation.",
        ],
      },
      {
        title: "Datenschutz",
        description: "Was moto speichert und wie lange.",
        ordered: false,
        steps: [
          "`Anwesenheitsprotokoll`: Das Team sieht die Anwesenheit und frühere Räume. Ausgeschaltet bleiben frühere Raumbesuche nur für den heutigen Tag. Beim Ausschalten bestätigen Sie zuerst die endgültige Löschung. Geben Sie dafür `LÖSCHEN` ein. Das gilt auch bei `Zurücksetzen` auf den ausgeschalteten Standard. Abgeschlossene Raumbesuche vor heute werden sofort gelöscht. Das lässt sich nicht rückgängig machen. Erneutes Einschalten stellt nichts wieder her. Heutige Raumbesuche bleiben bis zum nächtlichen Löschen erhalten. Der aktuelle Raum und die Anwesenheitszeiten bleiben erhalten.",
          "Vor dem Einschalten prüft die Schule die Rechtsgrundlage. Sie ist für die Nutzung verantwortlich.",
          "`Nutzungsanalyse`: moto erhält Nutzungsstatistiken Ihrer Schule, aber keine eingegebenen Inhalte.",
          "Löschfristen wie `Anwesenheit aufbewahren (Tage)`: Wie lange moto Daten behält. Sie können sie nur lesen.",
        ],
      },
      {
        title: "Kalender und Export",
        description: "Termine und Zeitkonten außerhalb von moto nutzen.",
        ordered: false,
        steps: [
          "`Kalenderzugang mit App-Passwort erlauben`: Das Team sieht seine Termine in einem Kalenderprogramm.",
          "`Zeitkonten per SFTP übertragen`: Nach der vollständigen Einrichtung können Sie die Zeitkonten im Export-Dialog selbst übertragen. Auch der automatische Versand an das Lohnbüro nutzt diese Gegenstelle, wenn unter `Abrechnung` der `Versandweg` `SFTP` gewählt ist.",
        ],
      },
      {
        title: "Sicherheit",
        description: "Wie gut Konten bei der Anmeldung geschützt sind.",
        ordered: false,
        steps: [
          "`Zwei-Faktor-Anmeldung (Code per E-Mail)`: Beim Anmelden kommt zusätzlich ein Code per E-Mail.",
          "`Fehlversuche bis zur Kontosperre`: Nach so vielen falschen Bestätigungscodes pausiert die Code-Prüfung.",
        ],
      },
      {
        title: "Startseite für alle und Personalisierung",
        ordered: false,
        steps: [
          "`Startseite für alle`: Welche Bereiche alle auf ihrer Startseite sehen.",
          "`Personalisierung`: Ein eigenes Bild für die Login-Seite Ihrer OGS.",
        ],
      },
    ],
    notes: [
      "Unter jeder Einstellung steht in moto ein Satz, der sagt, was sie bewirkt.",
      "Sie suchen eine bestimmte Einstellung? Nutzen Sie oben das Feld `Einstellung suchen`. Es sucht in allen Bereichen.",
    ],
    differences: [
      "Welche Bereiche und Einstellungen Sie sehen, hängt von Ihren Rechten und den Funktionen Ihrer OGS ab.",
      "Manche Einstellungen erscheinen erst, wenn Sie eine andere einschalten.",
    ],
    related: [
      HELP_TOPICS.settings,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.leadNfcSettings,
    ],
  };
}
