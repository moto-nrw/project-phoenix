import {
  Activity,
  Building2,
  CalendarDays,
  ClipboardCheck,
  ClipboardList,
  Clock3,
  Database,
  Eye,
  FileText,
  KeyRound,
  LayoutDashboard,
  MessageSquare,
  Repeat,
  Search,
  TabletSmartphone,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type GuideTone = "blue" | "green" | "orange" | "red" | "purple" | "gray";

/** A short highlighted note attached to a single step. */
export interface GuideCallout {
  readonly title: string;
  readonly body: string;
  readonly tone?: GuideTone;
}

/** One documentation card: a single concrete task or sidebar area. */
export interface GuideStep {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  /** Ordered actions. Optional: a card may be checklist-only instead. */
  readonly steps?: readonly string[];
  /** Verification items rendered as a checklist block. */
  readonly checklist?: readonly string[];
  /** Highlighted hint shown under the steps. */
  readonly callout?: GuideCallout;
  /** Caption / alt text describing the supporting screenshot. */
  readonly screenshot: string;
  /** Path under /public to the screenshot image. Falls back to a placeholder. */
  readonly image?: string;
  /** Shown instead of a number badge on function-reference pages. */
  readonly icon?: LucideIcon;
}

/** A titled group of steps with its own accent tone. */
export interface GuideChapter {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly icon: LucideIcon;
  readonly tone: GuideTone;
  readonly steps: readonly GuideStep[];
}

export interface GuideEntryPoint {
  readonly href: string;
  readonly title: string;
  readonly body: string;
  readonly icon: LucideIcon;
  readonly points: readonly string[];
}

export const guideEntryPoints: readonly GuideEntryPoint[] = [
  {
    href: "/anleitung/ersteinrichtung",
    title: "Ersteinrichtung",
    body: "Schritt für Schritt vom leeren System bis zum ersten echten Betreuungstag.",
    icon: ClipboardCheck,
    points: [
      "Zugang, Rollen und Stammdaten",
      "Mitarbeitende, Räume, Gruppen, Kinder",
      "Testlauf und Go-live-Checkliste",
    ],
  },
  {
    href: "/anleitung/funktionen",
    title: "Die App im Alltag",
    body: "Jeder Punkt der Seitenleiste verständlich erklärt: was er macht und wie man ihn nutzt.",
    icon: LayoutDashboard,
    points: [
      "Kindersuche, Aufsicht, Räume, Mitarbeiter",
      "Vertretungen, Stundenplan, Zeiterfassung",
      "Datenverwaltung, Anmeldungen, Feedback",
    ],
  },
  {
    href: "/anleitung/nfc",
    title: "NFC & Tablets",
    body: "Zusätzliche Schritte nur für Einrichtungen mit Tablets oder NFC-Armbändern.",
    icon: TabletSmartphone,
    points: [
      "Kinder vor der Armband-Zuweisung prüfen",
      "Räume und Aktivitäten tablet-tauglich benennen",
      "Geräte prüfen und ersten Einsatz absichern",
    ],
  },
];

/**
 * Ersteinrichtung: dependency-ordered chapters. The order is intentional:
 * staff before groups (group leader), rooms before groups (groups assign a
 * room), rooms + staff before activities, groups before the child import (so
 * the import can map children to groups), data before the go-live test.
 */
export const setupChapters: readonly GuideChapter[] = [
  {
    id: "zugang-und-team",
    title: "Zugang und Team",
    description:
      "Zuerst der eigene Admin-Zugang, dann das Team. Danach kann sich jede Person mit ihrer eigenen E-Mail-Adresse anmelden.",
    icon: KeyRound,
    tone: "green",
    steps: [
      {
        id: "konto-erstellen",
        title: "Konto erstellen und anmelden",
        summary:
          "Mit der Einladungs-Mail das Admin-Konto erstellen und zum ersten Mal einloggen.",
        steps: [
          "Einladungs-Mail von moto öffnen.",
          "Dem Link aus der E-Mail folgen.",
          "Konto erstellen und ein starkes Passwort setzen.",
          "moto öffnen und `E-Mail-Adresse` und `Passwort` eingeben.",
          "Auf `Anmelden` klicken.",
          "Prüfen, ob nach dem Login der Name der richtigen Einrichtung erscheint.",
        ],
        callout: {
          title: "Passwort vergessen",
          body: "Wenn das Passwort nicht bekannt ist, auf `Passwort vergessen?` klicken und den Anweisungen folgen.",
          tone: "gray",
        },
        screenshot: "Loginseite mit E-Mail-Adresse, Passwort und Anmelden.",
        image: "/anleitung/screens/konto-erstellen.png",
      },
      {
        id: "mitarbeitende-anlegen",
        title: "Mitarbeitende anlegen und einladen",
        summary:
          "Das Team anlegen und jeder Person gleich die passende Rolle geben, damit sich alle mit ihrer eigenen E-Mail-Adresse anmelden können.",
        steps: [
          "`Datenverwaltung` öffnen und `Personal` wählen.",
          "Neue Person anlegen.",
          "`Vorname` und `Nachname` eintragen.",
          "`E-Mail` eintragen. Diese Adresse wird für die Anmeldung genutzt.",
          "Passende `Rolle` wählen. Admin-Rechte nur für Personen, die Stammdaten oder Einstellungen ändern sollen.",
          "Speichern und die Person zum Login auffordern.",
        ],
        screenshot: "Personalformular mit Vorname, Nachname, E-Mail und Rolle.",
        image: "/anleitung/screens/mitarbeitende-anlegen.png",
      },
    ],
  },
  {
    id: "struktur-anlegen",
    title: "Ihre OGS-Struktur anlegen",
    description:
      "Räume, Gruppen und Aktivitäten bilden das Gerüst für Aufsicht, Planung und Suche. Klare Namen helfen später im Alltag.",
    icon: Building2,
    tone: "blue",
    steps: [
      {
        id: "raeume-anlegen",
        title: "Räume anlegen",
        summary:
          "Alle Räume anlegen, die für Aufsicht, Stundenplan und Gruppen gebraucht werden.",
        steps: [
          "`Datenverwaltung` öffnen und `Räume` wählen.",
          "Neuen Raum anlegen.",
          "`Raumname`, `Kategorie`, `Gebäude`, `Etage` und `Farbe` pflegen.",
          "Kurze, eindeutige Namen nutzen, zum Beispiel `Mensa`, `Turnhalle`, `Gruppenraum Blau`.",
          "Speichern und die Liste auf Dubletten prüfen.",
        ],
        screenshot:
          "Räume-Liste in der Datenverwaltung mit Beispiel-Einträgen.",
        image: "/anleitung/screens/raeume-anlegen.png",
      },
      {
        id: "gruppen-anlegen",
        title: "Gruppen anlegen",
        summary:
          "OGS-Gruppen anlegen und je einen Raum sowie eine Gruppenleitung zuordnen.",
        steps: [
          "`Datenverwaltung` öffnen und `Gruppen` wählen.",
          "Neue Gruppe anlegen.",
          "`Gruppenname` eintragen.",
          "Wenn möglich einen `Gruppenraum` zuordnen.",
          "Wenn bekannt eine oder mehrere Personen als `Gruppenleitung` wählen.",
          "Speichern.",
        ],
        screenshot: "Gruppen-Liste in der Datenverwaltung.",
        image: "/anleitung/screens/gruppen-anlegen.png",
      },
      {
        id: "aktivitaeten-anlegen",
        title: "Aktivitäten anlegen",
        summary:
          "Wiederkehrende Angebote vorbereiten, damit sie im Stundenplan und spontan verfügbar sind.",
        steps: [
          "`Datenverwaltung` öffnen und `Aktivitäten` wählen.",
          "Neue Aktivität anlegen.",
          "`Name` kurz und verständlich eintragen.",
          "`Kategorie` wählen.",
          "`Maximale Teilnehmer` eintragen, wenn eine Grenze gilt.",
          "Speichern.",
        ],
        screenshot:
          "Aktivitätsformular mit Name, Kategorie und maximale Teilnehmer.",
        image: "/anleitung/screens/aktivitaeten-anlegen.png",
      },
    ],
  },
  {
    id: "kinder-und-zeiten",
    title: "Kinder und Betreuungszeiten",
    description:
      "Kinder anlegen, als Liste importiert oder einzeln, und ihre Ankunfts- und Abholzeiten pflegen.",
    icon: Users,
    tone: "orange",
    steps: [
      {
        id: "kinder-importieren",
        title: "Kinder aus einer Liste importieren (optional)",
        summary:
          "Nur nötig, wenn Ihnen die Kinderdaten bereits als Excel- oder CSV-Liste vorliegen, dann legen Sie alle Kinder auf einmal an. Ohne solche Liste überspringen Sie diesen Schritt und legen die Kinder im nächsten Schritt einzeln an.",
        steps: [
          "`Datenverwaltung` öffnen und `Kinder` wählen.",
          "Auf `Importieren` klicken.",
          "`Excel (.xlsx)` oder `CSV (Komma-getrennt)` wählen.",
          "`Vorlage herunterladen` und die Pflichtfelder vollständig ausfüllen.",
          "Datei hochladen und die `Datenvorschau` Zeile für Zeile prüfen.",
          "Fehler in der Datei beheben und erneut hochladen.",
          "Erst wenn die Vorschau stimmt, auf `Schüler importieren` klicken.",
          "Eine Stichprobe in der `Kindersuche` prüfen.",
        ],
        callout: {
          title: "Geburtstage im Import",
          body: "Geburtstage können als `JJJJ-MM-TT`, `TT.MM.JJJJ` oder `TT.MM.JJ` eingetragen werden.",
          tone: "blue",
        },
        screenshot:
          "Kinderimport mit Vorlage herunterladen, Datenvorschau und Schüler importieren.",
        image: "/anleitung/screens/kinder-importieren.png",
      },
      {
        id: "kind-manuell-anlegen",
        title: "Einzelnes Kind manuell anlegen",
        summary:
          "Der Standardweg ohne Importliste, auch um einzelne Kinder jederzeit nachzutragen.",
        steps: [
          "`Datenverwaltung` öffnen und `Kinder` wählen.",
          "Neues Kind anlegen.",
          "`Vorname`, `Nachname`, `Klasse` und `OGS Gruppe` eintragen.",
          "`Erziehungsberechtigte` und Abholinformationen ergänzen.",
          "`Datenschutzeinwilligung erteilt` nur aktivieren, wenn die Einwilligung vorliegt.",
          "Speichern und das Kind in der `Kindersuche` suchen.",
        ],
        screenshot:
          "Kinderformular mit Stammdaten, Gruppe und Erziehungsberechtigten.",
        image: "/anleitung/screens/kind-manuell-anlegen.png",
      },
      {
        id: "betreuungszeiten-pflegen",
        title: "Ankunfts- und Abholzeiten pflegen",
        summary:
          "Regelmäßige Zeiten und Ausnahmen eintragen, damit erwartete Kinder und Abholzeiten stimmen.",
        steps: [
          "Kind über `Kindersuche` öffnen.",
          "`Betreuungszeiten` öffnen.",
          "Im Ankunftsbereich auf `Bearbeiten` klicken und Zeiten im Format `HH:MM` eintragen.",
          "Im Abholbereich auf `Bearbeiten` klicken und Abholer sowie Hinweise eintragen.",
          "Für einzelne Ausnahmen `Tag bearbeiten` nutzen.",
          "Speichern.",
        ],
        screenshot:
          "Betreuungszeiten mit Ankunftsplan, Abholplan und Tag bearbeiten.",
        image: "/anleitung/screens/betreuungszeiten-pflegen.png",
      },
    ],
  },
  {
    id: "testlauf",
    title: "Testlauf vor dem Start",
    description:
      "Eine letzte Prüfung, die die meisten Supportfälle am ersten Betreuungstag verhindert.",
    icon: ClipboardCheck,
    tone: "purple",
    steps: [
      {
        id: "go-live-check",
        title: "Go-live-Check vor dem ersten echten Tag",
        summary:
          "Arbeiten Sie diese Punkte mit einem Admin-Konto ab. Erst wenn alles stimmt, ist die Einrichtung startklar.",
        checklist: [
          "Mit einem Admin-Konto anmelden.",
          "Drei Kinder aus verschiedenen Gruppen in der `Kindersuche` prüfen.",
          "Eine Gruppe öffnen und die Kinderzuordnung prüfen.",
          "Eine spontane Aktivität starten und wieder beenden.",
          "Ein Kind als `Entschuldigt` markieren und mit `Zurück auf erwartet` korrigieren.",
          "Das Team kurz einweisen: Suche, Aufsicht, Räume, Zeiterfassung, Feedback.",
        ],
        screenshot:
          "Go-live-Übersicht mit Kindersuche, aktueller Aufsicht und Räumen.",
        image: "/anleitung/screens/go-live-check.png",
      },
    ],
  },
];

/**
 * "Die App im Alltag": one card per sidebar item, grouped into chapters but
 * kept in the real top-to-bottom order of the staff sidebar, so the
 * documentation still maps 1:1 to what users see.
 */
export const appChapters: readonly GuideChapter[] = [
  {
    id: "alltag-und-aufsicht",
    title: "Alltag und Aufsicht",
    description:
      "Die Bereiche für die laufende Betreuung: Kinder finden, eigene Gruppen sehen und die Aufsicht in Echtzeit steuern.",
    icon: Eye,
    tone: "green",
    steps: [
      {
        id: "kindersuche",
        title: "Kindersuche",
        icon: Search,
        summary:
          "Findet jedes Kind und zeigt seinen aktuellen Status und Aufenthaltsort.",
        steps: [
          "`Kindersuche` öffnen.",
          "Namen oder Namensbestandteil in das Suchfeld eingeben.",
          "Bei Bedarf nach Gruppe, Klasse oder Status filtern.",
          "Ein Kind öffnen, um Details, Raum und Zeiten zu sehen.",
        ],
        screenshot: "Kindersuche mit Suchfeld, Filtern und Status-Badges.",
        image: "/anleitung/screens/kindersuche.png",
      },
      {
        id: "meine-gruppen",
        title: "Meine Gruppen",
        icon: Users,
        summary:
          "Schneller Zugriff auf die Gruppen, für die du als Aufsicht eingeteilt bist, mit aktueller Anwesenheitszahl.",
        steps: [
          "In der Seitenleiste `Meine Gruppen` aufklappen.",
          "Die gewünschte Gruppe wählen.",
          "Anwesenheit der Gruppe ansehen und Kinder bearbeiten.",
        ],
        screenshot:
          "Seitenleiste mit aufgeklappten eigenen Gruppen und Anwesenheitszahl.",
        image: "/anleitung/screens/meine-gruppen.png",
      },
      {
        id: "aktuelle-aufsicht",
        title: "Aktuelle Aufsicht",
        icon: Eye,
        summary:
          "Steuert die laufende Betreuung in Echtzeit: einchecken, entschuldigen, korrigieren und spontane Aktivitäten starten.",
        steps: [
          "`Aktuelle Aufsicht` öffnen und Raum oder Aktivität wählen.",
          "Bereich `Erwartet` prüfen.",
          "Bei anwesendem Kind auf `Einchecken` klicken.",
          "Bei bekannter Abwesenheit `Entschuldigt` wählen.",
          "Falsche Markierung mit `Zurück auf erwartet` korrigieren.",
          "Ungeplantes Kind über `Weiteren Schüler suchen...` hinzufügen.",
          "Für ein neues Angebot `Spontane Aktivität starten`.",
        ],
        screenshot:
          "Laufende Aufsicht mit Anwesend, Erwartet und Spontane Aktivität.",
        image: "/anleitung/screens/aktuelle-aufsicht.png",
      },
    ],
  },
  {
    id: "raeume-team-vertretung",
    title: "Räume, Team und Vertretung",
    description:
      "Den Überblick über Angebote, Räume und das Team behalten und kurzfristige Vertretungen organisieren.",
    icon: Building2,
    tone: "blue",
    steps: [
      {
        id: "aktivitaeten",
        title: "Aktivitäten",
        icon: Activity,
        summary:
          "Liste aller Aktivitäten mit Suche und Filter; hier legst du neue Angebote an.",
        steps: [
          "`Aktivitäten` öffnen.",
          "Nach Name, Betreuer oder Kategorie suchen oder filtern.",
          "Eine Aktivität öffnen, um sie anzusehen oder zu bearbeiten.",
          "Über die Schaltfläche zum Anlegen eine neue Aktivität erstellen.",
        ],
        screenshot: "Aktivitätenliste mit Suche, Filter und neuer Aktivität.",
        image: "/anleitung/screens/aktivitaeten.png",
      },
      {
        id: "raeume",
        title: "Räume",
        icon: Building2,
        summary:
          "Zeigt, welche Räume frei oder belegt sind und welche Kinder noch keinem Raum zugeordnet sind.",
        steps: [
          "`Räume` öffnen.",
          "Bei Bedarf `Raum suchen...` nutzen oder nach `Gebäude` und `Status` filtern.",
          "Eine Raumkarte öffnen, um die Kinderliste zu sehen.",
          "Bereich `Unterwegs` prüfen und ein Kind ohne Raum über `Zuweisen` zuordnen.",
        ],
        screenshot:
          "Räume-Übersicht mit Statusfilter, Frei/Belegt und Unterwegs.",
        image: "/anleitung/screens/raeume.png",
      },
      {
        id: "mitarbeiter",
        title: "Mitarbeiter",
        icon: ClipboardList,
        summary:
          "Zeigt den Status des Teams: wer anwesend ist, in welchem Raum oder im Homeoffice.",
        steps: [
          "`Mitarbeiter` öffnen.",
          "Nach Name suchen.",
          "Nach Status filtern, zum Beispiel `Anwesend` oder `Homeoffice`.",
          "Eine Person öffnen, um Details zu sehen.",
        ],
        screenshot:
          "Mitarbeiterliste mit Status-Badges und aktiven Aufsichten.",
        image: "/anleitung/screens/mitarbeiter.png",
      },
      {
        id: "vertretungen",
        title: "Vertretungen",
        icon: Repeat,
        summary:
          "Weist verfügbare Fachkräfte einer OGS-Gruppe für einen Zeitraum zu (nur für Admins).",
        steps: [
          "`Vertretungen` öffnen.",
          "Filter `Verfügbar` wählen und eine Person auswählen.",
          "`Vertretung zuweisen` öffnen.",
          "`OGS-Gruppe` und `Anzahl der Tage` festlegen.",
          "Mit `Zuweisen` speichern.",
          "Nach Ende im aktiven Eintrag auf `Beenden` klicken.",
        ],
        screenshot:
          "Vertretungen mit verfügbaren Fachkräften und Dialog Vertretung zuweisen.",
        image: "/anleitung/screens/vertretungen.png",
      },
    ],
  },
  {
    id: "planung-und-zeit",
    title: "Planung und Zeit",
    description:
      "Angebote im Voraus planen und die eigene Arbeitszeit erfassen.",
    icon: CalendarDays,
    tone: "orange",
    steps: [
      {
        id: "stundenplan",
        title: "Stundenplan",
        icon: CalendarDays,
        summary:
          "Plant Termine, Serien, Räume, Personal und erwartete Kinder im Voraus (nur für Admins).",
        steps: [
          "`Stundenplan` öffnen und die Planungsperiode wählen.",
          "Auf `Termin` klicken und Zeit, Raum, Personal und Kinder eintragen.",
          "Für wiederkehrende Angebote eine Serie anlegen und `Termine erzeugen`.",
          "Einen Termin mit `Jetzt starten` beginnen.",
        ],
        screenshot:
          "Stundenplan-Kalender mit Termin, Serien und Jetzt starten.",
        image: "/anleitung/screens/stundenplan.png",
      },
      {
        id: "zeiterfassung",
        title: "Zeiterfassung",
        icon: Clock3,
        summary: "Erfasst Arbeitszeit, Pausen und Abwesenheiten.",
        steps: [
          "`Zeiterfassung` öffnen.",
          "`In der OGS` oder `Homeoffice` wählen.",
          "Mit `Einstempeln` beginnen und am Ende `Ausstempeln`.",
          "Pausen starten und beenden.",
          "Für Krankheit oder Urlaub `Abwesenheit melden` mit `Art der Abwesenheit` und Zeitraum.",
        ],
        screenshot:
          "Zeiterfassung mit Einstempeln, Pause, Ausstempeln und Abwesenheit melden.",
        image: "/anleitung/screens/zeiterfassung.png",
      },
    ],
  },
  {
    id: "verwaltung-und-austausch",
    title: "Verwaltung und Austausch",
    description:
      "Die Admin-Bereiche für Stammdaten und Anmeldungen sowie der Kanal für Rückmeldungen.",
    icon: Database,
    tone: "purple",
    steps: [
      {
        id: "datenverwaltung",
        title: "Datenverwaltung",
        icon: Database,
        summary:
          "Der Admin-Bereich für alle Stammdaten: Kinder, Personal, Räume, Aktivitäten, Gruppen, Rollen, Geräte und Berechtigungen.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "Den gewünschten Bereich wählen: `Kinder`, `Personal`, `Räume`, `Aktivitäten`, `Gruppen`, `Rollen`, `Geräte` oder `Berechtigungen`.",
          "Einträge anlegen, bearbeiten oder prüfen.",
        ],
        screenshot: "Datenverwaltung mit allen Bereichen und Eintragszahlen.",
        image: "/anleitung/screens/datenverwaltung.png",
      },
      {
        id: "anmeldungen",
        title: "Anmeldungen",
        icon: FileText,
        summary:
          "Verwaltet Online-Anmeldungen: Phasen, Angebote, Formulare und eingegangene Anmeldungen (nur für Admins).",
        steps: [
          "`Anmeldungen` öffnen.",
          "Im `Überblick` eingegangene Anmeldungen prüfen.",
          "Eine Anmeldung öffnen und Kind, Angebot und Formularangaben prüfen.",
          "`Genehmigen`, `Ablehnen` oder `Warteliste` wählen.",
          "Unter `Anmeldephasen`, `Betreuungsangebote` und `Anmeldeformulare` die öffentliche Anmeldung einrichten.",
        ],
        screenshot:
          "Anmeldungen-Überblick mit Eingangsliste und Entscheidungsoptionen.",
        image: "/anleitung/screens/anmeldungen.png",
      },
      {
        id: "feedback",
        title: "Feedback",
        icon: MessageSquare,
        summary:
          "Probleme, Wünsche und Ideen melden und über bestehende Beiträge abstimmen.",
        steps: [
          "`Feedback` öffnen.",
          "`Feedback durchsuchen...` nutzen, um Dubletten zu vermeiden.",
          "Über `Neuer Beitrag` einen klaren Titel und eine Beschreibung erfassen.",
          "Bei einem bestehenden Beitrag kommentieren oder abstimmen.",
        ],
        screenshot:
          "Feedback-Übersicht mit Suche, Neuer Beitrag und Statusfilter.",
        image: "/anleitung/screens/feedback.png",
      },
    ],
  },
];

