import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

// OGS-Leitung und Verwaltung. Geruest nach der Informationsarchitektur
// (#2229, Abschnitt 7): Einrichtung zuerst, danach die wiederkehrende
// Verwaltung. Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App
// geprueft, deshalb bleiben es Entwuerfe.
//
// Was eine Leitung genauso erledigt wie eine Betreuungskraft, steht nicht
// hier: `login`, `installApp`, `studentSearch`, `parentMessage`,
// `parentRequests`, `dayLog` und `sharedFiles` tragen beide Rollen in
// `audience` und erscheinen dadurch in beiden Seitenleisten.
/**
 * Raumkatalog unter `/database/rooms`. Die Seite selbst haengt an keiner
 * Einstellung: sie steht weder in `BINARY_HIDDEN_HREFS` noch in
 * `NFC_ONLY_HREFS` (sidebar.tsx), und jede Gruppe bekommt ueber
 * `groups.config.tsx` einen Raum zugewiesen -- auch bei einfacher Anwesenheit.
 *
 * Zwei Felder wirken aber nur bei detaillierter Anwesenheit: `scanBinary`
 * (backend/modules/devicescan/internal/application/scan.go) schaltet nur die
 * Anwesenheit um und legt keinen Raumbesuch an, und `checkRoomCapacity`
 * (checkin.go) zaehlt genau diese offenen Raumbesuche.
 */
export function roomsCatalogTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const tracksRooms = presenceMode !== "binary";
  return {
    id: HELP_TOPICS.leadRooms,
    title: "Räume anlegen",
    question: "Wie lege ich die Räume der OGS an?",
    summary: "Legen Sie jeden Raum einmal an. Gruppen brauchen einen Raum.",
    group: "einrichten",
    audience: "lead",
    icon: "Building2",
    // Kein `requirements`: wer die Leitungshilfe liest, hat die Rechte der
    // Leitung. Der Fall „Bereich fehlt trotzdem" steht bei den Problemen,
    // wo er zu einer Handlung fuehrt.
    steps: [],
    instructionGroups: [
      {
        title: "Einen Raum anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Räume`.",
          "Wählen Sie oben rechts `+ Raum`.",
          "Tragen Sie bei `Raumname` den Namen des Raums ein.",
          "Wählen Sie eine `Kategorie`.",
          "Tragen Sie bei Bedarf `Gebäude` und `Etage` ein.",
          "Wählen Sie bei Bedarf eine `Farbe`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Einen Raum ändern",
        steps: [
          "Öffnen Sie `Räume` und wählen Sie den Raum aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einen Raum löschen",
        steps: [
          "Öffnen Sie `Räume` und wählen Sie den Raum aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Der Raum steht sofort zur Auswahl. Sie können ihn einer Gruppe zuweisen.",
    notes: [
      "Die Farbe erscheint später an jedem Ort-Schild dieses Raums.",
      "Oben rechts fassen Sie die Liste unter `Gruppieren` nach `Gebäude`, `Etage` oder `Keine` zusammen.",
      ...(tracksRooms
        ? [
            "`Maximale Belegung` begrenzt, wie viele Kinder gleichzeitig im Raum sind.",
            "Mit `Offener Raum` dürfen Kinder den Raum jederzeit als Ziel wählen. Eine Aufsicht ersetzt das nicht.",
          ]
        : [
            "`Maximale Belegung` und `Offener Raum` wirken bei Ihnen nicht. Ihre OGS hält nur fest, ob ein Kind da ist.",
          ]),
    ],
    differences: [
      "Auf dem Handy steht statt `+ Raum` ein rundes Plus unten rechts.",
      "`Schulhof`, `WC` und `Toilette` legt moto selbst an. Ihre Namen sind fest, und `Löschen` fehlt.",
      "Beim `WC` und bei der `Toilette` fehlen zusätzlich `Farbe` und `Offener Raum`.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Lässt sich ein Raum nicht löschen? Dann ist er gerade belegt. Oder ein Betreuungsangebot braucht ihn.",
      ...(tracksRooms
        ? [
            "Meldet moto oder das Tablet, der Raum ist voll? Dann ist `Maximale Belegung` erreicht. Erhöhen Sie die Zahl hier. Ist dagegen die Aktivität voll, ändern Sie `Maximale Teilnehmer` unter `Aktivitäten`.",
          ]
        : []),
    ],
    related: [
      HELP_TOPICS.leadGroups,
      HELP_TOPICS.leadActivities,
      HELP_TOPICS.dataManagement,
    ],
  };
}
