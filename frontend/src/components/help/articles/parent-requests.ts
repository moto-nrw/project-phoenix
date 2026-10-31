import type { HelpTopic, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function parentRequestsTopic(groupMode: HelpGroupMode): HelpTopic {
  return {
    id: HELP_TOPICS.parentRequests,
    title: "Anfragen von Eltern prüfen und bearbeiten",
    question: "Wie prüfe und bearbeite ich Anfragen von Eltern?",
    summary:
      "Vergleichen Sie die bisherigen Angaben mit dem Elternwunsch. Tragen Sie danach Ihre Entscheidung ein.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "ListChecks",
    requirements: [
      "Ihr Konto darf Kinderdaten bearbeiten.",
      // Wer freigibt, legt die Leitung fest (#3804): Admins, zustaendige
      // Gruppenleitungen oder das berechtigte Team.
      "Ihre OGS lässt Sie diese Anfragen freigeben. Das legt die Leitung fest.",
    ],
    steps: [
      "Öffnen Sie `Anfragen` in der Seitenleiste.",
      "Wählen Sie den Tab `Eltern`.",
      "Suchen Sie nach dem Namen des Kindes oder wählen Sie unter `Anfrageart` einen Filter.",
      "Öffnen Sie die Anfrage und vergleichen Sie `Aktuell` mit `Gewünscht`.",
      "Wählen Sie `Freigeben`, wenn die Änderung übernommen werden soll.",
      "Oder tragen Sie eine kurze Begründung ein und wählen Sie `Ablehnen`.",
    ],
    result:
      "moto übernimmt die freigegebene Änderung. Die Bezugsperson sieht die Entscheidung in der Eltern-App.",
    differences: [
      // #5024: Freigeben braucht standardmäßig keine Begründung mehr.
      "Zeigt das Feld `Begründung (Pflicht)` statt `Begründung (Pflicht bei Ablehnung)`? Dann verlangt Ihre OGS auch beim Freigeben eine Begründung. Tragen Sie sie vorher ein.",
      "Bei einer geänderten Abholzeit zeigt moto vorher betroffene Termine an.",
      // #3945: Tagesanfrage mit Abholart.
      "Eine Anfrage für einen einzelnen Tag kann die Abholzeit, die Abholart oder beides ändern. Die neue Abholart gilt nur an diesem Tag.",
      // #4780: Abholperson fuer einen Tag und Abholberechtigung je Wochentag.
      "Eine Anfrage für einen Tag kann auch eine Abholperson nennen. Nach dem Freigeben steht beim Kind für diesen Tag `Abholung durch ... (Erlaubnis der Eltern)`.",
      "Eine `Abholberechtigung` trägt eine Kontaktperson als abholberechtigt ein, an allen Tagen oder nur an bestimmten Tagen. Sie gilt erst nach dem Freigeben.",
      "Haben die Eltern unterschrieben, öffnen Sie mit `Nachweis (PDF)` die Erklärung und die Unterschrift.",
      // #3946: freigegebene Tagesänderung markiert die Kinderkarte.
      "Ist der Tag heute, zeigt die Kinderkarte nach dem Freigeben unten einen orangen Hinweis.",
      // #5102: von Hand gepflegte Regeltermine nach einer Angebotsanfrage.
      "Planen von Hand gepflegte Regeltermine das Kind nach einer Angebotsanfrage weiter ein? Dann bleibt die Anfrage nach dem Freigeben offen stehen. Sie zeigt `Nicht angepasst, weil von Hand gepflegt`. Wählen Sie `Dort auch entfernen` oder `Fertig`.",
      // #5137: Klassen-Regeltermine holen das Kind wieder herein.
      "Plant ein Regeltermin alle Kinder einer Klasse, eines Jahrgangs oder einer Gruppe, steht dort `Mit Angebot verbinden`. Nur so fehlt das Kind dauerhaft.",
      // #4767: Warteliste je AG; Nachrücken ist die normale Freigabe.
      "Fragen Eltern einen Platz in einer vollen AG an? Dann zeigt die Anfrage zum Beispiel `Warteliste Platz 2`. Die älteste Anfrage ist zuerst dran.",
      "Oben steht die Karte `Wartelisten`. Sie zeigt je AG die belegten Plätze und die wartenden Kinder in Reihenfolge. `Zur Anfrage` öffnet die Anfrage des Kindes.",
      "Ist in einer AG ein Platz frei, steht dort `Platz frei` und beim ersten Kind `Als Nächstes dran`. Wählen Sie bei seiner Anfrage `Freigeben`, dann rückt es nach.",
      "Anmeldungsänderungen öffnen Sie über `Prüfen`. Dafür brauchen Sie ein zusätzliches Recht.",
      // Ohne feste Gruppen gibt es keine Gruppenleitung, an der die
      // Freigabe haengen koennte.
      ...(groupMode === "open_care"
        ? []
        : [
            "Fehlt eine Anfrage? Je nach Einstellung Ihrer OGS sehen Sie nur Kinder Ihrer Gruppen.",
          ]),
    ],
    notes: [
      "Über `Historie` sehen Sie bereits entschiedene oder zurückgezogene Anfragen.",
    ],
    troubleshootingDetails: [
      // Live geprueft: ohne das Recht steht `Anfragen` nicht in der
      // Seitenleiste, und der Aufruf der Adresse landet auf der Startseite.
      "Fehlt `Anfragen` in der Seitenleiste? Dann darf Ihre Rolle keine Elternanfragen bearbeiten. Fragen Sie Ihre Leitung.",
    ],
    related: [
      HELP_TOPICS.parentMessage,
      HELP_TOPICS.editStudent,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
