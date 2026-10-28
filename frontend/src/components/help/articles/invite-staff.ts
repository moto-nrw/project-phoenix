import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Personal anlegen: einzeln ueber `/database/personal` mit Einladung per
 * E-Mail (`InvitationForm`), viele auf einmal unter
 * `/database/personal/import`. An keine der drei Arbeitsweisen gebunden;
 * sichtbar mit `staff:manage` oder `staff:stammdaten` (sidebar.tsx:704).
 */
export function inviteStaffTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadInviteStaff,
    title: "Mitarbeitende anlegen und einladen",
    question: "Wie bekommt das Team Zugang zu moto?",
    summary: "Einzeln über eine Einladung. Viele auf einmal über eine Vorlage.",
    group: "einrichten",
    audience: "lead",
    icon: "UserRoundPlus",
    image: "/help/screens/personal-wiederaufnahme.webp",
    imageAlt:
      "Einladung zur Wiederaufnahme mit der Auswahl für Stammdaten, Qualifikationen und Arbeitszeit",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Person einzeln einladen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
          "Wählen Sie oben rechts `+ Personal` und danach `Personal einladen`.",
          "Tragen Sie die `E-Mail-Adresse` ein.",
          "Zeigt moto eine frühere Beschäftigung? Wählen Sie `Wiederaufnahme` oder `Neuaufnahme`.",
          "Bei `Wiederaufnahme`: Prüfen Sie `Diese Daten übernehmen`. Entfernen Sie ein Häkchen, um den Bereich zurückzusetzen.",
          "Wählen Sie eine `Rolle`.",
          "Tragen Sie bei Bedarf `Vorname (optional)` und `Nachname (optional)` ein.",
          "Tragen Sie bei Bedarf eine `Position (optional)` ein, zum Beispiel `Pädagogische Fachkraft`.",
          "Wählen Sie `Einladung senden`.",
        ],
      },
      {
        // ADR 0091: extern ist ein Merkmal, der Zugang eine eigene Wahl.
        title: "Eine externe Person anlegen",
        description:
          "Zum Beispiel jemand von der Musikschule. Externe Personen führen keine Zeiterfassung. Nehmen Sie dafür nicht `Personal einladen`. Dort zählt die Person als normales Personal.",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
          "Wählen Sie oben rechts `+ Personal` und danach `Externe Person anlegen`.",
          "Tragen Sie `Vorname` und `Nachname` ein. Tragen Sie bei Bedarf die `Organisation (freiwillig)` ein.",
          "Wählen Sie den `Zugang`. Was jede Stufe erlaubt, steht unten.",
          "Bei `Nur ansehen` und `Eigene Termine betreuen` tragen Sie die `E-Mail-Adresse` ein.",
          "Wählen Sie `Anlegen`.",
        ],
      },
      {
        title: "Welchen Zugang wählen?",
        ordered: false,
        steps: [
          "`Kein Zugang`: Die Person bekommt kein Konto und keine E-Mail. Sie steht nur im Plan. Den Termin startet eine OGS-Kraft.",
          "`Nur ansehen`: Die Person bekommt eine Einladung und die Rolle `Gast`. Sie sieht Kinder, Räume und AGs. Elterndaten und Nachrichten sieht sie nicht. Ändern kann sie nichts.",
          "`Eigene Termine betreuen`: Die Person bekommt eine Einladung und die Rolle `AG-Leitung`. Sie startet ihre eigenen Termine, checkt Kinder ein und aus und beendet die Termine.",
        ],
      },
      {
        title: "Eine externe Person später einladen",
        description:
          "In der Liste steht bei ihr `Kein Konto`. Das gilt auch, wenn eine Einladung abgelaufen ist.",
        steps: [
          "Öffnen Sie `Personal` und suchen Sie die Person in der Gruppe `Extern`.",
          "Öffnen Sie das Menü mit den drei Punkten in ihrer Zeile.",
          "Wählen Sie `Konto einladen`.",
          "Wählen Sie den `Zugang`: `Nur ansehen` oder `Eigene Termine betreuen`.",
          "Tragen Sie die `E-Mail-Adresse` ein und wählen Sie `Einladen`.",
        ],
      },
      {
        title: "Viele Personen auf einmal übernehmen",
        description:
          "Für ein ganzes Team ist die Vorlage schneller als einzelne Einladungen.",
        steps: [
          "Öffnen Sie `Personal` und wählen Sie oben `Importieren`.",
          "Wählen Sie bei `Format wählen` `Excel (.xlsx)` oder `CSV (Komma-getrennt)`.",
          "Wählen Sie `Vorlage herunterladen`.",
          "Füllen Sie `Vorname`, `Nachname` und `Rolle` aus. Diese drei sind Pflicht.",
          "Tragen Sie bei Bedarf E-Mail, Personalnummer, Adresse und Vertrag ein.",
          "Wählen Sie unter `Was soll der Import tun?` `Nur neue anlegen`, `Nur bestehende aktualisieren` oder `Beides`.",
          "Laden Sie die Datei hoch und prüfen Sie die Vorschau.",
          "Bestätigen Sie den Import.",
        ],
      },
    ],
    result:
      "Wer eine E-Mail-Adresse hat, bekommt einen Link und legt sein Passwort selbst fest.",
    notes: [
      "Die `Rolle` bestimmt, was die Person später sehen und tun darf.",
      "Eine externe Person können Sie sofort einplanen, auch mit `Kein Zugang`. Bis sie eine Einladung annimmt, steht bei ihr `Einladung offen`.",
      "Steht bei einer externen Person `Einladung offen` und die E-Mail kam nicht an? Wählen Sie im Menü `Einladung erneut senden`. Dort können Sie auch die Adresse korrigieren.",
      "Nach der Annahme steht die Person unter `Mitarbeiter` und lässt sich einer Gruppe zuweisen.",
      "Bei `Wiederaufnahme` wird die frühere Beschäftigung fortgeführt. Ausgewählte Bereiche bleiben unverändert. Zurückgesetzte Angaben bleiben als Historie erhalten.",
      "`Arbeitszeit` umfasst Stundenkonto, Urlaub und Arbeitszeitmodell gemeinsam. Nach dem Zurücksetzen startet der Urlaub mit dem vollen Jahresanspruch. Übertrag gibt es dann nicht. Den Jahresanspruch können Sie danach korrigieren.",
      "Ist eine übernommene Personalnummer inzwischen vergeben, bleibt sie leer.",
      "Bei `Neuaufnahme` beginnt eine neue Beschäftigung an derselben Person. Die frühere bleibt beendet. Stammdaten, Qualifikationen, Stundenkonto und Urlaub beginnen neu.",
      "Den früheren Zeitraum sehen Sie unter der `E-Mail-Adresse`. Dokumente und künftige Schichten und Abwesenheiten kommen nicht zurück. Gruppen, Klassen und Vertretungen legen Sie danach neu fest.",
      "Ist die Person bei der Annahme schon wieder beschäftigt? Dann wird nur der Schulzugang hergestellt. Die eingeladene Person muss nichts zur Beschäftigung entscheiden.",
      "Das Blatt `Hinweise` in der Vorlage erklärt jede Spalte.",
    ],
    differences: [
      "Beim Import muss die Spalte `Rolle` genau einer vorhandenen Rolle entsprechen.",
      "Zur Wahl stehen `Verwaltung`, `Betreuung`, `Lehrkraft`, `Schulverwaltung`, `AG-Leitung`, `Gast` und die eigenen Rollen der Schule. Die alten Namen `Administrator` und `Betreuer` erkennt moto auch.",
      "`Nur ansehen` fehlt bei einer externen Person? Diesen Zugang darf nur vergeben, wer Personal verwalten darf.",
      "Fehlt in einer Zeile die E-Mail-Adresse, gibt es keinen Zugang. Die Person steht dann nur in der Liste.",
      "Auf dem Handy steht `+ Personal` als runder Knopf unten rechts. Er öffnet dieselbe Auswahl.",
    ],
    troubleshootingDetails: [
      "Die Einladung kam nicht an? Prüfen Sie die E-Mail-Adresse und den Spam-Ordner. Danach laden Sie erneut ein.",
      "Eine Zeile wird als Fehler gemeldet? Dann gibt es die Person schon.",
      "moto erkennt eine bekannte Zeile an der Personalnummer, sonst an der E-Mail, sonst am Namen.",
      "Fehlt `Personal` in der Seitenleiste? Dafür braucht Ihre Rolle ein eigenes Recht.",
    ],
    related: [
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadStaffPermissions,
      HELP_TOPICS.leadTeacherAccess,
    ],
  };
}
