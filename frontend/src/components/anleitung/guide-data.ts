import {
  BadgeCheck,
  CalendarDays,
  ClipboardList,
  FileText,
  HeartHandshake,
  LayoutDashboard,
  MonitorSmartphone,
  Radio,
  Settings,
  ShieldCheck,
  Sparkles,
  TabletSmartphone,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type GuideTone = "blue" | "green" | "orange" | "red" | "purple" | "gray";

interface GuideCallout {
  readonly title: string;
  readonly body: string;
  readonly tone?: GuideTone;
}

export interface GuideSection {
  readonly id: string;
  readonly title: string;
  readonly kicker: string;
  readonly body: string;
  readonly steps?: readonly string[];
  readonly checklist?: readonly string[];
  readonly callout?: GuideCallout;
  readonly screenshot?: string;
  readonly columns?: readonly {
    readonly title: string;
    readonly items: readonly string[];
  }[];
}

export interface GuideChapter {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly icon: LucideIcon;
  readonly tone: GuideTone;
  readonly sections: readonly GuideSection[];
}

export const standardHighlights = [
  {
    title: "Einrichtung vorbereiten",
    body: "Kinder, Personal, Räume, Gruppen und Aktivitäten sauber anlegen.",
    icon: ClipboardList,
  },
  {
    title: "OGS-Alltag steuern",
    body: "Kinder suchen, Aufsicht führen, spontane Aktivitäten starten und Räume prüfen.",
    icon: LayoutDashboard,
  },
  {
    title: "Verwaltung abschließen",
    body: "Stundenplan, Zeiterfassung, Anmeldungen, Feedback und Einstellungen nutzen.",
    icon: BadgeCheck,
  },
] as const;

export const nfcHighlights = [
  {
    title: "moto vorbereiten",
    body: "Alle Standard-Schritte plus klare Benennung für Tablet und Armbänder.",
    icon: TabletSmartphone,
  },
  {
    title: "NFC sauber starten",
    body: "Aktivitäten, Räume, Gruppen und Geräte vor dem ersten Einsatz prüfen.",
    icon: Radio,
  },
  {
    title: "Fehler schneller finden",
    body: "Wenn ein Kind nicht gefunden wird, zuerst Stammdaten und Status prüfen.",
    icon: ShieldCheck,
  },
] as const;

export const standardChapters: readonly GuideChapter[] = [
  {
    id: "erste-schritte",
    title: "Erste Schritte",
    description:
      "Alles, was vor dem ersten vollständigen Nutzungstag vorbereitet sein sollte.",
    icon: Sparkles,
    tone: "green",
    sections: [
      {
        id: "konto",
        kicker: "Zugang",
        title: "Konto erstellen und anmelden",
        body: "Administratoren werden per E-Mail zu moto eingeladen. Danach kann das Konto erstellt und moto geöffnet werden.",
        steps: [
          "Einladungs-Mail von moto öffnen.",
          "Dem Link in der E-Mail folgen.",
          "Konto erstellen.",
          "moto öffnen.",
          "`E-Mail-Adresse` eingeben.",
          "`Passwort` eingeben.",
          "Auf `Anmelden` klicken.",
        ],
        callout: {
          title: "Passwort vergessen",
          body: "Wenn das Passwort nicht bekannt ist, auf `Passwort vergessen?` klicken und den Anweisungen folgen.",
          tone: "gray",
        },
        screenshot:
          "Loginseite mit Ihr OGS Portal, E-Mail-Adresse, Passwort und Anmelden",
      },
      {
        id: "navigation",
        kicker: "Orientierung",
        title: "Die wichtigsten Bereiche finden",
        body: "Die Navigation ist der feste Ausgangspunkt. Für die Einrichtung ist zuerst die Datenverwaltung wichtig. Für den Alltag sind Kindersuche, Räume, aktuelle Aufsicht, Stundenplan und Zeiterfassung zentral.",
        columns: [
          {
            title: "Alltag",
            items: [
              "`Home`",
              "`Kindersuche`",
              "`Aktivitäten`",
              "`Räume`",
              "`Mitarbeiter`",
              "`Vertretungen`",
            ],
          },
          {
            title: "Planung und Verwaltung",
            items: [
              "`Stundenplan`",
              "`Zeiterfassung`",
              "`Feedback`",
              "`Einstellungen`",
              "`Datenverwaltung`",
              "`Anmeldungen`",
            ],
          },
        ],
        screenshot: "Desktop Navigation und mobile Navigation",
      },
      {
        id: "kinder",
        kicker: "Kinderdaten",
        title: "Kinder manuell anlegen oder importieren",
        body: "Kinder können einzeln angelegt oder über eine Excel- oder CSV-Datei importiert werden. Für viele Kinder ist der Import der schnellste Weg.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "`Kinder` öffnen.",
          "Für ein einzelnes Kind ein neues Kind anlegen.",
          "`Vorname`, `Nachname`, `Klasse` und `OGS Gruppe` eintragen.",
          "`Erziehungsberechtigte` ausfüllen.",
          "Bei Bedarf `Fährt mit dem Bus` aktivieren.",
          "`Datenschutzeinwilligung erteilt` nur aktivieren, wenn die Einwilligung vorliegt.",
          "Für viele Kinder auf `Importieren` klicken.",
          "`Excel (.xlsx)` oder `CSV (Komma-getrennt)` wählen.",
          "`Vorlage herunterladen`, Datei ausfüllen, hochladen und `Datenvorschau` prüfen.",
          "Zum Abschluss auf `Schüler importieren` klicken.",
        ],
        callout: {
          title: "Geburtstage im Import",
          body: "Geburtstage können als `JJJJ-MM-TT`, `TT.MM.JJJJ` oder `TT.MM.JJ` eingetragen werden.",
          tone: "blue",
        },
        screenshot:
          "Kinderimport mit Vorlage herunterladen, Datei hochladen und Datenvorschau",
      },
      {
        id: "basisdaten",
        kicker: "Grundstruktur",
        title: "Mitarbeitende, Räume, Gruppen und Aktivitäten anlegen",
        body: "Diese Daten bilden die Grundlage für Aufsicht, Planung und Suche. Klare Namen helfen später im Alltag.",
        columns: [
          {
            title: "Personal",
            items: [
              "`Vorname` und `Nachname` eintragen.",
              "`E-Mail` eintragen.",
              "Bei Bedarf `Rolle`, `Qualifikationen` und `Interne Notizen` ergänzen.",
              "`Temporäres Passwort` vergeben, wenn das Feld angezeigt wird.",
            ],
          },
          {
            title: "Räume und Gruppen",
            items: [
              "`Raumname`, `Kategorie`, `Gebäude`, `Etage` und `Farbe` pflegen.",
              "Kategorien: `Normaler Raum`, `Gruppenraum`, `Themenraum`, `Sport`.",
              "`Gruppenname`, `Gruppenraum` und `Gruppenleitung` pflegen.",
            ],
          },
          {
            title: "Aktivitäten",
            items: [
              "`Name` eintragen.",
              "`Kategorie` wählen.",
              "`Maximale Teilnehmer` eintragen.",
              "Bei Bedarf `Hauptbetreuer` auswählen.",
            ],
          },
        ],
        checklist: [
          "Alle benötigten Räume sind angelegt.",
          "Alle Gruppen sind angelegt.",
          "Mitarbeitende können sich anmelden.",
          "Aktivitäten sind für Planung oder spontane Nutzung vorbereitet.",
        ],
        screenshot:
          "Datenverwaltung mit Kinder, Personal, Räume, Aktivitäten und Gruppen",
      },
    ],
  },
  {
    id: "alltag",
    title: "Im OGS-Alltag",
    description:
      "Suchen, beaufsichtigen, spontane Aktivitäten starten und den Tagesbetrieb im Blick behalten.",
    icon: HeartHandshake,
    tone: "blue",
    sections: [
      {
        id: "kindersuche",
        kicker: "Schnell finden",
        title: "Kinder suchen",
        body: "Die Kindersuche ist der schnellste Weg, ein Kind im Alltag zu finden und den aktuellen Status zu prüfen.",
        steps: [
          "`Kindersuche` öffnen.",
          "Nach dem Namen des Kindes suchen.",
          "Bei Bedarf Filter nutzen.",
          "Kind öffnen, um Details zu sehen.",
        ],
        columns: [
          {
            title: "Typische Informationen",
            items: [
              "Anwesenheitsstatus",
              "Raum oder Aktivität",
              "Gruppe und Klasse",
              "Ankunftszeit und Abholzeit",
              "Hinweise wie krank oder entschuldigt",
            ],
          },
        ],
        screenshot: "Kindersuche mit Suchfeld, Filtern und Kindkarte",
      },
      {
        id: "aufsicht",
        kicker: "Laufende Betreuung",
        title: "Aktuelle Aufsicht nutzen",
        body: "In der aktuellen Aufsicht wird dokumentiert, welche Kinder erwartet, anwesend oder abwesend sind.",
        steps: [
          "Aktuelle Aufsicht öffnen.",
          "Bereich `Jetzt geplant` prüfen.",
          "Raum oder laufende Aktivität auswählen.",
          "Kinderliste prüfen.",
          "Bei erwarteten Kindern auf `Einchecken` klicken.",
          "Bei anwesenden Kindern auf `Raum verlassen` klicken, wenn das Kind die Aktivität verlässt.",
          "Erwartete Kinder bei Bedarf als `Entschuldigt` oder `Abwesend` markieren.",
          "Mit `Zurück auf erwartet` eine Markierung korrigieren.",
          "`Weiteren Schüler suchen...` nutzen, um ein ungeplantes Kind hinzuzufügen.",
          "Zum Abschluss auf `Beenden` klicken.",
        ],
        screenshot:
          "Laufende Aktivität mit Anwesend, Erwartet, Abwesend und Beenden",
      },
      {
        id: "spontan",
        kicker: "Flexibel bleiben",
        title: "Spontane Aktivität starten",
        body: "Eine spontane Aktivität ist sinnvoll, wenn kurzfristig ein Angebot entsteht, das nicht im Stundenplan vorbereitet wurde.",
        steps: [
          "Aktuelle Aufsicht öffnen.",
          "Auf `Spontane Aktivität starten` klicken.",
          "Bei `Aktivität` einen Namen eingeben oder eine Aktivität auswählen.",
          "Bei `Raum` einen freien Raum auswählen.",
          "Bei Bedarf `Weitere Betreuer` auswählen.",
          "Auf `Aktivität starten` klicken.",
          "Kinder hinzufügen.",
          "Aktivität mit `Beenden` abschließen.",
        ],
        callout: {
          title: "Raum belegt",
          body: "Ein belegter Raum kann nicht erneut für eine spontane Aktivität ausgewählt werden.",
          tone: "orange",
        },
        screenshot:
          "Dialog Spontane Aktivität mit Aktivität, Raum und Weitere Betreuer",
      },
      {
        id: "raeume-vertretungen",
        kicker: "Übersicht behalten",
        title: "Räume, Mitarbeiter und Vertretungen prüfen",
        body: "Diese Seiten helfen dabei, Zuständigkeiten und Belegungen im Tagesbetrieb zu sehen.",
        columns: [
          {
            title: "`Räume`",
            items: [
              "`Raum suchen...` nutzen.",
              "Nach `Gebäude` oder `Status` filtern.",
              "`Frei` oder `Belegt` prüfen.",
              "Bereich `Unterwegs` über `Zuweisen` bearbeiten.",
            ],
          },
          {
            title: "`Mitarbeiter`",
            items: [
              "Nach Namen suchen.",
              "Status prüfen.",
              "`Alle`, `Anwesend`, `Abwesend`, `Im Raum`, `Homeoffice` oder `Krank/Urlaub` nutzen.",
            ],
          },
          {
            title: "`Vertretungen`",
            items: [
              "`Fachkraft suchen...` nutzen.",
              "`Verfügbar` oder `In Vertretung` filtern.",
              "`Vertretung zuweisen` öffnen.",
              "`OGS-Gruppe` und `Anzahl der Tage` wählen.",
              "Mit `Beenden` abschließen.",
            ],
          },
        ],
        screenshot:
          "Räume Übersicht, Vertretung zuweisen und aktive Vertretungen",
      },
      {
        id: "zeiterfassung",
        kicker: "Arbeitszeit",
        title: "Zeiterfassung nutzen",
        body: "Die Zeiterfassung dokumentiert Arbeitszeit, Pausen, Homeoffice und Abwesenheiten.",
        steps: [
          "`Zeiterfassung` öffnen.",
          "`In der OGS` oder `Homeoffice` wählen.",
          "Auf `Einstempeln` klicken.",
          "Bei Bedarf Pause starten.",
          "Pause beenden.",
          "Am Ende auf `Ausstempeln` klicken.",
          "Für Krankheit oder Urlaub `Abwesenheit melden` öffnen.",
          "`Art der Abwesenheit` und Zeitraum wählen.",
          "Auf `Speichern` klicken.",
        ],
        callout: {
          title: "Hinweise prüfen",
          body: "Wenn Hinweise zu langer Arbeitszeit, Pausen oder fehlenden Zeiten erscheinen, sollten diese direkt geprüft werden.",
          tone: "red",
        },
        screenshot:
          "Zeiterfassung mit Einstempeln, Pause, Ausstempeln und Abwesenheit melden",
      },
    ],
  },
  {
    id: "planung",
    title: "Planung",
    description:
      "Stundenplan, Termine, Serien und geplante Aktivitäten vorbereiten.",
    icon: CalendarDays,
    tone: "orange",
    sections: [
      {
        id: "stundenplan",
        kicker: "Wochenplanung",
        title: "Stundenplan nutzen",
        body: "Der Stundenplan dient zur Planung von Betreuungszeiten, Aktivitäten, Personal und erwarteten Kindern.",
        steps: [
          "`Stundenplan` öffnen.",
          "Planungsperiode auswählen.",
          "Zwischen Monat, Woche, Jahr und Serien wechseln.",
          "Auf `Termin` klicken.",
          "Einmaligen oder wiederkehrenden Termin anlegen.",
          "Zeit, Raum, Kinder und Personal eintragen.",
          "Auf `Speichern` klicken.",
          "Bei Serien `Termine erzeugen` nutzen.",
          "Termin im Kalender anklicken, um Details zu öffnen.",
        ],
        columns: [
          {
            title: "Begriffe",
            items: [
              "Ein Termin ist ein konkreter Eintrag im Kalender.",
              "Eine Serie ist ein wiederkehrender Termin.",
              "Eine Planungsperiode ist der Zeitraum, in dem geplant wird.",
            ],
          },
          {
            title: "Planungsoptionen",
            items: [
              "`Lücken füllen` legt fehlende Termine aus Serien an.",
              "`Woche neu berechnen` berechnet die gewählte Woche neu.",
            ],
          },
        ],
        screenshot:
          "Stundenplan mit Termin, Serien, Lücken füllen und Woche neu berechnen",
      },
      {
        id: "geplant-starten",
        kicker: "Vom Plan in den Alltag",
        title: "Geplante Aktivität starten",
        body: "Eine geplante Aktivität kann aus dem Stundenplan heraus gestartet werden. Danach wird sie in der aktuellen Aufsicht weitergeführt.",
        steps: [
          "`Stundenplan` öffnen.",
          "Geplanten Termin öffnen.",
          "`Details`, `Personal` und `Kinder` prüfen.",
          "Auf `Jetzt starten` klicken.",
          "Danach die Kinderliste in der aktuellen Aufsicht nutzen.",
          "Aktivität mit `Beenden` abschließen.",
        ],
        screenshot:
          "Termindetail mit Details, Personal, Kinder und Jetzt starten",
      },
    ],
  },
  {
    id: "kinderakten",
    title: "Kinderakten und Verläufe",
    description:
      "Stammdaten, Erziehungsberechtigte, Betreuungszeiten und Historien pflegen.",
    icon: Users,
    tone: "purple",
    sections: [
      {
        id: "kinderdetail",
        kicker: "Akte öffnen",
        title: "Kinderdetail nutzen",
        body: "Das Kinderdetail bündelt die wichtigsten Informationen zu einem Kind.",
        columns: [
          {
            title: "Tabs",
            items: [
              "`Stammdaten`",
              "`Erziehungsberechtigte`",
              "`Betreuungszeiten`",
              "`Historie`",
            ],
          },
          {
            title: "Verläufe",
            items: [
              "`Anwesenheits-Historie`",
              "`Anwesenheitsprotokoll`",
              "`Feedbackhistorie`",
              "`Mensaverlauf`",
            ],
          },
        ],
        screenshot:
          "Kinderdetail mit Stammdaten, Erziehungsberechtigte, Betreuungszeiten und Historie",
      },
      {
        id: "zeiten",
        kicker: "Regelmäßige Zeiten",
        title: "Ankunftsplan und Abholplan pflegen",
        body: "Regelmäßige Zeiten helfen dabei, erwartete Kinder und Abholzeiten im Alltag richtig zu sehen.",
        steps: [
          "Kinderdetail öffnen.",
          "`Betreuungszeiten` öffnen.",
          "Im Ankunftsbereich auf `Bearbeiten` klicken.",
          "Uhrzeiten im Format `HH:MM` eintragen.",
          "Bei Bedarf `Besonderheiten...` ergänzen.",
          "Im Abholbereich auf `Bearbeiten` klicken.",
          "`Abholer, Besonderheiten...` ergänzen.",
          "Einzelne Tage über `Tag bearbeiten` anpassen.",
        ],
        callout: {
          title: "Einzelne Ausnahmen",
          body: "Wenn nur ein Tag anders ist, sollte der einzelne Tag bearbeitet werden. Der Wochenplan bleibt dadurch sauber.",
          tone: "blue",
        },
        screenshot:
          "Betreuungszeiten mit Ankunftsplan, Abholplan und Tag bearbeiten",
      },
    ],
  },
  {
    id: "anmeldungen",
    title: "Anmeldungen",
    description:
      "Phasen, Angebote, Formulare und eingegangene Anmeldungen verwalten.",
    icon: FileText,
    tone: "green",
    sections: [
      {
        id: "ueberblick",
        kicker: "Eingänge",
        title: "Anmeldungen prüfen",
        body: "Der Überblick zeigt aktive Phasen und eingegangene Anmeldungen.",
        steps: [
          "`Anmeldungen` öffnen.",
          "`Überblick` öffnen.",
          "Aktive Phasen und Eingänge prüfen.",
          "Anmeldung öffnen.",
          "Angaben prüfen.",
          "Entscheidung treffen.",
        ],
        screenshot: "Anmeldungen Überblick mit Phasen und Eingängen",
      },
      {
        id: "setup",
        kicker: "Anmeldung vorbereiten",
        title: "Phasen, Angebote und Formulare einrichten",
        body: "Eine Online-Anmeldung besteht aus Anmeldephase, Betreuungsangeboten und einem Formular.",
        columns: [
          {
            title: "`Anmeldephasen`",
            items: [
              "`Schuljahr`, `Ferienbetreuung` oder `Sonstiges` wählen.",
              "Betreuungszeitraum eintragen.",
              "Anmeldezeitraum eintragen.",
              "Formular zuweisen.",
            ],
          },
          {
            title: "`Betreuungsangebote`",
            items: [
              "Name und Beschreibung pflegen.",
              "Wochentage wählen.",
              "Ferienbetreuung oder Mittagessen aktivieren.",
              "Kapazität und Preis eintragen.",
            ],
          },
          {
            title: "`Anmeldeformulare`",
            items: [
              "Feste Pflichtfelder prüfen.",
              "Eigene Felder ergänzen.",
              "Feldtypen wie `Ja/Nein`, `Text`, `Datum`, `Auswahl` oder `Wochenzeiten` nutzen.",
              "Vorschau prüfen.",
            ],
          },
        ],
        screenshot: "Anmeldephasen, Betreuungsangebote und Anmeldeformulare",
      },
      {
        id: "oeffentlich",
        kicker: "Vor dem Teilen",
        title: "Öffentliche Anmeldung testen",
        body: "Vor dem Teilen des Links sollte die öffentliche Anmeldung einmal vollständig getestet werden.",
        checklist: [
          "Die richtige Phase ist aktiv.",
          "Die richtigen Angebote sind sichtbar.",
          "Das Formular enthält alle nötigen Felder.",
          "Die Bestätigung ist verständlich.",
          "Die Anmeldung erscheint danach im Überblick.",
        ],
        screenshot:
          "Öffentliche Anmeldung mit Auswahl, Formular und Bestätigung",
      },
    ],
  },
  {
    id: "weitere-funktionen",
    title: "Weitere Funktionen und Einstellungen",
    description:
      "Feedback, Rollen, Geräte, Einstellungen und das eigene Profil.",
    icon: Settings,
    tone: "gray",
    sections: [
      {
        id: "feedback",
        kicker: "Rückmeldungen",
        title: "Feedback nutzen",
        body: "Im Feedback-Bereich können Wünsche und Probleme gesammelt, sortiert und kommentiert werden.",
        steps: [
          "`Feedback` öffnen.",
          "`Feedback durchsuchen...` nutzen.",
          "Bei `Sortierung` zwischen `Beliebteste` und `Neueste` wählen.",
          "Bei Bedarf nach `Status` filtern.",
          "Auf `Neuer Beitrag` klicken.",
          "Titel und Beschreibung eintragen.",
          "Beitrag speichern.",
        ],
        screenshot:
          "Feedback Übersicht mit Neuer Beitrag, Sortierung und Statusfilter",
      },
      {
        id: "datenverwaltung",
        kicker: "Nachschlagewerk",
        title: "Datenverwaltung und Einstellungen verstehen",
        body: "Nicht jeder Bereich wird täglich gebraucht. Rollen, Berechtigungen, Geräte und Einstellungen sollten bewusst geändert werden.",
        columns: [
          {
            title: "`Rollen` und `Berechtigungen`",
            items: [
              "`Name`, `Beschreibung`, `Systemrolle` und `Berechtigungen` prüfen.",
              "Rollen nur ändern, wenn die Auswirkung klar ist.",
            ],
          },
          {
            title: "`Geräte`",
            items: [
              "`Geräte-ID`, `Gerätetyp` und `Gerätename` prüfen.",
              "`Status`, `Verbindung`, `Letzter Standort` und `API-Schlüssel` ansehen.",
            ],
          },
          {
            title: "`Einstellungen` und `Profil`",
            items: [
              "Bereiche `Betrieb`, `Datenschutz`, `Geräte`, `Anmeldung`, `System`, `Allgemein` prüfen.",
              "Im `Profil` Vorname, Nachname, Profilbild und Passwort ändern.",
            ],
          },
        ],
        screenshot: "Einstellungen, Profil, Rollen und Geräte",
      },
    ],
  },
];