/** NFC & Tablets: additional preparation steps, only for NFC schools. */
export const nfcChapters: readonly GuideChapter[] = [
  {
    id: "nfc-vorbereiten",
    title: "Daten und Geräte vorbereiten",
    description:
      "NFC funktioniert erst sauber, wenn Kinder, Namen und Geräte in moto stimmen.",
    icon: TabletSmartphone,
    tone: "blue",
    steps: [
      {
        id: "nfc-kinder-vorbereiten",
        title: "Kinder vor der Armband-Zuweisung prüfen",
        summary:
          "NFC funktioniert erst sauber, wenn jedes Kind in moto eindeutig vorhanden ist.",
        steps: [
          "`Kindersuche` öffnen.",
          "Kind über den Nachnamen suchen.",
          "Vorname, Nachname, Klasse und OGS-Gruppe prüfen.",
          "Dubletten ausschließen.",
          "Fehlende oder falsche Daten in der Kinderakte korrigieren.",
          "Erst danach die Armband-Zuweisung vorbereiten.",
        ],
        screenshot:
          "Kindersuche mit eindeutig gefundenem Kind vor NFC-Zuweisung.",
      },
      {
        id: "nfc-namen",
        title: "Räume, Gruppen und Aktivitäten tablet-tauglich benennen",
        summary:
          "Kurze Namen helfen, am Tablet schnell die richtige Auswahl zu treffen.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "`Räume`, `Gruppen` und `Aktivitäten` prüfen.",
          "Unklare Namen wie `Raum 1`, `Test` oder `Neu` ändern.",
          "Kurze Namen wie `Mensa`, `Schulhof`, `Hausaufgaben`, `Freispiel` verwenden.",
          "Änderungen mit den Mitarbeitenden abstimmen, die die Tablets nutzen.",
        ],
        screenshot:
          "Datenverwaltung mit klar benannten Räumen, Gruppen und Aktivitäten.",
      },
      {
        id: "nfc-geraete-pruefen",
        title: "Geräte in moto prüfen",
        summary:
          "Status, Verbindung, Geräte-ID und letzten Standort der Tablets kontrollieren.",
        steps: [
          "`Datenverwaltung` öffnen und `Geräte` wählen.",
          "`Geräte-ID`, `Gerätetyp` und `Gerätename` prüfen.",
          "`Status` sowie `Verbindung` und `Letzter Standort` ansehen.",
          "Unbekannte oder alte Geräte intern klären.",
        ],
        screenshot:
          "Gerätedetail mit Geräte-ID, Gerätetyp, Status und Verbindung.",
      },
    ],
  },
  {
    id: "nfc-erster-einsatz-kapitel",
    title: "Vor dem ersten Einsatz",
    description:
      "Eine letzte Prüfung, bevor die Tablets in den echten Betreuungsalltag gehen.",
    icon: ClipboardCheck,
    tone: "green",
    steps: [
      {
        id: "nfc-erster-einsatz",
        title: "Checkliste vor dem ersten Tablet-Einsatz",
        summary:
          "Letzte Prüfung für Kinder, Gruppen, Räume, Aktivitäten, Geräte und Teamwissen.",
        checklist: [
          "Kinder stichprobenartig in der `Kindersuche` prüfen.",
          "Gruppen, Räume und Aktivitäten prüfen.",
          "Gerätestatus prüfen.",
          "Ein Tablet mit einem Demo- oder Testablauf durchspielen.",
          "Dem Team erklären, was bei „Kind nicht gefunden“ zu tun ist.",
          "Festlegen, wer am ersten Tag Datenfehler korrigiert.",
        ],
        screenshot:
          "NFC-Prüfliste mit Kindern, Gruppen, Räumen, Aktivitäten und Geräten.",
      },
    ],
  },
];
