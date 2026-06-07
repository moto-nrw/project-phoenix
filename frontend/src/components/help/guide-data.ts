import {
  Activity,
  Building2,
  CalendarDays,
  CalendarRange,
  CircleStop,
  ClipboardCheck,
  ClipboardList,
  Clock3,
  Database,
  Eye,
  FileText,
  KeyRound,
  LayoutDashboard,
  MessageSquare,
  Nfc,
  PlayCircle,
  PlugZap,
  Repeat,
  ScanLine,
  Search,
  SlidersHorizontal,
  TabletSmartphone,
  Users,
  Wrench,
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
  /** Path under /public to the screenshot image. When omitted, the step
      renders no image (no placeholder); `screenshot` still documents intent. */
  readonly image?: string;
  /**
   * Ordered sequence of tablet screens for this step. When present it renders
   * as a captioned grid instead of the single `image` , used by the NFC manual
   * so every tablet state from the printed guide is shown, not just one.
   */
  readonly gallery?: readonly {
    readonly image: string;
    readonly caption: string;
  }[];
  readonly printCompact?: boolean;
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
    href: "/help/setup",
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
    href: "/help/features",
    title: "Die App im Alltag",
    body: "Jeder Bereich der App verständlich erklärt: was er macht und wie man ihn nutzt.",
    icon: LayoutDashboard,
    points: [
      "Kindersuche, Aufsicht, Räume, Mitarbeiter",
      "Vertretungen, Stundenplan, Zeiterfassung",
      "Datenverwaltung, Anmeldungen, Feedback",
    ],
  },
  {
    href: "/help/nfc",
    title: "NFC & Tablets",
    body: "Das komplette Tablet-Handbuch für Einrichtungen mit NFC-Armbändern - vom Aufstellen bis zur Fehlerbehebung.",
    icon: TabletSmartphone,
    points: [
      "Gerät aufstellen, anmelden und Armbänder zuweisen",
      "Aufsicht starten, Kinder ein- und auschecken",
      "Einstellungen und Fehlerbehebung",
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
        image: "/help/screens/konto-erstellen.webp",
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
          "Passende `System-Rolle` wählen. Admin-Rechte nur für Personen, die Stammdaten oder Einstellungen ändern sollen.",
          "Speichern und die Person zum Login auffordern.",
        ],
        screenshot: "Personalformular mit Vorname, Nachname, E-Mail und Rolle.",
        image: "/help/screens/mitarbeitende-anlegen.webp",
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
        summary: "Alle Räume anlegen, die von Kindern genutzt werden.",
        steps: [
          "`Datenverwaltung` öffnen und `Räume` wählen.",
          "Neuen Raum anlegen.",
          "`Raumname`, `Kategorie`, `Gebäude`, `Etage` und `Farbe` pflegen.",
          "Kurze, eindeutige Namen nutzen, zum Beispiel `Mensa`, `Turnhalle`, `Gruppenraum Blau`.",
          "Speichern.",
        ],
        screenshot:
          "Räume-Liste in der Datenverwaltung mit Beispiel-Einträgen.",
        image: "/help/screens/raeume-anlegen.webp",
      },
      {
        id: "gruppen-anlegen",
        title: "Gruppen anlegen",
        summary:
          "OGS-Gruppen anlegen, wenn Ihre Einrichtung mit festen Gruppen arbeitet, und bei Bedarf Raum sowie Gruppenleitung zuordnen.",
        steps: [
          "`Datenverwaltung` öffnen und `Gruppen` wählen.",
          "Neue Gruppe anlegen.",
          "`Gruppenname` eintragen.",
          "Wenn möglich einen `Gruppenraum` zuordnen.",
          "Wenn bekannt eine oder mehrere Personen als `Gruppenleitung` wählen.",
          "Speichern.",
        ],
        callout: {
          title: "Gruppen und Rechte",
          body: "Gruppen müssen nur angelegt werden, wenn Ihre OGS mit Gruppen arbeitet. Gruppenleitungen haben Rechte für ihre Kinder, zum Beispiel für Anmeldungen, Krankmeldungen und Stammdatenänderungen. Falls unklar ist, wer diese Rechte bekommen soll, kann das später mit dem moto-Team geklärt oder in den Einstellungen angepasst werden.",
          tone: "blue",
        },
        screenshot: "Gruppen-Liste in der Datenverwaltung.",
        image: "/help/screens/gruppen-anlegen.webp",
      },
      {
        id: "aktivitaeten-anlegen",
        title: "Aktivitäten anlegen",
        summary:
          "Wiederkehrende Angebote vorbereiten. Dieser Schritt ist vor allem für Einrichtungen relevant, die mit NFC oder Tablets arbeiten.",
        steps: [
          "`Datenverwaltung` öffnen und `Aktivitäten` wählen.",
          "Neue Aktivität anlegen.",
          "`Name` kurz und verständlich eintragen.",
          "`Kategorie` wählen.",
          "`Maximale Teilnehmer` eintragen, wenn eine Grenze gilt.",
          "Speichern.",
        ],
        callout: {
          title: "Optional ohne NFC",
          body: "Wenn Ihre Einrichtung nicht mit NFC oder Tablets arbeitet, können Sie diesen Schritt für die Ersteinrichtung zunächst überspringen und Aktivitäten später ergänzen.",
          tone: "gray",
        },
        screenshot:
          "Aktivitätsformular mit Name, Kategorie und maximale Teilnehmer.",
        image: "/help/screens/aktivitaeten-anlegen.webp",
      },
    ],
  },
  {
    id: "kinder-und-zeiten",
    title: "Kinder und Betreuungszeiten",
    description:
      "Kinder kommen automatisch ins System, wenn Familien die moto-Anmeldung nutzen. Ohne Online-Anmeldung legen Sie Kinder als Liste oder einzeln an. Erziehungsberechtigte und Betreuungszeiten erfassen Sie beim einzelnen Anlegen direkt mit.",
    icon: Users,
    tone: "orange",
    steps: [
      {
        id: "kinder-importieren",
        title: "Kinder aus einer Liste importieren (optional)",
        summary:
          "Nutzen Sie den Import, wenn Ihnen die Kinderdaten bereits als Excel- oder CSV-Liste vorliegen. So legen Sie alle Kinder auf einmal an. Ohne solche Liste überspringen Sie diesen Schritt und legen die Kinder im nächsten Schritt einzeln an.",
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
        image: "/help/screens/kinder-importieren.webp",
      },
      {
        id: "kind-manuell-anlegen",
        title: "Einzelnes Kind manuell anlegen",
        summary:
          "Der Standardweg ohne Importliste, auch um einzelne Kinder jederzeit nachzutragen. Erziehungsberechtigte und Betreuungszeiten erfassen Sie optional direkt im selben Formular.",
        steps: [
          "`Datenverwaltung` öffnen und `Kinder` wählen.",
          "Neues Kind anlegen.",
          "`Vorname`, `Nachname`, `Klasse` und `OGS Gruppe` eintragen.",
          "Optional im Abschnitt `Erziehungsberechtigte` mit `Neu anlegen` eine Bezugsperson erfassen oder mit `Vorhandene/n suchen` eine bereits angelegte Person verknüpfen.",
          "Optional im Abschnitt `Betreuungszeiten` auf `Wochenplan hinzufügen` klicken und die regelmäßigen `Ankunft`- und `Abholung`-Zeiten je Wochentag eintragen.",
          "`Datenschutzeinwilligung erteilt` nur aktivieren, wenn die Einwilligung vorliegt.",
          "Auf `Erstellen` klicken. Erziehungsberechtigte und Betreuungszeiten werden zusammen mit dem Kind gespeichert.",
          "Das Kind anschließend in der `Kindersuche` prüfen; weitere Angaben ergänzen Sie jederzeit auf der Schülerdetailseite.",
        ],
        screenshot:
          "Kinderformular mit Stammdaten sowie den Abschnitten Erziehungsberechtigte und Betreuungszeiten.",
        image: "/help/screens/kind-manuell-anlegen.webp",
      },
      {
        id: "betreuungszeiten-pflegen",
        title: "Ankunfts- und Abholzeiten pflegen",
        summary:
          "Für importierte Kinder oder zum späteren Ändern: die wöchentlich wiederkehrenden Ankunfts- und Abholzeiten auf der Schülerdetailseite eintragen und einzelne Abweichungen ergänzen. Beim einzelnen Anlegen ist das bereits im Formular möglich.",
        steps: [
          "`Datenverwaltung` öffnen und `Kinder` wählen.",
          "Das Kind in der Liste auswählen.",
          "Im rechten Detailbereich den Tab `Betreuungszeiten` öffnen.",
          "Im Bereich `Ankunftsplan & Notizen` auf `Bearbeiten` klicken und die regelmäßigen Zeiten pro Wochentag im Format `HH:MM` eintragen.",
          "Im Bereich `Abholplan & Notizen` auf `Bearbeiten` klicken und die regelmäßigen Abholzeiten, Abholer sowie Hinweise eintragen.",
          "Einzelne Abweichungen später über das Stift-Symbol am jeweiligen Wochentag pflegen.",
          "Speichern.",
        ],
        screenshot:
          "Betreuungszeiten mit Ankunftsplan, Abholplan und Tag bearbeiten.",
        image: "/help/screens/betreuungszeiten-pflegen.webp",
      },
    ],
  },
  {
    id: "testlauf",
    title: "Testlauf vor dem Start",
    description: "Die letzte Prüfung vor einem erfolgreichen ersten Tag.",
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
        image: "/help/screens/go-live-check.webp",
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
          "Bei Bedarf nach Gruppe, Stufe oder Status filtern.",
          "Ein Kind öffnen, um Details, Raum und Zeiten zu sehen.",
        ],
        screenshot: "Kindersuche mit Suchfeld, Filtern und Status-Badges.",
        image: "/help/screens/kindersuche.webp",
      },
      {
        id: "kinderdetailansicht",
        title: "Kinderdetailansicht",
        icon: FileText,
        summary:
          "In der `Kindersuche` auf die Karte eines Kindes klicken öffnet seine Detailansicht. Der Kopfbereich zeigt Name, Gruppe und den aktuellen Aufenthalt mit Uhrzeit sowie die heutige Ankunft und Abholung; darunter liegen vier Tabs.",
        steps: [
          "In der `Kindersuche` auf die Karte des Kindes klicken.",
          "Im Kopfbereich den aktuellen Aufenthalt (z. B. `OGS-Raum 1 seit 12:00 Uhr`) sowie `Heutige Ankunft` und `Heutige Abholung` ablesen.",
          "Über `Krank melden` das Kind als krank und über `Entschuldigen` als entschuldigt markieren.",
          "Tab `Stammdaten`: Name, Klasse, Gruppe, Geburtstag, Gesundheitsinformationen, Notizen, Foto und Datenschutz ansehen und über `Bearbeiten` ändern.",
          "Tab `Erziehungsberechtigte`: Bezugspersonen mit Kontaktdaten, Abholberechtigung und Notfallkontakten pflegen.",
          "Tab `Betreuungszeiten`: die wöchentlichen Ankunfts- und Abholzeiten je Wochentag verwalten.",
          "Tab `Historie`: die Anwesenheits-Historie der letzten Tage mit Raum-Details nachvollziehen.",
        ],
        callout: {
          title: "Status immer im Blick",
          body: "Der Kopfbereich zeigt unabhängig vom geöffneten Tab, wo sich das Kind gerade befindet und seit wann.",
          tone: "blue",
        },
        screenshot:
          "Kinderdetailansicht mit Statuskopf, den Aktionen Krank melden und Entschuldigen sowie den Tabs Stammdaten, Erziehungsberechtigte, Betreuungszeiten und Historie.",
        image: "/help/screens/kinderdetailansicht.webp",
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
          "Im Gruppenstatus die Krank- und Entschuldigt-Zahlen der Gruppe prüfen.",
        ],
        screenshot:
          "Seitenleiste mit aufgeklappten eigenen Gruppen und Anwesenheitszahl.",
        image: "/help/screens/meine-gruppen.webp",
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
        image: "/help/screens/aktuelle-aufsicht.webp",
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
          "Nach Name suchen oder nach `Kategorie` und `Meine Aktivitäten` filtern.",
          "Eine Aktivität öffnen, um sie anzusehen oder zu bearbeiten.",
          "Über die Schaltfläche zum Anlegen eine neue Aktivität erstellen.",
        ],
        screenshot: "Aktivitätenliste mit Suche, Filter und neuer Aktivität.",
        image: "/help/screens/aktivitaeten.webp",
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
        image: "/help/screens/raeume.webp",
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
        image: "/help/screens/mitarbeiter.webp",
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
        image: "/help/screens/vertretungen.webp",
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
          "Für wiederkehrende Angebote eine Serie anlegen. Bei Bedarf die passende `Klassengruppe` setzen, damit Phoenix später Abweichungen im erwarteten Dienstplan markieren kann.",
          "`Termine erzeugen` nutzen, damit die Serie in konkrete Termine der Planungsperiode übernommen wird.",
          "Geplante Termine erscheinen zur Startzeit in der `Aktuellen Aufsicht` unter `Jetzt geplant` und werden dort mit `Jetzt starten` begonnen.",
        ],
        callout: {
          title: "Stundenplan zuerst aktivieren",
          body: "Der Stundenplan ist anfangs ausgeblendet. Ein Admin schaltet ihn unter `Einstellungen` -> `Betrieb` mit `Stundenplan aktivieren` frei; danach erscheint er in der Seitenleiste.",
          tone: "blue",
        },
        screenshot: "Stundenplan-Kalender mit Termin und Serien.",
        image: "/help/screens/stundenplan.webp",
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
        image: "/help/screens/zeiterfassung.webp",
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
        image: "/help/screens/datenverwaltung.webp",
      },
      {
        id: "anmeldungen-einrichten",
        title: "Anmeldungen einrichten",
        icon: LayoutDashboard,
        summary:
          "Der Admin-Bereich für die Online-Anmeldung: eingegangene Anmeldungen bearbeiten und in vier Unterseiten den Ablauf einrichten - `Überblick`, `Anmeldephasen`, `Betreuungsangebote` und `Anmeldeformulare`.",
        steps: [
          "`Anmeldungen` öffnen. Du landest im `Überblick` mit allen Anmeldephasen und der Zahl der Eingänge (`Gesamt`, `Offen`, `Bestätigt`, `Abgelehnt`).",
          "Beim ersten Einrichten führt dich der Bereich `Einrichtung` (`Online-Anmeldung vorbereiten`) Schritt für Schritt durch alles Nötige. Zuerst `Online-Anmeldung aktivieren`: schaltet den Elternlink frei (in den `Einstellungen` unter `Anmeldung`).",
        ],
        callout: {
          title: "So hängt alles zusammen",
          body: "Alles hängt an der `Anmeldephase`: Sie legt den Zeitraum und das Anmeldefenster fest. `Betreuungsangebote` gehören zu einer Phase, und jede Phase nutzt ein `Anmeldeformular`. Richte deshalb in dieser Reihenfolge ein: zuerst die `Online-Anmeldung` in den Einstellungen aktivieren (sonst ist der Elternlink nicht erreichbar), dann eine Anmeldephase anlegen, danach die Betreuungsangebote, bei Bedarf ein eigenes Formular - am Ende die Elternansicht testen. Der `Überblick` enthält dafür den Bereich `Einrichtung`, der dich Schritt für Schritt führt.",
          tone: "blue",
        },
        screenshot:
          "Anmeldungen-Überblick mit Einrichtungsbereich und Einstieg in die Online-Anmeldung.",
      },
      {
        id: "anmeldungen-pruefen",
        title: "Anmeldungen prüfen",
        icon: LayoutDashboard,
        summary:
          "Eingegangene Anmeldungen öffnen, Angaben prüfen und die passende Entscheidung setzen.",
        steps: [
          "Bei einer Phase auf `Anmeldungen ansehen` klicken, um die eingegangenen Anmeldungen zu prüfen.",
          "Eine Anmeldung öffnen und Kind, gewähltes Betreuungsangebot und Formularangaben prüfen.",
          "Mit `Bestätigen`, `Warteliste` oder `Ablehnen` entscheiden; mit `Zur Prüfung` für später vormerken.",
          "Über `Elternansicht öffnen` jederzeit prüfen, was Familien gerade sehen.",
        ],
        screenshot:
          "Anmeldungen-Überblick mit Eingangsliste und Entscheidungsoptionen.",
        image: "/help/screens/anmeldungen.webp",
      },
      {
        id: "anmeldephasen",
        title: "Anmeldephasen",
        icon: CalendarRange,
        summary:
          "Eine Anmeldephase ist der Zeitraum, für den Eltern anmelden - zum Beispiel ein Schuljahr oder eine Ferienbetreuung. Sie steuert das öffentliche Anmeldefenster.",
        steps: [
          "`Anmeldephasen` öffnen und auf `Neue Anmeldephase` klicken.",
          "`Name` und `Typ` (`Schuljahr`, `Ferienbetreuung` oder `Sonstiges`) wählen.",
          "`Betreuungszeitraum` mit `Beginn` und `Ende` festlegen.",
          "`Anmeldefenster` mit `Öffnung` und `Schließung` setzen. Bleiben beide leer, ist die Anmeldung jederzeit offen.",
          "Unter `Formular` das `Basisformular` lassen oder eine eigene Vorlage wählen.",
          "`Verhalten bei voller Betreuung` festlegen und mit `Aktiv` die Phase für Eltern sichtbar machen.",
        ],
        callout: {
          title: "Das Anmeldefenster steuert die öffentliche Anmeldung",
          body: "Das `Anmeldefenster` der Phase entscheidet, wann Familien absenden können. Über das Aktionsmenü öffnest du eine Phase mit `Phase ansehen` als Elternlink oder bereitest mit `Anschlussphase erstellen` eine Folgephase vor.",
          tone: "gray",
        },
        screenshot:
          "Anmeldephase mit Betreuungszeitraum, Anmeldefenster und Formularwahl.",
        image: "/help/screens/anmeldephasen.webp",
      },
      {
        id: "betreuungsangebote",
        title: "Betreuungsangebote",
        icon: ClipboardList,
        summary:
          "Betreuungsangebote sind die Optionen, die Eltern im Formular auswählen - etwa Regelbetreuung oder ein Angebot mit Mittagessen. Jedes Angebot gehört zu einer Anmeldephase.",
        steps: [
          "`Betreuungsangebote` öffnen und oben die `Anmeldephase` wählen.",
          "Auf `Neues Betreuungsangebot` klicken.",
          "`Name`, `Beschreibung` und die möglichen `Wochentage` festlegen.",
          "Unter `Stundenplan-Vorlage` die passende Serie verknüpfen, wenn genehmigte Anmeldungen automatisch im Stundenplan erwartet werden sollen.",
          "Optional `Kapazität`, `Preis in Cent` sowie `Mittagessen` oder `Ferienbetreuung` ergänzen.",
          "`Aktiv` setzen - nur aktive Angebote sind für Eltern auswählbar.",
        ],
        callout: {
          title: "Anmeldung und Stundenplan verbinden",
          body: "Eltern wählen weiterhin nur Angebot und Tage. Die genaue Betreuung entsteht später aus genehmigter Anmeldung plus verknüpfter Stundenplan-Vorlage. Phoenix zeigt Hinweise, wenn Angebotstage, Vorlage, erwartete Ankunft oder Klassengruppe nicht zusammenpassen.",
          tone: "blue",
        },
        screenshot:
          "Betreuungsangebote einer Anmeldephase mit Tagen, Kapazität und Extras.",
        image: "/help/screens/betreuungsangebote.webp",
      },
      {
        id: "anmeldeformulare",
        title: "Anmeldeformulare",
        icon: FileText,
        summary:
          "Das Anmeldeformular bestimmt, welche Angaben Eltern machen. Das `Basisformular` ist immer vorhanden; eigene Vorlagen ergänzen nur zusätzliche Fragen.",
        steps: [
          "`Anmeldeformulare` öffnen. Das `Basisformular` fragt Elternteil, Kind, Klassenstufe und das gewünschte Betreuungsangebot ab.",
          "Nur bei zusätzlichem Bedarf über `Neue Vorlage` eine eigene Formularvorlage mit Zusatzfragen anlegen.",
          "Mit `Vorschau` prüfen, wie das Formular für Eltern aussieht.",
          "Die Vorlage wirkt erst, wenn du sie in einer `Anmeldephase` als Formular auswählst.",
        ],
        callout: {
          title: "Formular und Phase gehören zusammen",
          body: "Ein Formular wirkt nicht für sich allein: Eine Phase nutzt entweder das `Basisformular` oder eine ausgewählte Vorlage. Ohne ausdrückliche Auswahl gilt automatisch das Basisformular.",
          tone: "blue",
        },
        screenshot: "Anmeldeformulare mit Basisformular und eigenen Vorlagen.",
        image: "/help/screens/anmeldeformulare.webp",
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
        image: "/help/screens/feedback.webp",
      },
    ],
  },
];