export const nfcChapters: readonly GuideChapter[] = [
  ...standardChapters,
  {
    id: "nfc-vorbereiten",
    title: "NFC vorbereiten",
    description:
      "Zusätzliche Schritte für Einrichtungen mit NFC-Tablets oder Armbändern.",
    icon: TabletSmartphone,
    tone: "blue",
    sections: [
      {
        id: "kinder-nfc",
        kicker: "Vor der Armband-Zuweisung",
        title: "Kinder zuerst in moto anlegen",
        body: "Ein NFC-Armband kann erst sinnvoll zugewiesen werden, wenn das Kind in moto vorhanden und auffindbar ist.",
        steps: [
          "Kinder manuell anlegen oder per Excel oder CSV importieren.",
          "Vorname, Nachname und Klasse prüfen.",
          "OGS Gruppe prüfen.",
          "Kind in der `Kindersuche` suchen.",
          "Erst danach die Armband-Zuweisung vorbereiten.",
        ],
        callout: {
          title: "Keine Namen auf dem Armband",
          body: "NFC-Armbänder enthalten keine Namen und keine persönlichen Daten.",
          tone: "green",
        },
        screenshot: "Kindersuche mit gefundenem Kind vor NFC-Zuweisung",
      },
      {
        id: "nfc-benennung",
        kicker: "Klarheit auf dem Tablet",
        title: "Räume, Gruppen und Aktivitäten eindeutig benennen",
        body: "Kurze und klare Namen helfen, damit auf Tablets schnell die richtige Auswahl getroffen wird.",
        columns: [
          {
            title: "Gute Beispiele",
            items: [
              "Gruppenraum Blau",
              "Mensa",
              "Turnhalle",
              "Schulhof",
              "Freispiel",
              "Hausaufgaben",
            ],
          },
          {
            title: "Vermeiden",
            items: ["Raum 1", "Neu", "Test", "AG Montag 2", "Gruppe alt"],
          },
        ],
        screenshot: "Datenverwaltung mit klar benannten Räumen und Aktivitäten",
      },
    ],
  },
  {
    id: "nfc-tablets",
    title: "Aktivitäten und Geräte für NFC",
    description:
      "Aktivitäten, Geräte und Prüfungen vor dem ersten Tablet-Einsatz.",
    icon: MonitorSmartphone,
    tone: "purple",
    sections: [
      {
        id: "nfc-aktivitaeten",
        kicker: "Tablet-Auswahl",
        title: "Aktivitäten für Tablets vorbereiten",
        body: "Aktivitäten sollten so benannt sein, dass sie im Alltag sofort verstanden werden.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "`Aktivitäten` öffnen.",
          "Benötigte Aktivitäten anlegen.",
          "Kurze Namen verwenden.",
          "Passende Kategorie wählen.",
          "`Maximale Teilnehmer` eintragen.",
          "Aktivität speichern.",
        ],
        checklist: [
          "Alle benötigten Aktivitäten sind angelegt.",
          "Namen sind kurz und verständlich.",
          "Räume und Gruppen sind ebenfalls sauber benannt.",
        ],
        screenshot:
          "Aktivitätsformular mit Name, Kategorie und Maximale Teilnehmer",
      },
      {
        id: "nfc-geraete",
        kicker: "Technik prüfen",
        title: "Geräte in moto prüfen",
        body: "Der Bereich Geräte zeigt, welche Terminals oder Tablets in moto bekannt sind.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "`Geräte` öffnen.",
          "`Geräte-ID`, `Gerätetyp` und `Gerätename` prüfen.",
          "`Status` prüfen.",
          "`Verbindung` und `Letzter Standort` ansehen, falls sichtbar.",
        ],
        callout: {
          title: "Statuswerte",
          body: "Statuswerte können unter anderem `Aktiv`, `Inaktiv`, `Wartung` oder `Offline` sein.",
          tone: "gray",
        },
        screenshot: "Gerätedetail mit Status, Verbindung und API-Schlüssel",
      },
      {
        id: "nfc-check",
        kicker: "Erster Einsatz",
        title: "Vor dem Tablet-Einsatz prüfen",
        body: "Vor dem ersten Einsatz lohnt sich eine kurze Prüfung in moto.",
        checklist: [
          "Alle Kinder sind angelegt.",
          "Alle benötigten Gruppen sind angelegt.",
          "Alle wichtigen Räume sind angelegt.",
          "Die benötigten Aktivitäten sind angelegt.",
          "Mitarbeitende können sich anmelden.",
          "Die Kindersuche findet die Kinder.",
          "Geräte werden mit passendem Status angezeigt.",
        ],
        screenshot:
          "NFC-Prüfliste mit Kinder, Gruppen, Räume, Aktivitäten und Geräte",
      },
    ],
  },
];