/**
 * NFC & Tablets: the complete tablet manual, mirroring the printed
 * "Bedienungsanleitung NFC-Tablet für die OGS" chapter for chapter, with two
 * extra web-app chapters (data prep, settings) that the print version omits.
 */
export const nfcChapters: readonly GuideChapter[] = [
  {
    id: "nfc-hardware",
    title: "Ihr neues NFC-Tablet",
    description:
      "Das NFC-Tablet ist ein eigenständiges Gerät, das ausschließlich die moto-Anwendung ausführt. Wartung und Updates erfolgen vollständig aus der Ferne durch das moto-Team.",
    icon: TabletSmartphone,
    tone: "blue",
    steps: [
      {
        id: "nfc-lieferumfang",
        title: "Lieferumfang und Anschlüsse",
        summary:
          "Das Gerät ist sofort einsatzbereit. Zusätzliche Installationen, Registrierungen oder Konfigurationsschritte sind nicht nötig - Netzwerk und Software sind vorkonfiguriert.",
        steps: [
          "Lieferumfang prüfen: `1x NFC-Tablet` mit Touchscreen und NFC-Leser sowie `1x fest montiertes Netzkabel`.",
          "Vorderseite: Der `NFC-Sensor` sitzt unten am Standfuß - hier halten die Kinder später ihr Armband an.",
          "Rückseite: `LAN-Anschluss` für eine optionale Kabelverbindung und die `VESA-Halterung` für Tisch- oder Wandmontage.",
          "Das `Stromkabel` ist fest montiert und muss nur in eine Steckdose gesteckt werden.",
        ],
        callout: {
          title: "Kein herkömmliches Tablet",
          body: "Auf dem Gerät lassen sich keine weiteren Apps installieren. Das macht es besonders sicher, stabil und einfach zu bedienen. Bei Bedarf können Sie über den VESA-Standard eine Halterung Ihrer Wahl montieren.",
          tone: "gray",
        },
        screenshot:
          "Vorder- und Rückansicht des NFC-Tablets mit NFC-Sensor, LAN-Anschluss, VESA-Halterung und fest montiertem Stromkabel.",
      },
    ],
  },
  {
    id: "nfc-aufstellen",
    title: "Gerät aufstellen & einschalten",
    description:
      "Aufstellen, einstecken, fertig. Das Gerät startet bei Stromzufuhr automatisch und ist innerhalb weniger Minuten einsatzbereit - ein Einschaltknopf ist nicht vorhanden.",
    icon: PlugZap,
    tone: "blue",
    steps: [
      {
        id: "nfc-aufstellen-schritte",
        title: "Aufstellen, einstecken und starten",
        printCompact: true,
        summary:
          "Platzieren Sie das Tablet gut erreichbar für die Kinder, verbinden Sie das Netzkabel und warten Sie den Startbildschirm ab.",
        steps: [
          "`Standort wählen`: in der Nähe des Eingangsbereichs oder an einem zentralen Ort, an dem die Kinder regelmäßig vorbeikommen. Eine Steckdose muss in der Nähe sein.",
          "`Befestigen`: per VESA-Standhalterung auf dem Tisch oder per VESA-Wandhalterung. Achten Sie darauf, dass das Kabel keine Stolperfalle bildet und der NFC-Sensor für die Kinder frei zugänglich bleibt.",
          "`Einstecken`: Netzkabel mit der Steckdose verbinden. Das Gerät startet automatisch und zeigt zunächst einen Ladebildschirm.",
          "`Warten`: Nach ca. 1-2 Minuten erscheint der Startbildschirm mit `Willkommen bei moto!`. Sobald er sichtbar ist, ist das Gerät einsatzbereit.",
        ],
        callout: {
          title: "Netzwerkverbindung",
          body: "Das Tablet ist bereits mit dem WLAN Ihrer Einrichtung verbunden; alternativ ist eine LAN-Verbindung möglich. Solange die Verbindung in Ordnung ist, wird kein Symbol angezeigt. Nur bei schlechter oder fehlender Verbindung erscheint unten rechts ein rotes WLAN-Warnsymbol - prüfen Sie dann Ihre Internetverbindung oder wenden sich an den moto-Support.",
          tone: "blue",
        },
        screenshot:
          "Tablet-Startbildschirm mit moto-Logo und der Begrüßung „Willkommen bei moto!“.",
        image: "/help/screens/nfc-tablet-willkommen.webp",
      },
    ],
  },
  {
    id: "nfc-vorbereiten",
    title: "Daten und Geräte vorbereiten",
    description:
      "Bevor Sie Armbänder zuweisen, sollten Kinder, Namen und Geräte in moto stimmen. Dieser Schritt passiert im Browser, nicht am Tablet.",
    icon: ClipboardCheck,
    tone: "orange",
    steps: [
      {
        id: "nfc-kinder-vorbereiten",
        title: "Kinder vor der Armband-Zuweisung prüfen",
        summary:
          "NFC funktioniert erst sauber, wenn jedes Kind in moto eindeutig vorhanden ist. Die Kinder müssen vor der Zuweisung in der moto-App angelegt sein.",
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
        image: "/help/screens/kindersuche.webp",
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
        image: "/help/screens/datenverwaltung.webp",
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
        image: "/help/screens/nfc-geraete-pruefen.webp",
      },
    ],
  },
  {
    id: "nfc-anmelden",
    title: "Am Tablet anmelden",
    description:
      "Eine PIN stellt sicher, dass nur berechtigte Mitarbeitende auf das System zugreifen und keine Kinder. Die Anmeldung ist bewusst einfach gehalten.",
    icon: KeyRound,
    tone: "green",
    steps: [
      {
        id: "nfc-tablet-anmelden",
        title: "Mit PIN anmelden",
        summary:
          "Bevor Sie Armbänder zuweisen oder eine Aufsicht starten, melden Sie sich mit Ihrer PIN am Tablet an.",
        steps: [
          "Auf dem Startbildschirm auf den großen Button `Anmelden` tippen.",
          "Im Zahlenfeld die 4-stellige PIN Ihrer Einrichtung eingeben. Jede Ziffer wird als Punkt angezeigt, damit niemand mitlesen kann.",
          "Nach der vierten Ziffer prüft das Tablet die PIN automatisch - einen `Bestätigen`-Button gibt es nicht.",
          "Bei korrekter Eingabe öffnet sich das `Menü`. Bei falscher PIN erscheint eine Fehlermeldung; mit der `C`-Taste löschen Sie die gesamte Eingabe.",
        ],
        callout: {
          title: "Standard-PIN ändern",
          body: "Die Standard-PIN bei Auslieferung ist `1234`. Ändern Sie sie nach der ersten Anmeldung im Browser unter `Einstellungen` -> `Geräte` -> `OGS Geräte-PIN`. Bei vergessener PIN wenden Sie sich an Ihre OGS-Leitung oder den moto-Support.",
          tone: "gray",
        },
        screenshot:
          "Tablet mit PIN-Eingabe und Tastenfeld für die 4-stellige PIN.",
        gallery: [
          {
            image: "/help/screens/nfc-tablet-pin.webp",
            caption: "PIN-Eingabe: 4-stellige PIN über das Zahlenfeld.",
          },
          {
            image: "/help/screens/nfc-tablet-menue.webp",
            caption: "Menü nach erfolgreicher Anmeldung.",
          },
        ],
      },
    ],
  },
  {
    id: "nfc-armbaender-zuweisen",
    title: "Armbänder den Kindern zuweisen",
    description:
      "Vor der ersten Nutzung bekommt jedes Kind einmalig ein NFC-Armband zugewiesen. Das passiert direkt am Tablet und dauert pro Kind nur wenige Sekunden.",
    icon: Nfc,
    tone: "green",
    steps: [
      {
        id: "nfc-armband-scannen",
        title: "Armband scannen und erkennen",
        printCompact: true,
        summary:
          "Über `Armband identifizieren` lesen Sie ein Armband ein und sehen sofort seinen Status.",
        steps: [
          "Im `Menü` oben links auf `Armband identifizieren` tippen.",
          "Auf `Scan starten` tippen und das Armband des Kindes flach an den NFC-Sensor unten am Tablet halten.",
          "Nach etwa einer Sekunde erscheint `Armband wird erkannt ...` mit einem blauen Lade-Symbol.",
          "Das Tablet meldet `Armband erkannt` und zeigt, ob das Armband bereits einem Kind zugewiesen oder noch frei ist.",
        ],
        callout: {
          title: "Auf dem Armband stehen keine Daten",
          body: "Jedes Armband enthält nur eine zufällige Nummer - keine Namen oder Fotos. Erst die Zuweisung im System verbindet sie mit einem Kind. Geht ein Armband verloren, kann niemand persönliche Informationen auslesen. Die Armbänder sind wasserfest, robust und ohne Batterie für den Dauergebrauch im Schulalltag gemacht.",
          tone: "gray",
        },
        screenshot:
          "Tablet-Bildschirm „Armband identifizieren“ mit Button „Scan starten“.",
        gallery: [
          {
            image: "/help/screens/nfc-armband-scan.webp",
            caption: "„Scan starten“ und Armband an den NFC-Sensor halten.",
          },
          {
            image: "/help/screens/nfc-armband-wird-erkannt.webp",
            caption: "„Armband wird erkannt ...“ mit blauem Lade-Symbol.",
          },
          {
            image: "/help/screens/nfc-armband-erkannt-frei.webp",
            caption: "„Armband erkannt“ - hier noch keinem Kind zugewiesen.",
          },
        ],
      },
      {
        id: "nfc-armband-person-auswaehlen",
        title: "Kind auswählen",
        summary:
          "Aus der Liste das richtige Kind wählen und bei Bedarf über Klasse oder OGS-Gruppe eingrenzen.",
        steps: [
          "Bei einem freien Armband auf `Person auswählen` tippen.",
          "Es öffnet sich eine Liste aller Kinder und Mitarbeitenden, jeweils mit Name und Zugehörigkeit (Gruppe, Klasse).",
          "Die Liste über `Klasse` und `OGS-Gruppe` eingrenzen oder mit der Seitennavigation blättern.",
        ],
        callout: {
          title: "Kind nicht in der Liste?",
          body: "Prüfen Sie in der Web-`Kindersuche`, ob das Kind angelegt ist und nicht doppelt existiert. Neue Kinder zuerst in der `Datenverwaltung` anlegen, dann am Tablet zuweisen.",
          tone: "blue",
        },
        screenshot:
          "Tablet-Bildschirm „Person auswählen“ mit Filtern nach Klasse und OGS-Gruppe.",
        gallery: [
          {
            image: "/help/screens/nfc-person-auswaehlen.webp",
            caption:
              "„Person auswählen“: Liste aller Kinder und Mitarbeitenden.",
          },
          {
            image: "/help/screens/nfc-person-gefiltert.webp",
            caption: "Über Klasse und OGS-Gruppe eingrenzen.",
          },
        ],
      },
      {
        id: "nfc-armband-zuweisen",
        title: "Armband zuweisen",
        summary:
          "Das markierte Kind dauerhaft mit dem Armband verbinden und direkt das nächste Armband vorbereiten.",
        steps: [
          "Das richtige Kind antippen - die Auswahl wird farbig hervorgehoben.",
          "Mit `Armband zuweisen` bestätigen - es erscheint `Erfolgreich!` mit dem Namen des Kindes.",
          "Mit `Weiteres Armband scannen` direkt das nächste Kind vorbereiten.",
        ],
        screenshot:
          "Tablet-Bildschirm mit ausgewähltem Kind und erfolgreicher Armband-Zuweisung.",
        gallery: [
          {
            image: "/help/screens/nfc-person-ausgewaehlt.webp",
            caption: "Kind antippen - die Auswahl wird grün hervorgehoben.",
          },
          {
            image: "/help/screens/nfc-armband-erfolg.webp",
            caption: "„Erfolgreich!“ - das Armband ist dem Kind zugewiesen.",
          },
        ],
      },
      {
        id: "nfc-armband-wechseln",
        title: "Zuweisung ändern oder aufheben",
        summary:
          "Wenn ein Kind die Einrichtung verlässt oder ein Armband getauscht wird, lösen oder ändern Sie die bestehende Zuweisung.",
        steps: [
          "Erneut `Armband identifizieren` öffnen und das Armband scannen.",
          "Das Tablet zeigt unter `Aktuell zugewiesen an:` das aktuelle Kind.",
          "Für einen Wechsel auf `Anderer Person zuweisen` tippen und das neue Kind wählen.",
          "Zum vollständigen Lösen auf `Armband freigeben` (rot) tippen.",
          "Die Rückfrage mit `Ja` bestätigen - danach ist das Armband wieder frei.",
        ],
        callout: {
          title: "Gefahrlos neu zuweisen",
          body: "Ein freigegebenes Armband kann jederzeit gefahrlos einem neuen Kind zugewiesen werden, da auf dem Armband selbst keine persönlichen Daten gespeichert sind.",
          tone: "gray",
        },
        screenshot:
          "Tablet zeigt das aktuell zugewiesene Kind mit „Anderer Person zuweisen“ und „Armband freigeben“.",
        image: "/help/screens/nfc-armband-wechseln.webp",
      },
    ],
  },
  {
    id: "nfc-aufsicht-starten",
    title: "Eine Aufsicht starten",
    description:
      "Bevor Kinder ein- und auschecken können, muss eine Aufsicht gestartet werden. Sie beschreibt, welche Aktivität stattfindet, wer betreut und in welchem Raum - in drei einfachen Schritten.",
    icon: PlayCircle,
    tone: "orange",
    steps: [
      {
        id: "nfc-aufsicht-auswahl",
        title: "Aktivität wählen",
        summary:
          "Der Start beginnt mit der Frage, welche Aktivität oder welches Angebot gerade läuft.",
        steps: [
          "Im `Menü` auf den großen Button `Aufsicht starten` tippen.",
          "`Was machen wir?`: die Aktivität antippen (sie wird grün umrandet) und mit `Weiter` bestätigen.",
        ],
        callout: {
          title: "Tipp: Letzte Aufsicht wiederholen",
          body: "`Letzte Aufsicht wiederholen` im Menü übernimmt Aktivität, Team und Raum der vorherigen Aufsicht automatisch - praktisch für wiederkehrende Nachmittagsangebote.",
          tone: "blue",
        },
        screenshot:
          "Tablet-Bildschirm „Was machen wir?“ mit grün markierter Aktivität und „Weiter“.",
        gallery: [
          {
            image: "/help/screens/nfc-aktivitaet-leer.webp",
            caption: "„Was machen wir?“ - Aktivitäten zur Auswahl.",
          },
          {
            image: "/help/screens/nfc-aktivitaet-waehlen.webp",
            caption: "Aktivität antippen (grün) und mit „Weiter“ bestätigen.",
          },
        ],
      },
      {
        id: "nfc-aufsicht-team-raum",
        title: "Team und Raum wählen",
        summary:
          "Danach legen Sie fest, welche Mitarbeitenden dabei sind und in welchem Raum die Aufsicht stattfindet.",
        steps: [
          "`Wer ist dabei?`: alle beteiligten Betreuerinnen und Betreuer antippen - mindestens eine Person ist nötig. Dann `Weiter`.",
          "`Wo machen wir das?`: den Raum antippen und mit `Weiter` bestätigen.",
        ],
        screenshot:
          "Tablet-Bildschirme zur Auswahl von Team und Raum vor dem Start der Aufsicht.",
        gallery: [
          {
            image: "/help/screens/nfc-wer-ist-dabei-leer.webp",
            caption: "„Wer ist dabei?“ - verfügbare Betreuer.",
          },
          {
            image: "/help/screens/nfc-wer-ist-dabei.webp",
            caption: "Mindestens eine Person wählen, dann „Weiter“.",
          },
          {
            image: "/help/screens/nfc-raum-waehlen.webp",
            caption: "„Wo machen wir das?“ - den Raum antippen.",
          },
        ],
      },
      {
        id: "nfc-aufsicht-bestaetigen",
        title: "Aufsicht prüfen und starten",
        summary:
          "Eine Zusammenfassung zeigt Aktivität, Team und Raum auf einen Blick, bevor die Aufsicht beginnt.",
        steps: [
          "Im Fenster `Aufsicht starten?` die gewählte Aktivität, die Anzahl der Betreuer und den Raum prüfen.",
          "Bei einem Fehler über den Zurück-Pfeil zu den vorherigen Schritten navigieren.",
          "Mit dem grünen Button `Aufsicht starten` bestätigen und den NFC-Scanner aktivieren.",
          "Das Tablet wechselt zum Hauptbildschirm mit Aktivität, Raum und einem großen Zähler. Ab jetzt können sich die Kinder ein- und auschecken.",
        ],
        screenshot:
          "Bestätigungsfenster „Aufsicht starten?“ mit Aktivität, Betreuer und Raum.",
        image: "/help/screens/nfc-aufsicht-bestaetigen.webp",
      },
    ],
  },
  {
    id: "nfc-ein-auschecken",
    title: "Kinder ein- und auschecken",
    description:
      "Das Ein- und Auschecken ist die zentrale Funktion im Alltag. Die Kinder halten einfach ihr Armband an den NFC-Sensor, den Rest erledigt das System - ein Eingreifen durch die Betreuenden ist nicht nötig.",
    icon: ScanLine,
    tone: "green",
    steps: [
      {
        id: "nfc-kinder-einchecken",
        title: "Kinder einchecken",
        summary:
          "Sobald eine Aufsicht läuft, meldet jedes Armband das Kind automatisch für die aktuelle Aktivität an.",
        steps: [
          "Der Hauptbildschirm zeigt die Aktivität, den Raum und einen großen Zähler der eingecheckten Kinder.",
          "Ein noch nicht eingechecktes Kind hält sein Armband an den NFC-Sensor.",
          "Es erscheint eine große grüne Bestätigung mit `Hallo ...!`, der Abholzeit und dem aktuellen Raum.",
          "Die Anzeige schließt sich nach wenigen Sekunden automatisch und der Zähler erhöht sich um eins.",
        ],
        screenshot:
          "Tablet zeigt die Check-in-Bestätigung „Hallo Linus!“ mit Abholzeit und Raum.",
        gallery: [
          {
            image: "/help/screens/nfc-hauptbildschirm.webp",
            caption: "Hauptbildschirm mit Aktivität, Raum und Zähler.",
          },
          {
            image: "/help/screens/nfc-kind-eingecheckt.webp",
            caption: "„Hallo Linus!“ mit Abholzeit und aktuellem Raum.",
          },
        ],
      },
      {
        id: "nfc-kinder-auschecken",
        title: "Kinder auschecken",
        summary:
          "Hält ein bereits eingechecktes Kind sein Armband erneut an, fragt das Tablet, wohin es geht.",
        steps: [
          "Das Kind scannt sein Armband erneut - es erscheint `Wohin geht ...?`.",
          "`Raumwechsel`: das Kind wechselt in einen anderen Raum.",
          "`Schulhof`: das Kind geht nach draußen auf den Schulhof oder Spielplatz.",
          "`Toilette`: das Kind verlässt den Raum kurz für einen Toilettengang.",
        ],
        screenshot:
          "Tablet-Bildschirm „Wohin geht ...?“ mit Raumwechsel, Schulhof und Toilette.",
      },
      {
        id: "nfc-kinder-nach-hause",
        title: "Nach Hause auschecken und Ziele steuern",
        summary:
          "Der letzte Button meldet ein Kind endgültig ab; welche Ziele davor erscheinen, legen die Geräte-Einstellungen fest.",
        steps: [
          "`nach Hause`: das Kind wird abgeholt und verlässt die Einrichtung. Danach erscheint optional ein freiwilliges Tages-Feedback über drei Symbole (`Gut`, `Okay`, `Schlecht`).",
        ],
        callout: {
          title: "Welche Buttons erscheinen",
          body: "Welche Ziele unter `Wohin geht ...?` angezeigt werden, steuern Sie in den Einstellungen unter `Geräte` (siehe Kapitel „NFC-Einstellungen prüfen“). `Schulhof` und `Toilette` legen automatisch einen passenden Raum an.",
          tone: "gray",
        },
        screenshot:
          "Tablet-Bildschirm „Wohin geht ...?“ mit Raumwechsel, Schulhof, Toilette und nach Hause.",
        image: "/help/screens/nfc-auschecken.webp",
      },
    ],
  },
  {
    id: "nfc-aufsicht-beenden",
    title: "Aufsicht beenden & weitere Funktionen",
    description:
      "Neben dem Ein- und Auschecken können Sie eine laufende Aufsicht beenden, das Team anpassen und sich abmelden.",
    icon: CircleStop,
    tone: "purple",
    steps: [
      {
        id: "nfc-aufsicht-abschliessen",
        title: "Aufsicht beenden und abmelden",
        printCompact: true,
        summary:
          "Am Ende der Betreuungszeit die Aufsicht abschließen und das Tablet wieder sperren.",
        steps: [
          "Auf dem Hauptbildschirm oben rechts auf `Anmelden` tippen und die 4-stellige PIN eingeben.",
          "Im Menü oben rechts auf `Aufsicht beenden` tippen und im Dialog mit `Ja, beenden` bestätigen - alle eingecheckten Kinder werden automatisch ausgecheckt.",
          "Über `Abmelden` kehrt das Tablet zum Startbildschirm zurück.",
        ],
        callout: {
          title: "Am Tagesende",
          body: "Sie können das Tablet am Ende des Tages auch einfach vom Strom trennen. Die laufende Aufsicht wird dann ebenfalls automatisch beendet und alle Kinder ausgecheckt.",
          tone: "gray",
        },
        screenshot: "Tablet-`Menü` mit `Aufsicht beenden` und `Abmelden`.",
        image: "/help/screens/nfc-tablet-menue.webp",
      },
      {
        id: "nfc-team-anpassen",
        title: "Team während der Aufsicht auswählen",
        summary:
          "Ändert sich das Betreuungsteam, öffnen Sie die Auswahl und markieren die passenden Personen.",
        steps: [
          "Im Menü auf `Team anpassen` tippen.",
          "In der Personenauswahl Teammitglieder hinzufügen oder entfernen - ausgewählte Personen werden grün markiert.",
        ],
        screenshot: "Tablet-Bildschirm „Team anpassen“ mit Personenauswahl.",
        gallery: [
          {
            image: "/help/screens/nfc-team-leer.webp",
            caption: "„Team anpassen“: aktuelle Personenauswahl.",
          },
          {
            image: "/help/screens/nfc-team-anpassen.webp",
            caption: "Personen hinzufügen oder entfernen (grün markiert).",
          },
        ],
      },
      {
        id: "nfc-team-speichern",
        title: "Teamänderung speichern",
        summary:
          "Die Auswahl wird erst wirksam, wenn Sie sie speichern und die Bestätigung erscheint.",
        steps: [
          "Mit `Team speichern` bestätigen.",
          "Eine grüne Meldung `Team erfolgreich gespeichert!` bestätigt die Aktualisierung.",
        ],
        screenshot:
          "Tablet-Bildschirm „Team erfolgreich gespeichert!“ nach einer Teamänderung.",
        gallery: [
          {
            image: "/help/screens/nfc-team-gespeichert.webp",
            caption: "„Team erfolgreich gespeichert!“",
          },
        ],
      },
    ],
  },
  {
    id: "nfc-einstellungen-kapitel",
    title: "NFC-Einstellungen prüfen",
    description:
      "Einige NFC-Einstellungen legen Sie einmal im Browser fest. Sie steuern, was das Tablet anzeigt. Diese Optionen stehen nur online zur Verfügung.",
    icon: SlidersHorizontal,
    tone: "purple",
    steps: [
      {
        id: "nfc-einstellungen-geraete",
        title: "Geräte-Einstellungen anpassen",
        summary:
          "PIN und die Auswahl-Buttons des Tablets legen Sie im Browser unter `Einstellungen` fest.",
        steps: [
          "Im Browser die `Einstellungen` öffnen und den Tab `Geräte` wählen.",
          "Unter `OGS Geräte-PIN` die 4-stellige PIN setzen, mit der sich das Team am Tablet anmeldet.",
          "Mit `Raumwechsel-Button anzeigen`, `Schulhof-Button anzeigen` und `Toilette-Button anzeigen` festlegen, welche Ziele beim Auschecken (`Wohin geht ...?`) erscheinen.",
          "Änderungen werden automatisch gespeichert und beim nächsten Start vom Tablet übernommen.",
        ],
        callout: {
          title: "Weniger ist mehr",
          body: "`Schulhof` und `Toilette` legen automatisch einen passenden Raum an. Aktivieren Sie nur die Buttons, die Ihre Einrichtung wirklich nutzt - weniger Auswahl ist für die Kinder am Tablet einfacher.",
          tone: "blue",
        },
        screenshot:
          "Einstellungen, Tab `Geräte` mit OGS Geräte-PIN und Button-Schaltern.",
        image: "/help/screens/nfc-einstellungen-geraete.webp",
      },
      {
        id: "nfc-einstellungen-betrieb",
        title: "Abmeldezeit und Anwesenheits-Modus",
        summary:
          "Im Tab `Betrieb` steuern Sie, ab wann Kinder nach Hause auschecken können.",
        steps: [
          "In den `Einstellungen` den Tab `Betrieb` öffnen.",
          "Unter `Tägliche Abmeldezeit` festlegen, ab wann Kinder über `nach Hause` ausgecheckt werden können. Bleibt das Feld leer, ist das Auschecken jederzeit möglich.",
        ],
        callout: {
          title: "Anwesenheits-Modus nur über das moto-Team",
          body: "Der `Anwesenheits-Modus` (detailliert mit Räumen oder binär nur an-/abwesend) verändert grundlegend, wie das Tablet arbeitet, und wird aus Sicherheitsgründen nur vom moto-Team umgestellt. Melden Sie sich dafür beim Support.",
          tone: "gray",
        },
        screenshot: "Einstellungen, Tab `Betrieb` mit Tägliche Abmeldezeit.",
        image: "/help/screens/nfc-einstellungen-betrieb.webp",
      },
    ],
  },
  {
    id: "nfc-fehlerbehebung",
    title: "Fehlerbehebung",
    description:
      "In den meisten Fällen funktioniert das NFC-Tablet zuverlässig. Die häufigsten Störungen lassen sich in wenigen Sekunden ohne technische Kenntnisse beheben.",
    icon: Wrench,
    tone: "red",
    steps: [
      {
        id: "nfc-fehler-erkennung-netzwerk",
        title: "Erkennung, PIN und Netzwerk prüfen",
        summary:
          "Die häufigsten Startpunkte: Armband ruhig scannen, PIN prüfen und die Internetverbindung kontrollieren.",
        steps: [
          "`Armband wird nicht erkannt`: Armband ruhig und mittig flach auf den NFC-Sensor legen und ca. 1-2 Sekunden halten. Hilft das nicht, ein anderes Armband testen, sonst Gerät neu starten.",
          "`PIN wird nicht akzeptiert`: Prüfen, ob die richtige PIN eingegeben wird. Sie kann von der OGS-Leitung geändert worden sein - im Zweifel dort nach der aktuellen PIN fragen.",
          "`Kein Internet / Verbindungsprobleme`: Erscheint unten rechts ein rotes oder durchgestrichenes WLAN-Symbol, das Gerät neu starten. Bleibt das Problem, den WLAN-Router prüfen oder den IT-Dienstleister kontaktieren.",
        ],
        screenshot:
          "Übersicht der häufigsten Störungen bei Erkennung, PIN und Netzwerk.",
      },
      {
        id: "nfc-fehler-app-zuweisung",
        title: "App neu starten oder Zuweisung korrigieren",
        summary:
          "Wenn die App hängt oder das falsche Kind erscheint, hilft meist ein Neustart oder die Prüfung der Armband-Zuweisung.",
        steps: [
          "`App reagiert nicht / Bildschirm eingefroren`: Gerät für ca. 10 Sekunden vom Strom trennen und wieder einstecken. Die moto-App startet automatisch neu und ist nach ca. 1-2 Minuten wieder einsatzbereit.",
          "`Falsches Kind wird beim Scannen angezeigt`: Über `Armband identifizieren` das Armband scannen, die aktuelle Zuweisung prüfen, die falsche Zuweisung aufheben und das Armband dem richtigen Kind zuweisen.",
        ],
        callout: {
          title: "Neustart hilft fast immer",
          body: "Bei nahezu allen technischen Problemen hilft ein Neustart: Gerät ca. 10 Sekunden vom Strom trennen und wieder einstecken. Ein Einschaltknopf ist nicht nötig.",
          tone: "gray",
        },
        screenshot:
          "Übersicht der Lösungen für eingefrorene App und falsche Armband-Zuweisung.",
      },
    ],
  },
];
