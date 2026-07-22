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
  Download,
  Eye,
  FileText,
  KeyRound,
  LayoutDashboard,
  Megaphone,
  MessageSquare,
  MonitorPlay,
  Nfc,
  PlayCircle,
  PlugZap,
  Repeat,
  ScanLine,
  Search,
  SlidersHorizontal,
  TabletSmartphone,
  UtensilsCrossed,
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
  readonly searchable?: boolean;
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
      "Planung: Betreuungsplan, Dienstplan und Vertretung",
      "Datenverwaltung, Anmeldungen, Feedback, Einstellungen",
    ],
  },
  {
    href: "/help/nfc/erste-schritte",
    title: "NFC Erste Schritte",
    body: "Das kompakte Einlegeblatt für neue NFC-Tablets: montieren, anmelden und die ersten Armbänder zuweisen.",
    icon: PlugZap,
    points: [
      "Für den Versandkarton und den schnellen Start",
      "Drei Schritte kompakt zusammengefasst",
      "Als druckfertige A4-Seite aufgebaut",
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
export const nfcQuickstartChapters: readonly GuideChapter[] = [
  {
    id: "nfc-erste-schritte",
    title: "NFC Erste Schritte",
    description:
      "Drei kurze Schritte, um ein neues NFC-Tablet einsatzbereit zu machen und die ersten Armbänder den Kindern zuzuweisen.",
    icon: PlugZap,
    tone: "green",
    steps: [
      {
        id: "tablet-montieren-und-einschalten",
        title: "Tablet montieren und einschalten",
        summary:
          "Passenden Platz mit Strom und guter WLAN-Reichweite wählen, optional montieren und Strom einstecken.",
        steps: [
          "Vor der Montage einen Platz mit Strom und guter WLAN-Reichweite wählen.",
          "Optional mit Wand- oder Standhalterung montieren und die Angaben des Herstellers beachten.",
          "Danach Strom einstecken. Das Gerät startet automatisch und verbindet sich mit dem hinterlegten WLAN.",
        ],
        screenshot:
          "Das NFC-Tablet zeigt nach dem Start den moto-Startbildschirm.",
      },
      {
        id: "mit-geraete-pin-anmelden",
        title: "Mit Geräte-PIN anmelden",
        summary: "Auf Anmelden tippen und die 4-stellige Geräte-PIN eingeben.",
        steps: [
          "Auf dem Startbildschirm auf Anmelden tippen.",
          "Die 4-stellige PIN eingeben. Die Standard-PIN bei Auslieferung ist 1234.",
          "Nach der vierten Ziffer prüft das Tablet die Eingabe automatisch.",
        ],
        callout: {
          title: "Empfehlung",
          body: "Die Standard-PIN nach der ersten Anmeldung unter Einstellungen, Geräte, OGS Geräte-PIN ändern.",
          tone: "green",
        },
        screenshot:
          "Das Tablet prüft die Geräte-PIN automatisch nach der vierten Ziffer.",
      },
      {
        id: "armbaender-zuweisen",
        title: "Armbänder zuweisen",
        summary:
          "Armband identifizieren, Scan starten, Armband an das Lesegerät halten und dem Kind zuweisen.",
        steps: [
          "Auf dem Startbildschirm Armband identifizieren tippen und Scan starten.",
          "Armband ruhig an das Lesegerät halten.",
          "Person auswählen tippen, Kind auswählen und mit Armband zuweisen bestätigen.",
        ],
        callout: {
          title: "Voraussetzung",
          body: "Die Armbänder müssen angekommen sein und die Kinder müssen in moto angelegt sein.",
          tone: "green",
        },
        screenshot:
          "Die Zuweisung verbindet ein angekommenes Armband mit einem bereits angelegten Kind.",
      },
    ],
  },
];

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
        id: "passkey-einrichten",
        title: "Passkey einrichten",
        summary:
          "Nach der ersten Anmeldung kann ein Passkey für die Anmeldung ohne Passwort hinterlegt werden.",
        steps: [
          "`Profil` öffnen.",
          "In der Sektion `Passkeys` auf `Hinzufügen` klicken.",
          "Den Hinweis prüfen und `E-Mail senden` wählen.",
          "Das E-Mail-Postfach öffnen, den Code eingeben und einen Namen für das Gerät vergeben.",
          "Den Passkey mit der Gerätefreigabe speichern.",
          "Bei der nächsten Anmeldung `Mit Passkey anmelden` wählen.",
        ],
        callout: {
          title: "Gilt für Team- und Operator-Zugänge",
          body: "Passkeys stehen für normale Nutzerkonten und moto-Operatoren zur Verfügung. Elternkonten nutzen weiterhin den Elternbereich.",
          tone: "blue",
        },
        screenshot: "Profilseite mit der Sektion Passkeys.",
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
          "Um die Rolle einer bereits angelegten Person nachträglich zu ändern (z. B. jemanden zur Administratorin zu machen), die Person in der Personal-Liste auswählen und in der Detailansicht `Rolle verwalten` nutzen.",
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
          "Wiederkehrende Angebote vorbereiten. Dieser Schritt ist nur für Einrichtungen relevant, die mit NFC oder Tablets arbeiten.",
        steps: [
          "Falls Ihre Einrichtung mit NFC oder Tablets arbeitet: `Datenverwaltung` öffnen und `Aktivitäten` wählen.",
          "Neue Aktivität anlegen.",
          "`Name` kurz und verständlich eintragen.",
          "`Kategorie` wählen.",
          "`Maximale Teilnehmer` eintragen, wenn eine Grenze gilt.",
          "Speichern.",
        ],
        callout: {
          title: "Nur bei NFC-/Tablet-Nutzung sichtbar",
          body: "Wenn Ihre Einrichtung kein NFC oder keine Tablets nutzt, wird der Bereich `Aktivitäten` in der Datenverwaltung nicht angezeigt. In diesem Fall können Angebote später im Alltag über den `Betreuungsplan` oder, je nach Freischaltung, über `Aktuelle Aufsicht` entstehen (siehe `Die App im Alltag` -> `Aktuelle Aufsicht` und `Betreuungsplan`).",
          tone: "blue",
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
          "Erst wenn die Vorschau stimmt, auf `Kinder importieren` klicken.",
          "Eine Stichprobe in der `Kindersuche` prüfen.",
        ],
        callout: {
          title: "Geburtstage im Import",
          body: "Geburtstage können als `JJJJ-MM-TT`, `TT.MM.JJJJ` oder `TT.MM.JJ` eingetragen werden.",
          tone: "blue",
        },
        screenshot:
          "Kinderimport mit Vorlage herunterladen, Datenvorschau und Kinder importieren.",
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
          "Bei jeder Bezugsperson die `Portalrolle` prüfen: Eltern bzw. gesetzliche Vertretungen erhalten Elternportal-Zugriff, Abhol- oder Notfallkontakte nicht automatisch.",
          "Optional im Abschnitt `Betreuungszeiten` auf `Wochenplan hinzufügen` klicken und die regelmäßigen `Ankunft`- und `Abholung`-Zeiten je Wochentag eintragen.",
          "`Datenschutzeinwilligung erteilt` nur aktivieren, wenn die Einwilligung vorliegt.",
          "Auf `Erstellen` klicken. Erziehungsberechtigte und Betreuungszeiten werden zusammen mit dem Kind gespeichert.",
          "Das Kind anschließend in der `Kindersuche` prüfen; weitere Angaben ergänzen Sie jederzeit auf der Kinddetailseite.",
        ],
        screenshot:
          "Kinderformular mit Stammdaten sowie den Abschnitten Erziehungsberechtigte und Betreuungszeiten.",
        image: "/help/screens/kind-manuell-anlegen.webp",
      },
      {
        id: "betreuungszeiten-pflegen",
        title: "Ankunfts- und Gehzeiten pflegen",
        summary:
          "Für importierte Kinder oder zum späteren Ändern: die wöchentlich wiederkehrenden Ankunfts- und Abholzeiten auf der Kinddetailseite eintragen und einzelne Abweichungen ergänzen. Beim einzelnen Anlegen ist das bereits im Formular möglich.",
        steps: [
          "`Datenverwaltung` öffnen und `Kinder` wählen.",
          "Das Kind in der Liste auswählen.",
          "Im rechten Detailbereich den Tab `Betreuungszeiten` öffnen.",
          "Im Bereich `Ankunftsplan & Notizen` auf `Bearbeiten` klicken und die regelmäßigen Zeiten pro Wochentag im Format `HH:MM` eintragen.",
          "Im Bereich `Gehplan & Notizen` auf `Bearbeiten` klicken und die regelmäßigen Gehzeiten, Abholer sowie Hinweise eintragen.",
          "Einzelne Abweichungen später über das Stift-Symbol am jeweiligen Wochentag pflegen.",
          "Speichern.",
        ],
        screenshot:
          "Betreuungszeiten mit Ankunftsplan, Gehplan und Tag bearbeiten.",
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
          "Findet jedes Kind und zeigt seinen aktuellen Status und Aufenthaltsort. Über den Tagesfilter beantwortet sie auch die Frage, welche Kinder morgen oder an einem anderen Tag kommen.",
        steps: [
          "`Kindersuche` öffnen.",
          "Namen oder Namensbestandteil in das Suchfeld eingeben.",
          "Bei Bedarf nach Klasse, Gruppe, Stufe oder Status filtern.",
          "Über `Filter` im Abschnitt `Anwesenheit` beim Punkt `Tag` (`Heute`, `Morgen` oder ein frei gewähltes Datum bis zum Sonntag der laufenden Woche) festlegen, für welchen Tag die geplante Anwesenheit gilt. Direkt darunter grenzt der Filter `Kommt` beziehungsweise `Kommt nicht` die Liste auf den gewählten Tag ein; Krankmeldungen, Entschuldigungen und Tagesausnahmen werden für diesen Tag ausgewertet. Bei einem anderen Tag als heute bleiben aktuelle Aufenthaltsorte und Live-Filter ausgeblendet, denn wer gerade im Haus ist, sagt nichts über einen anderen Tag aus. Auch Ergebniszahl und `Exportieren` nutzen den gewählten Tag.",
          "Für aktuelle Klassenlisten im Filter `Klasse` den Klassenverband wählen und über `Exportieren` die Vorlage `Klassenliste` ausgeben. Ohne Klassenfilter erzeugt die Option `Nach Klassen getrennt` alle Klassenlisten auf einmal: jede Klasse erhält eine eigene Überschrift, im PDF beginnt jede Klasse auf einer neuen Seite. Phasebezogene Listen für Klassenlehrkräfte erstellst du in der jeweiligen `Anmeldephase`.",
          "Die Vorlage `Tagesliste` enthält den `Tagesstatus`, damit `Krank`, `Entschuldigt` und `Klassenfahrt` direkt auf der Liste stehen.",
          "Die Vorlage `Geburtstagsliste` gibt die Geburtstage der gefilterten Kinder nach Kalender sortiert aus, voreingestellt für den aktuellen Monat. Alle Listen der Schule gebündelt findest du unter `Datenverwaltung` -> `Exporte`.",
          "Über `Gruppieren` die Liste nach `Status`, `Raum`, `Ankunftszeit`, `Gehzeit`, `Abholregelung` oder `Laufgemeinschaft` bündeln. `Nach Laufgemeinschaft` zeigt die Kinder blockweise so, wie sie gemeinsam nach Hause gehen; Kinder ohne Verknüpfung stehen gesammelt am Ende unter `Ohne Laufgemeinschaft`.",
          "Auf jeder Karte rechts die `Aktivitäts-Indikatoren` ablesen: ein grüner Haken bedeutet, das Kind war heute schon im genannten Bereich (z. B. `Mensa`, `Hausaufgaben`), ein grauer Kreis steht für noch ausstehend.",
          "Ein Kind öffnen, um Details, Raum und Zeiten zu sehen.",
        ],
        callout: {
          title: "Keine Mensa-/Hausaufgaben-Hinweise sichtbar?",
          body: "Die `Aktivitäts-Indikatoren` erscheinen nur, wenn ein Admin sie eingeschaltet hat. Ist die Funktion aus, fehlen die Haken auf den Karten ganz. Das ist kein Fehler. Aktivieren und benennen lässt sie sich unter `Einstellungen` -> `Betrieb` -> `Aktivitäts-Indikatoren` (siehe Kapitel `Einstellungen`).",
          tone: "blue",
        },
        screenshot:
          "Kindersuche mit Suchfeld, Status-Badges und den Aktivitäts-Indikatoren Mensa und Hausaufgaben (grüner Haken = heute erledigt, grauer Kreis = ausstehend) rechts auf jeder Karte.",
        image: "/help/screens/kindersuche.webp",
      },
      {
        id: "kinderdetailansicht",
        title: "Kinderdetailansicht",
        icon: FileText,
        summary:
          "In der `Kindersuche` auf die Karte eines Kindes klicken öffnet seine Detailansicht. Der Kopfbereich zeigt Name, Gruppe und den aktuellen Aufenthalt mit Uhrzeit sowie die heutige Ankunft und Abholung; darunter liegen die Tabs zur Kartei.",
        steps: [
          "In der `Kindersuche` auf die Karte des Kindes klicken.",
          "Im Kopfbereich den aktuellen Aufenthalt (z. B. `OGS-Raum 1 seit 12:00 Uhr`) sowie `Heutige Ankunft` und `Heutige Abholung` ablesen.",
          "Über `Krank melden` das Kind als krank und über `Entschuldigen` als entschuldigt markieren. Die seltene Aktion `Klassenfahrt` liegt im Drei-Punkte-Menü der Aktionsleiste; dort einen Zeitraum und optional einen Hinweis erfassen.",
          "Tab `Stammdaten`: Name, Klasse, Gruppe, Geburtstag, Adresse, Gesundheitsinformationen, Notizen, Foto und Datenschutz ansehen und über `Bearbeiten` ändern. Unter `Erlaubte Heimwege` legen Sie je Wochentag fest, wie das Kind nach Hause kommt. Sobald an einem Tag `Anderes Kind` erlaubt ist, erscheint direkt darunter `Mit welchem Kind?`: dort verknüpfen Sie die Kinder, mit denen es gemeinsam geht (Laufgemeinschaft). Angeboten werden nur die Tage, an denen `Anderes Kind` erlaubt ist. Die Verknüpfung gilt immer für beide Kinder: Wer bei Lina eingetragen ist, hat Lina automatisch auch auf seiner eigenen Karte stehen. Erlaubt der Heimweg des anderen Kindes diese Tage noch nicht, fragt die App nach und ergänzt `Anderes Kind` dort auf Wunsch; bestehende Heimwege bleiben erhalten. Das Textfeld darunter ist nur für Begleitung durch eine Person, die kein Kind der Schule ist. Wenn das Kind über eine Online-Anmeldung übernommen wurde, stehen dort auch kindbezogene Zusatzantworten aus dem Anmeldeformular zur Ansicht.",
          "Tab `Nachrichten`: die Unterhaltung mit einer Bezugsperson zu diesem Kind ansehen und über `Neue Nachricht` der Bezugsperson schreiben. Pro Kind und Bezugsperson gibt es eine fortlaufende Unterhaltung (wie ein Chat, ohne Betreff). Ungelesene Eltern-Nachrichten sind mit einem roten Abzeichen markiert; geschrieben und beantwortet wird im Chat-Fenster.",
          "Tab `Erziehungsberechtigte`: Bezugspersonen mit Kontaktdaten, Abholberechtigung und Notfallkontakten pflegen. Pro Person zeigt ein Status, ob sie ein Konto für das Elternportal hat (`Konto aktiv`, `Einladung offen` oder `Kein Konto`); mit `Einladen` laden Sie eine bereits hinterlegte Bezugsperson zum Elternportal ein, ohne die Daten erneut einzugeben.",
          "Tab `Betreuungszeiten`: die wöchentlichen Ankunfts- und Abholzeiten je Wochentag verwalten und einzelne Tage anpassen. Hat ein Elternteil über das Elternportal eine Ankunfts- oder Abholzeit für einen Tag geändert, ist dieser Tag mit `Von Eltern` markiert; beim Ändern oder Entfernen dieser Zeit fragt die App zur Sicherheit nach, damit die Angabe der Eltern nicht versehentlich überschrieben wird.",
          "Tab `Historie`: die Anwesenheits-Historie je Betreuungsangebot nachvollziehen. Morgen- und Nachmittagsbetreuung erscheinen als getrennte Zeitslots; ungeplante Besuche sind gekennzeichnet. Die Daten lassen sich als PDF, DOCX oder XLSX exportieren. Raum-Details ergänzen die Slot-Historie, soweit die Aufbewahrungsfrist sie noch zulässt.",
          "Tab `Anmeldungen` (nur Admins): Online-Anmeldungen anzeigen, die dieses Kind ins System gebracht haben. Dort sehen Sie Betreuungsangebote, Gesundheitsangaben, Notfallkontakte, Zustimmungen und Zusatzantworten; Erziehungsberechtigte stehen weiterhin im eigenen Tab. Bei bestätigten Kindern können Betreuungsangebote dort nachträglich mit Begründung korrigiert werden, solange `Betreuungsangebote anbieten` unter `Einstellungen` -> `Anmeldung` aktiviert ist. Die Angaben können außerdem exportiert werden.",
        ],
        callout: {
          title: "Abmeldungen von Eltern",
          body: "Wenn das Elternportal aktiv ist, können Eltern ihr Kind selbst abmelden – wahlweise als `Krank` oder als `Entschuldigt` (z. B. wegen eines Termins), auch für Tage in der Zukunft. Bei einer entschuldigten Abmeldung ist eine kurze Begründung Pflicht. Eine Krankmeldung erscheint sofort wie eine des Teams (das Kind wird als krank angezeigt). Eine entschuldigte Abmeldung wird standardmäßig ebenfalls sofort eingetragen; Sie können in den `Einstellungen` im Bereich `Elternportal` aber festlegen, dass entschuldigte Abmeldungen erst vom Team bestätigt werden müssen. Bis zur Bestätigung gilt das Kind als erwartet, und die Anfrage erscheint unter `Eltern` > `Änderungsanfragen`.",
          tone: "blue",
        },
        screenshot:
          "Kinderdetailansicht mit Statuskopf, den Aktionen Krank melden, Entschuldigen und weiteren Statusaktionen im Drei-Punkte-Menü sowie den Tabs Stammdaten, Nachrichten, Erziehungsberechtigte, Betreuungszeiten, Historie und Anmeldungen.",
        image: "/help/screens/kinderdetailansicht.webp",
      },
      {
        id: "eltern-konten-verbinden",
        title: "Eltern-Konten verbinden",
        icon: Users,
        summary:
          "Steuern Sie, wer Zugriff auf ein Kind im Elternportal hat. Sie können weitere Bezugspersonen einladen und bestehende Zugänge wieder trennen – pro Kind sind oft mehrere Konten sinnvoll (zweiter Elternteil, Großeltern).",
        steps: [
          "Im Tab `Erziehungsberechtigte` eines Kindes am Status erkennen, wer bereits ein Konto hat (`Konto aktiv`), eingeladen ist (`Einladung offen`) oder noch keinen Zugang hat (`Kein Konto`).",
          "Mit `Einladen` neben einer Person eine Einladung zum Elternportal an deren hinterlegte E-Mail-Adresse senden. Die Person legt sich darüber selbst ein Passwort an.",
          "Einen bestehenden Zugang über `Bearbeiten` -> `Entfernen` wieder trennen – die Person sieht das Kind danach nicht mehr im Elternportal.",
          "Ob Eltern selbst weitere Bezugspersonen einladen dürfen, steuern Sie unter `Einstellungen` im Bereich `Elternportal` (`Deaktiviert`, `Direkt` oder `Mit Freigabe durch das Team`).",
        ],
        callout: {
          title: "Konto-Anfragen freigeben",
          body: "Ist `Mit Freigabe durch das Team` gewählt, landen von Eltern angestoßene Einladungen in der Seitenleiste unter `Eltern` > `Konto-Anfragen` (nur für Admins). Dort sehen Sie, wer für welches Kind angefragt wurde, und geben die Anfrage mit `Freigeben` frei oder lehnen sie mit `Ablehnen` ab. Erst nach der Freigabe wird der Zugang gewährt.",
          tone: "green",
        },
        screenshot:
          "Tab Erziehungsberechtigte mit Kontostatus-Markierungen und Einladen-Schaltflächen sowie die Admin-Seite Konto-Anfragen mit Freigeben- und Ablehnen-Aktionen.",
        gallery: [
          {
            image: "/help/screens/erziehungsberechtigte-konten.webp",
            caption:
              "Tab „Erziehungsberechtigte“: pro Person der Kontostatus (Konto aktiv / Kein Konto) und die Schaltfläche „Einladen“.",
          },
          {
            image: "/help/screens/konto-anfragen.webp",
            caption:
              "Seite „Konto-Anfragen“: von Eltern angestoßene Einladungen mit „Freigeben“ bestätigen oder mit „Ablehnen“ abweisen.",
          },
        ],
      },
      {
        id: "stammdaten-aenderungen-pruefen",
        title: "Änderungsanfragen der Eltern prüfen",
        icon: ClipboardCheck,
        summary:
          "Eltern pflegen viele Stammdaten ihres Kindes im Elternportal selbst. Sensible Angaben (Name, Geburtsdatum, Gehzeiten) und die dauerhaften Betreuungszeiten ändern sie nur auf Anfrage – diese geben Sie hier zentral frei.",
        steps: [
          "Die meisten Felder (z. B. Gesundheitshinweise, eigene Kontaktdaten der Eltern) ändern Eltern direkt; die Änderung wird sofort übernommen und protokolliert.",
          "Für Name, Geburtsdatum und Gehzeiten sowie für die dauerhaften Bring- und Abholzeiten reichen Eltern über `Änderung anfragen` einen Vorschlag ein, statt direkt zu ändern.",
          "Offene Anfragen finden Sie als Admin in der Seitenleiste unter `Eltern` > `Änderungsanfragen`, getrennt nach `Stammdaten`, `Betreuungszeiten` und – falls aktiviert – `Entschuldigte Abmeldungen`.",
          "Pro Anfrage sehen Sie das Kind und die Änderung (alter → neuer Wert); bei Betreuungszeiten den Wochenplan-Vergleich je Wochentag (Bringzeit, Abholzeit, Abholart); bei entschuldigten Abmeldungen die betroffenen Tage und die Begründung der Eltern.",
          "Mit `Freigeben` wird der neue Wert übernommen – bei Betreuungszeiten direkt in den Wochenplan des Kindes, bei einer entschuldigten Abmeldung wird das Kind für die Tage als entschuldigt eingetragen. Mit `Ablehnen` bleibt der bisherige Stand erhalten; bei Betreuungszeiten und entschuldigten Abmeldungen ist dafür eine Begründung erforderlich, bei Stammdaten optional.",
          "Die Eltern sehen die Entscheidung als Hinweis in ihrem Nachrichten-Verlauf.",
          "Ob Eltern Stammdaten direkt ändern bzw. Änderungen anfragen dürfen, steuern Sie unter `Einstellungen` im Bereich `Elternportal`.",
        ],
        callout: {
          title: "Was wird direkt übernommen?",
          body: "Direkt geänderte Felder sind sofort wirksam und im Verlauf nachvollziehbar. Nur die freigabepflichtigen Felder warten auf Ihre Bestätigung – bis dahin bleibt der bisherige Wert gültig.",
          tone: "blue",
        },
        screenshot:
          "Admin-Seite „Änderungsanfragen“ mit den Bereichen Stammdaten und Betreuungszeiten, je mit Kind, Änderung (alt → neu) sowie Freigeben- und Ablehnen-Schaltflächen.",
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
          "Im Gruppenstatus die Krank- und Entschuldigt-Zahlen der Gruppe prüfen; Klassenfahrt zählt als bekannte Entschuldigung und wird am Kind als eigener Status angezeigt.",
          "In der Schülerdatenbank bei Gruppierung nach Klasse oder Gruppe über das Drei-Punkte-Menü `Klassenfahrt planen` für mehrere Kinder gleichzeitig setzen.",
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
          "Ungeplantes Kind über `Weiteres Kind suchen...` hinzufügen.",
          "Im Bereich `Kinder unterwegs` Kinder ohne Raum auswählen, einen Zielraum wählen und mit `In Raum setzen` zuweisen.",
          "Den Schulhof über den Schulhof-Tab und `Aufsicht übernehmen` führen. Wird im Dialog für eine spontane Aktivität der Raum `Schulhof` gewählt, öffnet `Zur Schulhof-Aufsicht` direkt diesen separaten Tab.",
          "Für ein neues Angebot `Spontane Aktivität starten`.",
        ],
        callout: {
          title: "Warum steht ein Kind auf `Nicht eingeplant`?",
          body: "Wird einer Aktivität eine ganze Gruppe, Klasse oder Jahrgangsstufe zugewiesen, gehören alle Kinder dauerhaft dazu. Wer laut seinen `Betreuungszeiten` an diesem Wochentag gar nicht in der OGS ist, erscheint deshalb grau als `Nicht eingeplant` und zählt nicht zu `Erwartet`. Das Kind bleibt trotzdem in der Liste: Kommt es doch, checken Sie es ganz normal ein. Kinder ohne hinterlegte Betreuungszeiten gelten weiterhin als erwartet.",
        },
        screenshot:
          "Laufende Aufsicht mit Anwesend, Kinder unterwegs und Spontane Aktivität.",
        image: "/help/screens/aktuelle-aufsicht.webp",
      },
      {
        id: "erinnerungen",
        title: "Erinnerungen",
        icon: Clock3,
        summary:
          "Eine Übersicht, die rein visuell an Abholungen und Aktivitäten erinnert, die anstehen oder überfällig sind. Kein Ton.",
        steps: [
          "Oben rechts auf die Glocke tippen (auf jedem Gerät sichtbar, auch am Tablet).",
          "Die Vorschau zeigt die dringendsten Einträge zuerst (überfällige vor anstehenden).",
          "Je Eintrag stehen rechts die Uhrzeit und der Abstand (z. B. `in 10 Min` oder `8 Min überfällig`).",
          "Die rote Zahl an der Glocke zeigt jederzeit, wie viele Erinnerungen gerade offen sind. `Alle ansehen` öffnet die vollständige Liste, dort nach Art gruppiert: `Überfällige Abholung`, `Überfällige Aktivität` (geplante AG nicht rechtzeitig gestartet), `Anstehende Abholung` und `Aktivitätsbeginn`.",
          "Als Betreuer siehst du die Kinder deiner aktuellen Aufsicht; als Admin alle anwesenden Kinder.",
        ],
        callout: {
          title: "Nichts zu sehen?",
          body: "Erinnerungen sind im Auslieferungszustand komplett aus. Welche Arten erscheinen (und mit welcher Vorlaufzeit), schaltet ein Admin unter `Einstellungen` -> `Erinnerungen` ein. Solange keine Art aktiv ist oder gerade nichts ansteht, bleibt die Seite leer.",
          tone: "blue",
        },
        screenshot:
          "Glocke oben rechts mit roter Zähler-Markierung; geöffnetes Vorschau-Dropdown mit den dringendsten Einträgen zuerst (überfällige vor anstehenden), je Eintrag Name, Uhrzeit und Abstand, plus `Alle ansehen`.",
      },
    ],
  },
  {
    id: "raeume-team-vertretung",
    title: "Räume, Team und Gruppenzugriff",
    description:
      "Den Überblick über Angebote, Räume und das Team behalten und kurzfristigen Gruppenzugriff organisieren.",
    icon: Building2,
    tone: "blue",
    steps: [
      {
        id: "aktivitaeten",
        title: "Aktivitäten",
        icon: Activity,
        summary:
          "Liste aller Aktivitäten mit Suche und Filter; hier legst du neue Angebote an. Dieser Bereich ist nur bei Einrichtungen mit NFC-/Tablet-Nutzung sichtbar.",
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
          "Zeigt den Status des Teams: wer anwesend ist, in welchem Raum, im Homeoffice oder abwesend.",
        steps: [
          "`Mitarbeiter` öffnen.",
          "Nach Name suchen.",
          "Nach Status filtern, zum Beispiel `Anwesend`, `Abwesend`, `Im Raum`, `Homeoffice` oder `Krank/Urlaub`.",
          "Bei Karten mit Raumangabe die aktuelle Aufsicht direkt in der Liste prüfen.",
          "Als Admin eine Person öffnen, um das Mitarbeiterprofil zu sehen.",
        ],
        screenshot:
          "Mitarbeiterliste mit Status-Badges und aktiven Aufsichten.",
        image: "/help/screens/mitarbeiter.webp",
      },
      {
        id: "mitarbeiter-admin-profil",
        title: "Mitarbeiterprofil für Admins",
        icon: Eye,
        summary:
          "Bündelt Auswertung, Zeiterfassung, Arbeitszeitmodell und Abwesenheiten einer Person.",
        steps: [
          "`Mitarbeiter` öffnen und eine Person auswählen. Die Detailansicht ist nur für Admins erreichbar.",
          "Im Reiter `Übersicht` Stundenkonto, Urlaubstage und Krankheitstage prüfen. Die Diagramme lassen sich einzeln nach Zeitraum filtern.",
          "Im Reiter `Zeiterfassung` zwischen Woche und Monat wechseln und Plan (geplante Schicht aus dem Dienstplan), Check-in, Check-out, Pause, Soll, Ist, Saldo, Quelle und Hinweise kontrollieren. So stehen geplante Schicht und tatsächliche Zeit nebeneinander.",
          "In der Monatsansicht zeigt die `Monatskarte` Übertrag aus dem Vormonat, Summe Soll, Summe Ist, Gutschriften für Krankheit und Urlaub (mit Tagen), die im Dienstplan verplanten Stunden und den Monatssaldo. Krankheit und Urlaub erzeugen keine Minusstunden — der Tag wird mit dem Tagessoll gutgeschrieben. Alle Werte werden live berechnet: Eine Korrektur in einem früheren Monat aktualisiert den Übertrag automatisch.",
          "Eine Zeile mit Änderungshistorie aufklappen, um geplante gegen tatsächliche Zeit zu sehen. Wurde beim Stempeln eine Abweichung begründet, erscheint sie dort als `Abweichung` mit geplanter Zeit, Ist-Zeit und Grund.",
          "Bei einem Arbeitstag das Stift-Symbol nutzen, um Zeiten nachzutragen oder zu korrigieren. Eine Begründung ist erforderlich und landet im Audit-Log.",
          "Im Reiter `Arbeitszeitmodell` eine Vorlage zuweisen oder ein eigenes Modell mit 1 bis 4 Wochen Rotation pflegen. Pro Arbeitstag kann optional eine Startzeit hinterlegt werden.",
          "Im Reiter `Abwesenheiten` Urlaubsanspruch und offene Anträge prüfen, genehmigen oder mit Begründung ablehnen.",
          "Über `Krank melden` im Reiter `Abwesenheiten` eine Krankmeldung für die Person eintragen; sie storniert reguläre Schichten und markiert Betreuungsblöcke als abwesend. Bereits eingetragene Vertretungsschichten bleiben bestehen und müssen manuell geprüft werden. Ein halber Krankheitstag gilt immer für ein einzelnes Datum und ändert Dienst- und Betreuungsplan nicht automatisch. Beim Löschen der Krankmeldung werden nur Schichten und Blöcke ohne eingetragenen Ersatz automatisch wiederhergestellt.",
        ],
        callout: {
          title: "Änderungen bleiben nachvollziehbar",
          body: "Zeitkorrekturen und nachgetragene Einträge werden mit Begründung gespeichert. Zeilen mit Änderungshistorie lassen sich aufklappen, damit die Leitung spätere Korrekturen prüfen kann.",
          tone: "orange",
        },
        screenshot:
          "Mitarbeiterprofil mit Tabs für Übersicht, Zeiterfassung, Arbeitszeitmodell und Abwesenheiten.",
        printCompact: true,
      },
      {
        id: "vertretungen",
        title: "Gruppenzugriff",
        icon: Repeat,
        summary:
          "Gewährt verfügbaren Fachkräften vorübergehend Zugriff auf eine OGS-Gruppe, etwa wenn jemand kurzfristig einspringt (nur für Admins, nur bei Arbeit mit festen Gruppen).",
        steps: [
          "`Gruppenzugriff` öffnen.",
          "Filter `Verfügbar` wählen und eine Person auswählen.",
          "`Zugriff gewähren` öffnen.",
          "`OGS-Gruppe` und `Anzahl der Tage` festlegen.",
          "Mit `Zuweisen` speichern.",
          "Nach Ende im aktiven Eintrag auf `Beenden` klicken.",
        ],
        callout: {
          title: "Zugriff ist keine Vertretungsplanung",
          body: "Gruppenzugriff regelt nur, wer die Kinder einer fremden Gruppe sehen und betreuen darf. Wer Personalausfälle in geplanten Betreuungsblöcken organisiert, nutzt dafür den Bereich `Planung` -> `Vertretung`. Bei offener Betreuung ohne feste Gruppen wird dieser Bereich nicht angezeigt.",
          tone: "blue",
        },
        screenshot:
          "Gruppenzugriff mit verfügbaren Fachkräften und Dialog Zugriff gewähren.",
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
        id: "kalenderzeitraeume",
        title: "Kalenderzeiträume",
        icon: CalendarDays,
        summary:
          "Legt Halbjahre, Ferien und Sonderzeiträume als gemeinsame Basis für Anmeldung und Betreuungsplan an (nur für Admins).",
        steps: [
          "In der Seitenleiste den Bereich `Planung` aufklappen und `Kalenderzeiträume` öffnen. Alternativ führt der Zeitraum-Chip oben im Betreuungsplan oder Dienstplan über `Zeiträume verwalten` zur selben Seite.",
          "`Halbjahr anlegen` klicken: Name, Art und Start-/Enddatum des nächsten Halbjahres sind bereits vorausgefüllt und lassen sich anpassen.",
          "Für Ferien oder Sonderzeiträume `Zeitraum anlegen` nutzen und die Art entsprechend wählen.",
          "Nur aktive Zeiträume legen Termine aus Regelterminen des Betreuungsplans an.",
          "Zwei aktive Zeiträume derselben Art dürfen sich nicht überschneiden; Phoenix lehnt das Speichern mit einer Fehlermeldung ab. Ferien innerhalb eines Schuljahres bleiben möglich, weil sie eine andere Art haben.",
          "Beim Anlegen einer `Anmeldephase` kann der Zeitraum ausgewählt werden; Beginn und Ende der Phase werden daraus übernommen.",
          "Die Verknüpfung geht auch andersherum: Beim Bearbeiten eines Zeitraums lassen sich unter `Verknüpfte Anmeldephasen` Phasen direkt an- und abwählen.",
          "Die Spalte `Verwendung` zeigt, welche Anmeldephasen und Regeltermine auf einen Zeitraum verweisen.",
        ],
        callout: {
          title: "Betreuungsplan ohne Planungszeitraum",
          body: "Existiert noch kein Planungszeitraum, zeigt der Betreuungsplan statt des Rasters eine Hinweiskarte `Noch kein Planungszeitraum` mit der Schaltfläche `Planungszeitraum anlegen`; sie öffnet denselben Dialog wie `Zeitraum anlegen` hier in der Verwaltung.",
          tone: "blue",
        },
        screenshot:
          "Kalenderzeiträume-Liste mit angelegtem Halbjahr und Schnellaktion Halbjahr anlegen.",
        image: "/help/screens/kalenderzeitraeume.webp",
      },
      {
        id: "stundenplan",
        title: "Betreuungsplan",
        icon: CalendarDays,
        summary:
          "Plant Termine, Regeltermine, Räume, Personal und erwartete Kinder im Voraus (nur für Admins).",
        steps: [
          "In der Seitenleiste den Bereich `Planung` aufklappen und `Betreuungsplan` öffnen. Oben zwischen den Ansichten `Woche`, `Monat` und `Serien` wechseln; die Pfeile und `Heute` navigieren durch Woche oder Monat, die Serienansicht zeigt stattdessen die Liste aller Regeltermine des sichtbaren Planungszeitraums.",
          "Der `Zeitraum`-Chip in der Kontextzeile zeigt den Planungszeitraum des sichtbaren Datums; ein Klick öffnet eine Liste zum Umspringen, `Zeiträume verwalten` führt zur Verwaltungsseite. Der Chip `Bedarf: …` daneben zeigt, welche Anmeldephase den Bedarf des Zeitraums liefert.",
          "Der Lücken-Chip in der Kontextzeile zählt offene Personal-Lücken des sichtbaren Zeitraums; ein Klick öffnet eine Sprungliste mit Uhrzeit, Titel und Soll/Ist je Lücke, die direkt zum betroffenen Termin springt.",
          "In der Wochenansicht eine freie Zelle am gewünschten Tag und zur gewünschten Uhrzeit anklicken (beim Überfahren erscheint `+ Termin`). Das Formular öffnet sich als dreistufiger Assistent mit den Schritten `Termin`, `Wiederholung` und `Personal und Kinder`.",
          "Schritt 1 `Termin`: `Titel` eintragen, bei Bedarf eine `Kategorie` und `Raum` wählen. Der `Schulhof` kann hier als reiner Planungsort verwendet werden; die laufende Aufsicht wird später unabhängig davon über den separaten Schulhof-Tab geführt. Zugeordnete Kinder und Mitarbeitende werden nicht automatisch in die Schulhof-Aufsicht übernommen. `Datum`, `Start` und `Ende` sind aus der Zelle übernommen und lassen sich anpassen, dazu optional Notizen. Aus einer leeren Zelle heraus lässt sich der Termin schon nach Schritt 1 speichern; Wiederholung und Personal/Kinder sind optional und werden dann auf `Einmalig` ohne zugeordnetes Personal gesetzt.",
          "Schritt 2 `Wiederholung`: festlegen, wie oft das Angebot stattfindet: `Einmalig`, wöchentlich am gewählten Wochentag, `Jeden Wochentag (Mo–Fr)` oder `Benutzerdefiniert …` für eigene Rhythmen. Bei `Alle 2 Wochen` erscheint zusätzlich die Auswahl `Woche A` oder `Woche B`; vorausgewählt ist die Woche des angeklickten Datums, sofern der Kalenderzeitraum einen A/B-Zyklus hat. Dieser Schritt legt außerdem den Planungszeitraum fest und zeigt beim Bearbeiten eines Serientermins die Optionen zum Splitten oder Beenden der Serie.",
          "Schritt 3 `Personal und Kinder`: `Personal` und `Kinder` zuordnen. Mit `Jahrgang/Klasse/Gruppe komplett hinzufügen …` kommt eine ganze Zielgruppe auf einmal in die Auswahl. Suche und Filter helfen bei langen Kinderlisten. Hier erscheinen auch Hinweise zu doppelt belegten Räumen, Personal oder Kindern sowie zum Abgleich mit dem Dienstplan; sie verhindern das Speichern nicht.",
          "`Benötigtes Personal` legt in Schritt 3 den Personalbedarf des Blocks fest. Bleibt das Feld leer, berechnet Phoenix den Bedarf automatisch aus der Kinderzahl und dem Betreuungsschlüssel; eine eingetragene Zahl überschreibt diese Berechnung und bestimmt die Besetzungsanzeige (z. B. `2/3`).",
          "Bei einem Regeltermin in Schritt 3 unter `Zielgruppe` festlegen, für wen der Block gedacht ist: `Jahrgang`, `Klasse`, `Gruppe` oder `Angebotsauswahl`. Die Zielgruppe beschreibt den Block; bei Jahrgang, Klasse oder Gruppe übernimmt die angebotene Schaltfläche die passenden Kinder zusätzlich in die Auswahl. Bereits ausgewählte Kinder bleiben erhalten, und die Liste lässt sich danach weiter anpassen. Bei `Angebotsauswahl` kommen Kinder automatisch über ein verknüpftes Betreuungsangebot hinzu.",
          "Über `Termin wiederholen` an einem einzelnen Termin öffnet sich derselbe Assistent direkt bei Schritt 2 `Wiederholung`, um aus dem Einzeltermin eine Serie zu machen.",
          "Speichern. Wiederholte Termine trägt Phoenix automatisch für das gesamte Schuljahr ein.",
          "Neben Personal- und Kinderzahl zeigen Termine und Regeltermine eine kleine Besetzungsanzeige (z. B. `2/3`), die nach Betreuungsschlüssel färbt, wie gut der Block besetzt ist. Die Übersicht meldet auch unterbesetzte Termine, selbst wenn bereits Personal eingetragen ist.",
          "Beim Bearbeiten eines Termins aus einer Serie fragt die App, wofür die Änderung gilt: `Nur diese Woche`, `Ab jetzt dauerhaft` oder `Alle Termine der Serie`.",
          "Wenn Sie `Ab jetzt dauerhaft` oder `Alle Termine der Serie` wählen und einzelne Termine der Reihe zuvor per `Nur diese Woche` angepasst wurden (z. B. anderer Raum, andere Uhrzeit, geändertes Personal oder Kinder), warnt die App und listet die betroffenen Termine mit Datum auf, bevor diese Einzelanpassungen überschrieben werden. Notieren Sie sich diese Termine, um sie anschließend bei Bedarf erneut anzupassen.",
          "Für Notizen gibt es zwei Ebenen: Die `Wochennotiz` pflegen Sie am Regeltermin (Serie); sie erscheint an jedem Termin der Reihe und bleibt bei Re-Plan und Serienänderungen erhalten (z. B. `Raum erst ab 14 Uhr offen`). Die `Tagesnotiz` gilt nur für einen einzelnen Termin und wird über `Nur diese Woche` gespeichert. An einem Einzeltermin sehen Sie beide getrennt.",
          "Beim Löschen eines Serientermins wählen Sie zwischen `Nur diese Woche` und `Ab jetzt dauerhaft`; frühere Termine bleiben erhalten. Einen Regeltermin löschen Sie über `Bearbeiten` -> `Löschen` und wählen dort das `Ab Datum`.",
          "Geplante Termine in normalen Räumen erscheinen zur Startzeit in der `Aktuellen Aufsicht` unter `Als Nächstes` und werden dort mit `Starten` begonnen. Ein Schulhof-Termin bleibt ein Planungshinweis und wird nicht gestartet; Anwesenheit und Aufsicht werden unabhängig davon im separaten Schulhof-Tab geführt.",
          "Die Zahlen für erwartete und anwesende Kinder werden nur angezeigt, wenn `Erwartete Kinderzahl anzeigen` unter `Einstellungen` -> `Betrieb` aktiviert ist. Die Kinderliste und Planungslogik bleiben auch bei ausgeblendeten Zahlen erhalten.",
          "In der Wochenansicht lässt sich über das kleine Menü `Zeilenhöhe` in der Kontextzeile die Zeilenhöhe des Rasters zwischen `Kompakt`, `Normal` und `Komfortabel` umschalten.",
          "Öffnen Sie einen Termin, der eine Abwesenheit oder eine offene Personal-Lücke hat, zeigt der Termin-Editor unten `Vertretung bearbeiten`; der Link springt direkt zur Tagesansicht des Vertretungsplans für diesen Termin.",
        ],
        callout: {
          title: "Betreuungsplan bei Bedarf abschalten",
          body: "Der Betreuungsplan ist standardmäßig sichtbar. Einrichtungen, die ihn nicht nutzen, schalten ihn unter `Einstellungen` -> `Betrieb` mit `Betreuungsplan aktivieren` aus; Betreuungsplan, Dienstplan und Vertretung verschwinden dann aus dem Bereich `Planung` in der Seitenleiste und zeigen bei direktem Aufruf einen Hinweis, dass die Funktion deaktiviert ist. Die `Kalenderzeiträume` bleiben erreichbar, weil die Anmeldephasen (Bereich `Anmeldungen`) damit verknüpft werden.",
          tone: "blue",
        },
        screenshot:
          "Wochenansicht des Betreuungsplans mit Kopfzeile (Ansichten Woche/Monat/Serien, Zeitraum-Chip, Bedarfs-Chip, Lücken-Chip) und geplanten Terminen.",
        image: "/help/screens/stundenplan.webp",
      },
      {
        id: "vertretungsplan",
        title: "Vertretung",
        icon: Users,
        summary:
          "Zeigt für den heutigen Tag alle gestörten Positionen des Betreuungsplans und öffnet für jeden Block einen Editor für Abwesenheit, Ersatz, bewusst unbesetzte Blöcke oder eine Absage (nur für Admins).",
        steps: [
          "In der Seitenleiste den Bereich `Planung` aufklappen und `Vertretung` öffnen. Die Seite zeigt zunächst den heutigen Tag (am Wochenende den nächsten Montag).",
          "Links steht die Störungsliste des Tages: jede betroffene Position mit dem Soll/Ist/Abwesend-Tripel. Eine abwesende Person ohne eingetragenen Ersatz zeigt `Ersatz: ??`, eine bewusst unbesetzte Position `bewusst unbesetzt`, ein abgesagter Termin `abgesagt` mit Grund. Rechts zeigt der Tageskalender denselben Tag zur Orientierung; auf schmalen Bildschirmen entfällt die Kalenderspalte, die Liste bleibt allein bedienbar.",
          "Oben in der Wochenleiste (Montag bis Freitag) zeigt jeder Tages-Chip die Zahl offener Lücken dieses Tages; ein Klick wechselt den Tag, die Pfeile springen wochenweise, `Heute` kehrt zum aktuellen Tag zurück. Daneben zeigen zwei Zähler `Offen` und `Quittiert` die offenen bzw. bewusst unbesetzten Lücken des angezeigten Tages; für vergangene Tage oder bei einem Ladefehler erscheint ein Strich statt einer erfundenen Null.",
          "Mit dem Umschalter `Nur Störungen | Ganzer Tag` zwischen der reinen Störungsliste und allen Terminen des Tages wechseln. Ein Tag ohne Störungen zeigt automatisch alle Termine mit dem Hinweis `Keine Störungen an diesem Tag`.",
          "Bei einer Position `Bearbeiten` anklicken, um den Editor zu öffnen. Unter `Aktion` zwischen zwei Zweigen wählen: `Besetzung bearbeiten` markiert eine Person als abwesend, öffnet dafür eine Ersatzauswahl und erlaubt, den Block als `Bewusst unbesetzt` zu markieren, wenn keine Ersatzperson verfügbar ist; `Block absagen` sagt den Termin ab, optional mit einem Grund. Beide Zweige schließen sich gegenseitig aus.",
          "Ein einziges `Speichern` überträgt Abwesenheit, Ersatz, `Bewusst unbesetzt` oder Absage gemeinsam als eine Änderung. Der Regeltermin im Betreuungsplan bleibt dabei unverändert.",
          "Im Editor den Reiter `Verlauf` öffnen, um das Änderungsprotokoll zu sehen: wer wann Abwesenheiten, Vertretungen, Absagen oder bewusst offene Lücken eingetragen hat, samt Begründung. Bei Blöcken aus dem Betreuungsplan zwischen `Dieser Block` und `Ganzer Tag` wechseln.",
        ],
        callout: {
          title: "Abweichung statt neue Vorlage",
          body: "Änderungen in der Vertretung gelten nur für den bearbeiteten Tag und überschreiben den Betreuungsplan nicht. Wer dauerhaft etwas ändern will, passt den Regeltermin im Betreuungsplan an.",
          tone: "blue",
        },
        screenshot:
          "Vertretung-Tagesansicht mit Störungsliste, Wochenleiste und Tageskalender.",
        image: "/help/screens/vertretungsplan.webp",
      },
      {
        id: "mein-kalender",
        title: "Mein Kalender",
        icon: CalendarRange,
        summary:
          "Zeigt persönliche Termine, Einladungen, eigene Dienstplan-Schichten und zugewiesene Betreuungsangebote. Mitarbeitende mit Verwaltungsrecht erstellen Termine für Team, Eltern oder ganze Gruppen.",
        steps: [
          "`Mein Kalender` öffnen.",
          "Oben zwischen `Tag`, `Woche` und `Monat` wechseln und mit den Pfeilen oder `Heute` zum passenden Zeitraum springen. Samstag und Sonntag sind standardmäßig ausgeblendet; über `Sa/So` blendest du sie ein — steht dort ein Zähler, liegen Einträge am Wochenende.",
          "Die Tages- und Wochenansicht ordnen Einträge auf einer Stunden-Zeitachse an; Dienstplan-Schichten hinterlegen ihren Zeitraum als farbiges Band im Hintergrund, Termine und Betreuungsblöcke liegen darüber.",
          "Einträge an der Farbe unterscheiden: grüne Karten sind Termine (`Termin`), blaue Karten Betreuungsblöcke (`Betreuung`), orangefarbene die eigenen Dienstplan-Schichten (`Dienst`). Schichten kommen aus dem Dienstplan und lassen sich hier nicht bearbeiten.",
          "Mit `Neuer Termin` den Dialog öffnen. Diese Schaltfläche sehen nur Personen mit dem Recht, Kalendertermine zu verwalten.",
          "`Titel`, Datum, Uhrzeit, Ort und Beschreibung eintragen. Bei ganztägigen Terminen `Ganztägig` aktivieren.",
          "Unter `Antwortregel` wählen, ob Eingeladene zusagen/absagen müssen oder nur informiert werden.",
          "Unter `Teilnehmerübersicht` festlegen, wer die Liste der Eingeladenen und ihren Status sehen darf: nur du, Mitarbeitende mit Termin oder alle Eingeladenen.",
          "In `Empfänger auswählen` mehrere Zielgruppen kombinieren, zum Beispiel `Alle Mitarbeitenden`, einzelne Mitarbeitende, Eltern nach Klasse, Gruppe oder Kind sowie einzelne Eltern. Bereits durch eine Gruppe abgedeckte Personen werden markiert und nicht doppelt ausgewählt.",
          "Optional eine `Wiederholung` mit Intervall, Wochentagen und Enddatum setzen.",
          "Mit `Termin speichern` anlegen. Termine mit Antwortregel zeigen im Kalender `Zusagen` und `Absagen`; über `Teilnehmer` öffnest du die Übersicht.",
        ],
        callout: {
          title: "Eltern nur mit Portalzugang einladen",
          body: "Einzelne Eltern lassen sich nur auswählen, wenn sie für mindestens ein Kind Zugriff auf das Elternportal haben. So landen keine Einladungen bei Kontakten, die den Termin später nicht sehen oder beantworten können.",
          tone: "blue",
        },
        screenshot:
          "Mein Kalender mit Umschaltung Tag/Woche/Monat, Schaltfläche Neuer Termin, Termin-Dialog und Teilnehmerübersicht.",
      },
      {
        id: "dienstplan",
        title: "Dienstplan",
        icon: CalendarRange,
        summary:
          "Plant konkrete Schichten pro Mitarbeiter und Tag, zeigt Wochensummen gegen das Arbeitszeitmodell und die zugehörigen Einsätze aus dem Betreuungsplan und bietet eine Halbjahres-Übersicht (nur für Admins).",
        steps: [
          "In der Seitenleiste den Bereich `Planung` aufklappen und `Dienstplan` öffnen (mobil im `Mehr`-Menü). Der Bereich hat eine eigene Adresse; ein Neuladen der Seite oder ein geteilter Link zeigt wieder dieselbe Woche.",
          "Mit den Pfeilen zwischen den Wochen wechseln, `Heute` springt zurück zur aktuellen Woche. Oben rechts zwischen `Woche` und `Halbjahr` umschalten.",
          "Der `Zeitraum`-Chip in der Kopfzeile zeigt den Planungszeitraum der angezeigten Woche mit Name und Laufzeit; ein Klick öffnet die Liste zum Umspringen, `Zeiträume verwalten` führt zur Verwaltungsseite. Es ist dieselbe Anzeige wie im Betreuungsplan.",
          "Auf kleinen Bildschirmen die Wochenansicht horizontal wischen oder den beschrifteten Tabellenbereich fokussieren und mit der Tastatur scrollen, um weitere Wochentage zu sehen.",
          "Über `Schichtarten verwalten` (oben rechts) eigene Schichtarten pflegen: Name, Farbe, optionale Beschreibung und Aktiv-Status. `Beispiele hinzufügen` legt typische Schichtarten wie Betreuung, Vorbereitung, Vertretungsunterricht und Pause an, die anschließend frei anpassbar oder löschbar sind.",
          "Optional lassen sich einer Schichtart eine oder mehrere `Timetable-Kategorien` zuordnen. Betreuungsblöcke dieser Kategorien werden dadurch der Schichtart zugeordnet und zeigen sie im Betreuungsplan als Kennzeichnung. Eine Kategorie kann nur einer Schichtart zugeordnet sein.",
          "In der Zeile eines Mitarbeiters auf eine leere Zelle klicken, um eine Schicht anzulegen: `Beginn`, `Ende` und `Pause (Minuten)` eintragen, optional eine `Schichtart` auswählen und speichern. Jede Schicht erscheint als eigener Block mit der Farbkante der Schichtart; mehrere Schichten am selben Tag stapeln sich chronologisch, die Pause bleibt als sichtbare Lücke zwischen den Blöcken.",
          "Mit `Als Serie wiederholen` schreibt sich die Schicht automatisch fort: Wochentage anhaken, einen Kalenderzeitraum wählen und den Wochenrhythmus festlegen (`Jede Woche` oder bei zweiwöchigem Zyklus `Woche A`/`Woche B`). Die Serie plant die Schicht ab dem Folgetag für jede passende Woche bis zum Ende des Zeitraums oder bis zum optionalen Enddatum ein; Tage mit einer bereits vorhandenen Schicht werden übersprungen und nach dem Speichern aufgelistet.",
          "Eine vorhandene Schicht anklicken, um sie zu bearbeiten oder über `Schicht löschen` zu entfernen. Serien-Schichten (graues Wiederhol-Symbol auf dem Block) fragen dabei nach dem Umfang: `Nur diese Woche` ändert oder löscht genau diesen Termin (die Serie plant ihn nicht erneut ein, das Symbol färbt sich gelb), `Ab jetzt dauerhaft` teilt die Serie ab diesem Datum. Einzeln angepasste Termine bleiben bei späteren Serienänderungen erhalten.",
          "Beim Bearbeiten `Beginn`, `Ende`, `Pause` oder `Schichtart` anpassen, um eine Schicht zu verlängern oder zu verkürzen. Im Feld `Grund der Änderung (optional)` lässt sich kurz festhalten, warum (z. B. Krankheit, Fortbildung, Tausch).",
          "Fällt eine Schicht aus, `Diese Schicht fällt aus` anhaken. Der Block erscheint danach durchgestrichen mit dem Vermerk `Fällt aus` samt Grund und zählt nicht mehr zur geplanten Zeit. Ohne Ersatzeintrag bleibt die Lücke bewusst offen.",
          "Für eine Vertretung stattdessen `+ Vertretung hinzufügen` wählen und eine Ersatzperson mit `Beginn` und `Ende` eintragen. Über mehrere Einträge lässt sich die Lücke auf mehrere Personen aufteilen; jede Vertretung erscheint am gewählten Tag als eigener Block der Ersatzperson mit blauem Vermerk `Vertretung`, die ausgefallene Person als schraffierter Block.",
          "Ausfall, Grund und Vertretungen werden zusammen gespeichert: Wird eine ausgefallene Schicht wieder aktiviert (Haken entfernen), entfernt die App die zugehörigen Vertretungen automatisch.",
          "Über das kleine Menü an einem Schicht-Block `Verschieben nach` wählen, um die Schicht ohne den Umweg über Löschen und Neuanlegen zu versetzen: Zielperson und Zieltag wählen, Zeiten und Schichtart bei Bedarf anpassen und bestätigen. Bei ausgefallenen Schichten steht diese Aktion nicht zur Verfügung.",
          "Für Abwesenheiten gibt es zwei getrennte Wege. Fällt eine Person für mehrere Tage aus, über das Menü am Namen `Krank melden` öffnen, Zeitraum und optional eine Notiz erfassen; die Krankmeldung storniert reguläre Schichten im Zeitraum automatisch (Vermerk `Fällt aus · Krankheit`) und markiert betroffene Betreuungsblöcke als abwesend, bereits eingetragene Vertretungsschichten bleiben bestehen und müssen manuell geprüft werden. `Zur Vertretung` führt anschließend direkt in die Tagesansicht des Vertretungsplans. Für eine einzelne kurzfristige Lücke an nur einem Tag stattdessen `Für heute abwesend melden` im selben Menü wählen: Diese Aktion legt keine Krankmeldung an, sondern springt sofort in die Tagesansicht des Vertretungsplans, wo Abwesenheit und Ersatz für diesen einen Tag erfasst werden. Ein halber Tag bei der Krankmeldung gilt für ein einzelnes Datum und ändert Dienst- und Betreuungsplan nicht automatisch.",
          "Unter dem Namen jedes Mitarbeiters steht die geplante Wochenzeit aus den Schichten der angezeigten Woche (Schichtdauer abzüglich Pause) als `geplant/Soll h`. Ist ein Arbeitszeitmodell hinterlegt, färbt sich die Zahl bei Unterdeckung rot und bei Überdeckung gelb; ohne Soll erscheint nur die geplante Summe. In Wochen ohne angelegte Schichten erscheint keine Summe.",
          "Die Mitarbeiter-/Tag-Zellen zeigen außerdem die konkreten Einsätze aus dem Betreuungsplan mit Uhrzeit, Aktivität und Raum. Diese Einsätze sind im Dienstplan schreibgeschützt; Änderungen erfolgen im Betreuungsplan oder bei kurzfristigen Abweichungen im Vertretungsplan. Deckt eine Schicht einen Einsatz nicht vollständig ab, markiert der Dienstplan den Einsatz und nennt den nicht abgedeckten Zeitraum als Hinweis, der das Speichern im Betreuungsplan nicht verhindert.",
          "Unterhalb des Rasters zählt die Zeile `Kapazität 12–16` pro Wochentag, wie viele Personen zwischen 12 und 16 Uhr eingeplant sind, als grobe Übersicht zur Kernzeit-Besetzung. Die Legende darunter löst die Schichtart-Farben und die Blockzustände auf: schraffiert für abwesend, durchgestrichen für ausgefallen, blau für Vertretung.",
          "Über `Halbjahr` zur Halbjahres-Sicht wechseln: Die Spalten sind die Kalenderwochen des laufenden Planungszeitraums, jede Zelle zeigt die Wochenstundensumme einer Person mit derselben Rot-/Gelb-Färbung wie die Wochenansicht. Ein Klick auf eine Zelle springt zurück in die Wochenansicht der angeklickten Woche; bearbeiten lässt sich hier nichts.",
          "Mitarbeitende sehen ihre eigene geplante Schicht in der Zeiterfassung als Zeile `Geplant: 08:00–16:00` in der Stempeluhr.",
        ],
        callout: {
          title: "Dienstplan und Arbeitszeitmodell nicht verwechseln",
          body: "Das Arbeitszeitmodell auf der Mitarbeiter-Detailseite legt die vertraglichen Soll-Stunden pro Wochentag fest und steuert das Stundenkonto. Der Dienstplan plant konkrete Uhrzeiten pro Datum und vergleicht die geplante Wochenzeit nur sichtbar mit diesem Soll. Beide existieren nebeneinander: Das Soll bleibt die Basis für den Saldo, der Dienstplan bestimmt die geplanten Anwesenheitszeiten.",
          tone: "blue",
        },
        screenshot:
          "Dienstplan-Wochenansicht mit Schicht-Blöcken, Betreuungsplan-Einsätzen, Wochensummen im Zeilenkopf, Kapazitätszeile und Legende sowie die Halbjahres-Sicht mit Kalenderwochen-Spalten.",
        gallery: [
          {
            image: "/help/screens/dienstplan.webp",
            caption:
              "Wochenansicht: pro Person die Schicht-Blöcke der Woche, im Zeilenkopf die geplante Wochenzeit im Verhältnis zum Soll, darunter die Kapazitätszeile und die Legende.",
          },
          {
            image: "/help/screens/dienstplan-halbjahr.webp",
            caption:
              "Halbjahres-Sicht: die Spalten sind die Kalenderwochen des Planungszeitraums, jede Zelle die Wochenstundensumme mit derselben Rot-/Gelb-Färbung; ein Klick springt in die Wochenansicht der Woche.",
          },
        ],
      },
      {
        id: "listen-aus-betreuungsslots",
        title: "Tageslisten",
        icon: ClipboardList,
        summary:
          "Erzeugt druckbare Tageslisten aus Angeboten, Ganztag und dokumentierter Anwesenheit.",
        steps: [
          "`Tageslisten` in der Seitenleiste öffnen.",
          "Als Quelle `Freie Angebotsauswahl`, eine Listenart wie `Mensa` oder `Lernzeit` oder eine Ganztagsliste wie `Ganztag bis 14:30` wählen.",
          "Datum prüfen (vorausgewählt ist heute).",
          "Unter `Freie Angebotsauswahl` ein oder mehrere Angebote des Tages auswählen. Ohne Einschränkung nutzt Phoenix alle aktiven Angebote des gewählten Datums. Bei einer Listenart nimmt Phoenix automatisch die Termine, denen diese Listenart im Stundenplan zugeordnet wurde.",
          "Datenbasis wählen: `Plan` (laut Tagesplanung), `Ist` (dokumentierte Anwesenheit am Datum) oder `Abgleich` (geplant, anwesend, fehlend, abgemeldet und ungeplant in einer Liste). `Abgemeldet` ist ein berechtigtes Fehlen und wird vom unklaren `Fehlt` getrennt gezählt.",
          "Bei Bedarf über die Filter weiter eingrenzen: `Gruppe` oder `Klasse`.",
          "Optional über `Gruppieren` die Liste in Abschnitte unterteilen, z. B. nach `Klasse`, `Angebot`, `Raum` oder `Abholzeit`. Die Abschnitte erscheinen auch im PDF und XLSX.",
          "Vorschau, Zähler und die Angaben zu Datum, Datenbasis, enthaltenen Angeboten und Gruppierung prüfen. Die Auswahl steht auch in der URL und kann später erneut geöffnet werden.",
          "Danach `Drucken`, `PDF herunterladen` oder `XLSX` wählen.",
        ],
        callout: {
          title: "Woher kommen die Daten?",
          body: "Tageslisten entstehen aus den ausgewählten Angeboten des Datums oder aus der am Termin hinterlegten Listenart. Abgesagte Angebote werden angezeigt, aber nicht in Tageslisten aufgenommen. Ganztagslisten entstehen aus den hinterlegten Abholzeiten und den unter Einstellungen -> Betrieb -> Stundenplan gepflegten Grenzen. Die Seite zeigt die Datenherkunft jeweils an. Die Notfallliste bleibt davon unabhängig.",
          tone: "blue",
        },
        screenshot:
          "Tageslisten-Seite mit Quellen-Auswahl, Listenarten, Datum, Datenbasis Plan/Ist/Abgleich, enthaltenen Angeboten, Gruppierung, Gruppen-/Klassen-Filter und Vorschau-Tabelle.",
      },
      {
        id: "zeiterfassung",
        title: "Zeiterfassung",
        icon: Clock3,
        summary:
          "Erfasst Arbeitszeit, Pausen, Arbeitsort und einfache Abwesenheiten.",
        steps: [
          "`Zeiterfassung` öffnen.",
          "`In der OGS`, `Homeoffice` oder `Abwesend` wählen.",
          "Mit `Einstempeln` beginnen und am Ende `Ausstempeln`.",
          "Wenn die Einrichtung Einstempeln erst ab geplanter Startzeit aktiviert hat, wird ein zu früher Versuch mit Hinweis auf die Startzeit abgewiesen.",
          "Ist im Dienstplan eine Schicht geplant, zeigt die Stempeluhr sie als `Geplant: 08:00–16:00`, bei hinterlegter Schichtart mit farbigem Kürzel (z. B. `Betreuung`). Eine Schicht als Vertretung ist mit `Vertretung` gekennzeichnet, eine ausgefallene Schicht als `Entfällt`. Vergessene Ausstempelungen kann die Einrichtung automatisch zum geplanten Dienstende beenden lassen (Einstellung `Automatische Ausstempelung`); solche Einträge sind mit `Auto-Checkout` markiert und lassen sich korrigieren.",
          "Wer früher als geplant kommt oder später als geplant geht, wird beim Stempeln nach einem kurzen Grund gefragt (z. B. `Bus verspätet`). Der Grund wird zur Nachvollziehbarkeit gespeichert. Die Einrichtung kann diese Abfrage in den Einstellungen abschalten.",
          "Die Karte `Heute geplant` listet die eigenen Betreuungsplan-Einsätze des Tages mit Uhrzeit, Aufgabe und Raum. Kurzfristige Änderungen sind gekennzeichnet: `Vertretung`, `Entfällt`, `Abwesend` oder `Unterbesetzt`. Pflegt die Einrichtung keinen Betreuungsplan, bleibt die Karte ausgeblendet.",
          "Pausen starten und bei Bedarf eine individuelle Dauer wählen. Nach Ablauf der gewählten Dauer läuft die Arbeitszeit automatisch weiter; über `Pause beenden` kann die Pause früher beendet werden.",
          "Bei langen Arbeitstagen die Pausenhinweise beachten.",
          "Für Krankheit, Fortbildung oder sonstige Abwesenheit `Abwesend` wählen und die Abwesenheit mit Art, Zeitraum und optionaler Notiz speichern.",
        ],
        callout: {
          title: "Arbeitsort bewusst wählen",
          body: "Die App setzt keinen Arbeitsort voraus. Vor dem Einstempeln muss bewusst `In der OGS` oder `Homeoffice` gewählt werden, damit die Zeiterfassung später eindeutig bleibt.",
          tone: "orange",
        },
        screenshot:
          "Zeiterfassung mit Einstempeln, individueller Pausenlänge, Ausstempeln und Abwesenheit melden.",
        image: "/help/screens/zeiterfassung.webp",
      },
      {
        id: "zeiterfassung-urlaub-historie",
        title: "Urlaub, Historie und Korrekturen",
        icon: CalendarRange,
        summary:
          "Zeigt Resturlaub, Anträge, Wochen- oder Monatsansichten und nachvollziehbare Korrekturen.",
        steps: [
          "In der Karte `Urlaub` Resturlaub, beantragte, genehmigte und abgelehnte Anträge prüfen.",
          "Mit `Urlaub beantragen` einen Zeitraum wählen. Halbe Tage, Notiz, Überschneidungen und Resturlaub werden direkt im Dialog geprüft.",
          "Eigene Urlaubsanträge in `Meine Anträge` verfolgen und offene oder zukünftige genehmigte Anträge bei Bedarf stornieren.",
          "In der Tabelle `Zeiterfassung` zwischen Woche und Monat wechseln und mit `Diese Woche` oder `Diesen Monat` zurückspringen.",
          "In der Monatsansicht die eigene `Monatskarte` prüfen: Übertrag aus dem Vormonat, Summe Soll, Summe Ist, Gutschriften für Krankheit und Urlaub sowie der Monatssaldo stehen dort zusammen.",
          "Tageszeilen prüfen: Plan (geplante Schicht), Check-in, Check-out, Pause, Soll, Ist, Saldo, Status, Quelle und Hinweise zeigen, ob ein Tag vollständig erfasst wurde.",
          "Gesetzliche Feiertage (nach dem Bundesland der Einrichtung) tragen das Badge `Feiertag`. An diesen Tagen ist das Soll automatisch 0, es entstehen also keine Minusstunden. Wird an einem Feiertag trotzdem gestempelt, erscheint der Hinweis `Feiertagsarbeit`.",
          "Über das Stift-Symbol eigene Arbeitszeiteinträge korrigieren oder fehlende Arbeitstage nachtragen. Bei jeder Arbeitszeit-Korrektur einen Grund angeben.",
          "Geänderte Tage aufklappen, um die Änderungshistorie zu sehen. Für Auswertungen den Export im Tabellenkopf nutzen.",
        ],
        callout: {
          title: "Urlaub und Krankheit unterscheiden",
          body: "Urlaub läuft über den Antragsbereich und kann genehmigt oder abgelehnt werden. Krank, Fortbildung und sonstige Abwesenheiten werden über `Abwesend` in der Stempeluhr gemeldet.",
          tone: "blue",
        },
        screenshot:
          "Zeiterfassung mit Urlaubskarte, Wochen- oder Monatsansicht, Änderungshistorie und Export.",
        printCompact: true,
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
          "Der Admin-Bereich für alle Stammdaten: Kinder, Personal, Räume, Gruppen, Rollen und Berechtigungen. `Aktivitäten` und `Geräte` werden zusätzlich angezeigt, wenn Ihre Einrichtung mit NFC oder Tablets arbeitet.",
        steps: [
          "`Datenverwaltung` öffnen.",
          "Den gewünschten Bereich wählen: `Kinder`, `Personal`, `Räume`, `Gruppen`, `Rollen` oder `Berechtigungen`.",
          "Wenn NFC oder Tablets genutzt werden, zusätzlich `Aktivitäten` und `Geräte` öffnen.",
          "Einträge anlegen, bearbeiten oder prüfen.",
          "Unter `Exporte` liegen alle Listen der Schule gebündelt, siehe nächster Abschnitt.",
        ],
        screenshot: "Datenverwaltung mit allen Bereichen und Eintragszahlen.",
        image: "/help/screens/datenverwaltung.webp",
      },
      {
        id: "listen-exportieren",
        title: "Listen exportieren",
        icon: Download,
        summary:
          "Der zentrale Einstieg für alle Listen der Schule: Kinderlisten, die Geburtstagsliste, die Notfallliste und die Raumbelegung - an einer Stelle statt verstreut über die einzelnen Seiten.",
        steps: [
          "`Datenverwaltung` -> `Exporte` öffnen.",
          "Unter `Kinderlisten` hat jede Liste eine eigene Karte (`OGS Wochenliste`, `Klassenliste`, `Tagesliste`, `Abholliste`, `Checkliste`, `Geburtstagsliste` und weitere). `Liste erstellen` öffnet den Export direkt mit der passenden Vorlage; dort legst du nur noch das Format fest. Die freie Kombination aus beliebiger Vorlage und einzeln zu- oder abgewählten Spalten findest du weiterhin in der `Kindersuche` unter `Exportieren`.",
          "Die Karte `Geburtstagsliste` gibt die Geburtstage nach Kalender sortiert aus. Voreingestellt ist der aktuelle Monat; über die Monatsfelder wählst du weitere Monate dazu oder ab, mit `Ganzes Jahr` erhältst du alle Geburtstage des Jahres.",
          "Unter `Momentaufnahmen` erzeugt `Notfallliste` die Liste aller aktuell anwesenden Kinder mit den Kontaktdaten der Erziehungsberechtigten, wahlweise als PDF oder direkt zum Drucken. `Wer ist wo` gibt die aktuelle Belegung aller Räume mit Aufsicht und Kinderzahl aus.",
          "Unter `Auf anderen Seiten` führen `Anmeldungen` und `Zeitnachweis` auf die Seite, zu der der jeweilige Export gehört: Anmeldungen werden je Anmeldephase exportiert, Zeitnachweise je Person.",
        ],
        callout: {
          title: "Kinder ohne Geburtsdatum fehlen in der Geburtstagsliste",
          body: "Die Liste zeigt nur Kinder, bei denen ein Geburtsdatum hinterlegt ist. Fehlt jemand, ergänze das Geburtsdatum in der Kinddetailansicht unter `Stammdaten`. Die Geburtstagsliste findest du auch in der `Kindersuche` unter `Exportieren` als Vorlage `Geburtstagsliste`, dann bezogen auf die gerade gefilterten Kinder, zum Beispiel nur die eigene Gruppe.",
          tone: "blue",
        },
        screenshot:
          "Exporte-Seite der Datenverwaltung mit den Abschnitten Kinderlisten, Momentaufnahmen und Auf anderen Seiten.",
        image: "/help/screens/exporte.webp",
      },
      {
        id: "anmeldungen-einrichten",
        title: "Anmeldungen einrichten",
        icon: LayoutDashboard,
        summary:
          "Der Admin-Bereich für die Online-Anmeldung: eingegangene Anmeldungen bearbeiten, Änderungsanfragen prüfen und in vier Unterseiten den Ablauf einrichten - `Überblick`, `Änderungsanfragen`, `Anmeldephasen`, `Betreuungsangebote` und `Anmeldeformulare`.",
        steps: [
          "`Anmeldungen` öffnen. Du landest im `Überblick` mit allen Anmeldephasen, der Zahl der Eingänge (`Gesamt`, `Offen`, `Bestätigt`, `Abgelehnt`) und dem Einstieg zu offenen Änderungsanfragen.",
          "Beim ersten Einrichten führt dich der Bereich `Einrichtung` (`Online-Anmeldung vorbereiten`) Schritt für Schritt durch alles Nötige. Zuerst `Online-Anmeldung aktivieren`: schaltet den Elternlink frei (in den `Einstellungen` unter `Anmeldung`).",
          "Unter `Einstellungen` -> `Anmeldung` -> `Rechtstexte` aktivierst du nur die Blöcke, die eure Einrichtung tatsächlich nutzt. Jeder Block hat denselben Ablauf: `Im Anmeldeformular anzeigen` einschalten, den Pflichttext im Dialog eintragen und speichern. Bei `AGB / Teilnahmebedingungen` wählst du zuerst die Quelle: `Text eingeben` oder `PDF-Datei hochladen`. Nur die gewählte Quelle erscheint im Elternformular; die andere Quelle kann gespeichert bleiben, wird aber nicht angezeigt. Ausgeschaltete Blöcke bleiben im Hintergrund gespeichert, erscheinen aber nicht im Elternformular. Eigene Formularvorlagen können diese Standardblöcke unter `Rechtstexte und Einwilligungen` je Vorlage ein- oder ausblenden, abweichend bearbeiten oder um eigene Einwilligungen ergänzen.",
          "Für Familien, die die Frist verpasst haben, `Anmeldephasen` öffnen und in der passenden Phase im Drei-Punkte-Menü `Nachzügler-Link erstellen` wählen. E-Mail-Adresse der erziehungsberechtigten Person eintragen, optional einen internen Grund notieren und den erzeugten Link an die Familie schicken. Der Link öffnet genau diese Phase trotz geschlossener Frist und kann nur einmal erfolgreich genutzt werden.",
          "Als letzte Absicherung kann ein Admin unter `Anmeldephasen` in der passenden Phase im Drei-Punkte-Menü `Manuelle Anmeldung` wählen. Dort wird dieselbe Formularvorlage wie für Eltern geladen; nach interner Begründung und Bestätigung, dass die Einwilligung extern vorliegt, wird das Kind direkt angelegt und freigegeben.",
          "Unter `Einstellungen` -> `Anmeldung` legen Sie außerdem fest, ob Klassenstufe und Betreuungsangebote abgefragt werden. Ist die Klassenstufe ausgeschaltet, bleibt auch die konkrete Klasse wirkungslos. Ausgeschaltete Betreuungsangebote werden in Eltern-, Bearbeitungs-, Vorschau- und manuellen Formularen nicht angezeigt; der Angebotskatalog bleibt erhalten.",
        ],
        callout: {
          title: "So hängt alles zusammen",
          body: "Alles hängt an der `Anmeldephase`: Sie legt den Zeitraum und das Anmeldefenster fest. `Betreuungsangebote` gehören zu einer Phase, und jede Phase nutzt ein `Anmeldeformular`. Richte deshalb in dieser Reihenfolge ein: zuerst die `Online-Anmeldung` in den Einstellungen aktivieren (sonst ist der Elternlink nicht erreichbar), dann eine Anmeldephase anlegen, danach die Betreuungsangebote, bei Bedarf ein eigenes Formular - am Ende die Elternansicht testen. Für Nachzügler bleibt die Phase geschlossen; du erzeugst nur einen einzelnen Sonderlink oder nutzt die manuelle Freigabe.",
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
          "Eingegangene Anmeldungen öffnen, Angaben prüfen, nach Betreuungsangeboten filtern und die passende Entscheidung setzen.",
        steps: [
          "Bei einer Phase auf `Anmeldungen ansehen` klicken, um die eingegangenen Anmeldungen zu prüfen.",
          "Mit `Status`, den `Angeboten für die Auswertung`, der `Anzahl Betreuungstage`, `Zielklasse`, `Wochentag`, `Gehzeit` oder der Suche die Tabelle auf die Kinder eingrenzen, die du brauchst.",
          "Die Kennzahlen über der Tabelle zeigen, wie viele Kinder an einem, zwei, drei, vier oder fünf Tagen betreut werden. Die Karte `Einsatzplanung` zeigt zusätzlich, wie viele Kinder je Wochentag bis zu welcher Gehzeit bleiben.",
          "Für Klassenlehrkräfte unter `Klasse für Klassenliste` den Klassenverband wählen und `Klassenliste exportieren` nutzen. Mit `Alle Klassen` erhältst du alle Klassenlisten in einer Datei, sauber getrennt mit eigener Überschrift je Klasse (im PDF auf einer neuen Seite). Die Liste enthält den gesamten Klassenverband, auch Kinder ohne bestätigte Anmeldung, und zeigt pro Wochentag die gebuchten Angebote, zum Beispiel `Randstunde` oder `Ganztag`, inklusive Abholzeit, Geh-/Abholweise und Kontaktdaten der Erziehungsberechtigten.",
          "Eine Anmeldung öffnen und Kind, erziehungsberechtigte Personen (Hauptkontakt und weitere erziehungsberechtigte Personen), gewähltes Betreuungsangebot und Formularangaben prüfen.",
          "Wenn eine Familie nach der Frist nachgereicht hat, erscheint die Anmeldung nach Nutzung des Nachzügler-Links ganz normal in dieser Liste. Bei der manuellen Freigabe ist das Kind bereits bestätigt; prüfe anschließend bei Bedarf den Statuslink oder die Kinddetailseite.",
          "Mit `Bestätigen`, `Warteliste` oder `Ablehnen` entscheiden; mit `Zur Prüfung` für später vormerken. Ist die Warteliste in den Einstellungen deaktiviert, wird diese Aktion nicht angeboten. Nach einer Bestätigung lässt sich eine erziehungsberechtigte Person bei Bedarf in der Kinddetailseite manuell einladen oder erneut einladen.",
          "Fallen nach der Bestätigung falsche Kerndaten auf, öffne die Anmeldung und wähle beim bestätigten Kind `Anmeldedaten korrigieren`. Berichtige Name, Geburtsdatum, Ziel-Klassenstufe oder Zielklasse und gib einen Grund an. moto aktualisiert damit die Anmeldung und die verknüpften Stammdaten gemeinsam und protokolliert die Korrektur. Ändere in diesem Fall nicht zuerst die Stammdaten, weil die Anmeldung die Quelle für Anmeldestatistiken bleibt.",
          "Bei bestätigten Kindern können Betreuungsangebote über `Betreuungsangebote bearbeiten` nachträglich korrigiert werden, solange `Betreuungsangebote anbieten` unter `Einstellungen` -> `Anmeldung` aktiviert ist. Eine Begründung ist Pflicht; die Änderungshistorie bleibt auch nach dem Ausschalten sichtbar und zeigt, wer was wann angepasst hat.",
          "Wenn Eltern nach einer Entscheidung Daten korrigieren, erscheint die Anfrage unter `Änderungsanfragen`. Die Änderungsübersicht zeigt pro Kind oder erziehungsberechtigter Person, welche Felder von `Bisher` auf `Neu` geändert wurden. Dort kannst du Rückfragen senden, die Änderung freigeben oder mit Begründung ablehnen.",
          "Über `Elternansicht öffnen` jederzeit prüfen, was Familien gerade sehen.",
        ],
        callout: {
          title: "So zählt moto Betreuungstage",
          body: "Die Anzahl Betreuungstage ist die Summe der unterschiedlichen Wochentage pro Kind aus den aktuell berücksichtigten Angeboten. Standardmäßig zählt moto nur Angebote, bei denen `Als Betreuungstage zählen` aktiv ist. Angebote wie eine Randstunde können sichtbar bleiben und automatisch mitgebucht werden, ohne die Betreuungstage zu erhöhen.",
          tone: "blue",
        },
        screenshot:
          "Anmeldephase mit Eingangsliste, Filtern, Kennzahlen nach Betreuungstagen und Entscheidungsoptionen.",
        image: "/help/screens/anmeldungen.webp",
      },
      {
        id: "anmeldungen-exportieren",
        title: "Anmeldungen exportieren",
        icon: Download,
        summary:
          "Alle Anmeldungen einer Phase kompakt als Datei sichern, zum Ausdrucken oder als Archiv, damit die wichtigsten Angaben auch bei WLAN- oder Systemausfall offline verfügbar sind.",
        steps: [
          "Eine `Anmeldephase` öffnen - oben rechts findest du den Button `Export`.",
          "`Export` öffnen und das gewünschte Format wählen: `PDF`, `Word-Dokument` oder `Excel-Datei`.",
          "`PDF` und `Word-Dokument` erzeugen eine gut lesbare Datei mit einem Block pro Kind, gruppiert nach Status und innerhalb jeder Gruppe alphabetisch nach Nachname, inklusive Kontaktdaten, gewählten Angeboten, Zustimmungen und allen Formularangaben.",
          "`Excel-Datei` erzeugt eine Tabelle mit Gruppenzeilen pro Status, einer Datenzeile pro Kind und jedem Feld in einer eigenen Spalte - für Weiterverarbeitung oder Archiv.",
          "Über das `Status`-Auswahlfeld nur einen Teil exportieren (zum Beispiel nur `Bestätigt`); der Export übernimmt den gerade gewählten Status. `Alle` exportiert alles.",
          "Für eine Auswertung nach Betreuungsangeboten, Betreuungstagen, Wochentag oder Gehzeit nutzt du die Filter und die Karte `Auswertung exportieren`; dort wählst du `Excel`, `PDF` oder `Word-Dokument` für genau diese gefilterte Ansicht. PDF und Word beginnen mit einer Einsatzplanung nach Wochentag und Gehzeit.",
          "Für die Übergabe an Klassenlehrkräfte nutzt du in der Phase `Klasse für Klassenliste` und `Klassenliste exportieren`; diese Liste ist pro Klasse aufgebaut und zeigt pro Wochentag die bestätigten Angebote der Phase, Abholzeiten, Geh-/Abholweise und Erziehungsberechtigte. Mit `Alle Klassen` exportierst du alle Klassenlisten in einer Datei, mit eigener Überschrift je Klasse und im PDF auf jeweils einer neuen Seite.",
        ],
        callout: {
          title: "Vertrauliche Daten - sorgsam aufbewahren",
          body: "Die Datei enthält alle Kontakt- und Kinderdaten der Phase gebündelt an einem Ort. Jede erzeugte Datei trägt einen Vertraulichkeitshinweis in der Fußzeile. Drucke und Dateien bitte sicher verwahren und nicht unkontrolliert weitergeben.",
          tone: "orange",
        },
        screenshot: "Anmeldephase mit Export-Menü sowie dem Status-Filter.",
        image: "/help/screens/anmeldungen-exportieren.webp",
      },
      {
        id: "anmeldungen-bereinigen",
        title: "Fehlerhafte Anmeldungen sicher löschen",
        icon: CircleStop,
        summary:
          "Abgelehnte, zurückgezogene oder nicht mehr benötigte Anmeldungen kontrolliert entfernen, ohne bestehende Kinder oder Elternkonten mitzulöschen.",
        steps: [
          "Die betroffene Anmeldung öffnen. Für ein einzelnes abgelehntes oder zurückgezogenes Kind `Kind aus Anmeldung löschen` wählen. Ein bestätigtes Kind kann erst bereinigt werden, nachdem sein Kind-Datensatz separat in der Datenverwaltung gelöscht wurde.",
          "Für die gesamte Anmeldung unten in der Seitenleiste `Anmeldung löschen` wählen.",
          "Die Auswirkungs-Vorschau vollständig prüfen. Sie zeigt die zu löschenden Anmeldedaten und weist auf Erziehungsberechtigtenprofile oder Elternkonten hin, die ausdrücklich erhalten bleiben.",
          "Wenn ein bestehender Kind-Datensatz verknüpft ist, dem Link zur Kindverwaltung folgen und dort den eigenen Löschablauf verwenden. Die Anmeldung kann erst danach gelöscht werden.",
          "Einen kurzen, sachlichen Löschgrund eintragen und die Löschung bestätigen. Bleibt bei einer Mehrkind-Anmeldung noch mindestens ein Kind übrig, bleiben die gemeinsamen Anmeldedaten erhalten.",
        ],
        callout: {
          title: "Keine Sammellöschung von Personen",
          body: "moto löscht über diesen Ablauf weder bestehende Kind-Datensätze noch Erziehungsberechtigtenprofile oder Elternkonten. Diese Datensätze haben eigene, getrennte Löschabläufe.",
          tone: "orange",
        },
        screenshot:
          "Löschdialog einer Anmeldung mit Auswirkungs-Vorschau, Schutz bestehender Kind-Datensätze und Pflichtfeld für den Löschgrund.",
        image: "/help/screens/anmeldung-loeschen.png",
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
          "`Verhalten bei voller Betreuung` festlegen und mit `Aktiv` die Phase für Eltern sichtbar machen. Ist die Warteliste tenantweit ausgeschaltet, nimmt `Warteliste` bei Überbelegung weitere Anmeldungen ohne Wartelistenstatus an.",
          "Unter `Konkrete Klassen` die auswählbaren Klassen (zum Beispiel 2a, 2b, 3a) pflegen und festlegen, ob die Angabe ab der 2. Klasse verpflichtend ist. Für die 1. Klasse wird weiterhin nur die Klassenstufe abgefragt. Wirksam nur, wenn `Konkrete Klasse abfragen` in den `Einstellungen` unter `Anmeldung` aktiviert ist.",
        ],
        callout: {
          title: "Das Anmeldefenster steuert die öffentliche Anmeldung",
          body: "Das `Anmeldefenster` der Phase entscheidet, wann Familien absenden können. Über das Aktionsmenü öffnest du mit `Formular ansehen` den Elternlink, wechselst mit `Anmeldungen ansehen` zu den Eingängen, erstellst einen `Nachzügler-Link`, startest eine `Manuelle Anmeldung` oder bereitest mit `Anschlussphase erstellen` eine Folgephase vor.",
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
          "`Name`, `Beschreibung` und die möglichen `Wochentage` festlegen. Die Wochentage sind nicht vorausgewählt: mindestens ein Tag muss aktiv angeklickt werden, und nur die gewählten Tage sind später für Eltern auswählbar - ein Angebot nur für Montag darf also auch nur `Mo` gesetzt haben.",
          "Unter `Regeltermin` den passenden Regeltermin aus dem Betreuungsplan verknüpfen, wenn genehmigte Anmeldungen dort erwartet werden sollen. Für jeden angebotenen Wochentag muss die Regeltermin-Serie den gesamten Betreuungszeitraum der Anmeldephase lückenlos abdecken. Jeder Termin braucht eine vollständige Uhrzeit mit Ende und einen wirksamen Raum; eine Raum-Ausnahme gilt nur für ihr konkretes Datum. Bei einem aktiven Angebot müssen außerdem alle verwendeten Planungszeiträume aktiv sein.",
          "Ohne Verfügbarkeitsregel gilt das Angebot für alle Klassenstufen. Optional `Nur unter Bedingungen anbieten` aktivieren, `Klassenstufe des Kindes` als Quelle wählen und mit `ist eine von` oder `ist keine von` auf die gewünschten Klassenstufen eingrenzen. Bei mehreren Bedingungen festlegen, ob alle oder mindestens eine erfüllt sein muss.",
          "Unter `Betreuungstage & Mitbuchung` festlegen, ob das Angebot als Betreuungstage zählt und ob es mitgebucht wird, wenn Eltern bestimmte andere Angebote wählen.",
          "Bei der Mitbuchung die auslösenden Angebote auswählen und optional auf Klassenstufen eingrenzen. Diese Klassenstufen steuern nur die automatische Mitbuchung; sie sind von den Bedingungen für die Verfügbarkeit getrennt.",
          "Optional `Kapazität`, `Preis in Cent` sowie `Mittagessen` oder `Ferienbetreuung` ergänzen.",
          "`Aktiv` setzen - nur aktive Angebote sind für Eltern auswählbar.",
        ],
        callout: {
          title: "Anmeldung und Betreuungsplan verbinden",
          body: "Eltern wählen weiterhin nur Angebot und Tage. Phoenix übernimmt genehmigte Kinder in die verknüpfte Regeltermin-Serie und materialisiert sie dort an den passenden Angebotstagen. Eine Verknüpfung lässt sich nur speichern, wenn jeder angebotene Wochentag über den vollständigen Betreuungszeitraum lückenlos mit vollständiger Uhrzeit, wirksamem Raum und aktivem Planungszeitraum geplant ist. Solange ein aktives oder bereits gewähltes Angebot davon abhängt, können Änderungen am Phasenzeitraum sowie das Ändern oder Löschen verwendeter Uhrzeiten und Räume abgelehnt werden. Andernfalls zuerst die Serie, ihre Planungszeiträume oder den Betreuungszeitraum der Anmeldephase korrigieren. Hinweise zeigen außerdem, wenn erwartete Ankunft oder Klassengruppe nicht zusammenpassen.",
          tone: "blue",
        },
        screenshot:
          "Bearbeitungsformular eines Betreuungsangebots mit einem zum Betreuungszeitraum passenden Regeltermin.",
        image: "/help/screens/betreuungsangebote.webp",
      },
      {
        id: "anmeldeformulare",
        title: "Anmeldeformulare",
        icon: FileText,
        summary:
          "Das Anmeldeformular bestimmt, welche Angaben Eltern machen. Das `Basisformular` ist immer vorhanden; eigene Vorlagen ergänzen nur zusätzliche Fragen.",
        steps: [
          "`Anmeldeformulare` öffnen. Das `Basisformular` fragt Elternteil und Kind sowie - abhängig von den Einstellungen - Klassenstufe und Betreuungsangebot ab.",
          "Nur bei zusätzlichem Bedarf über `Neue Vorlage` eine eigene Formularvorlage mit Zusatzfragen anlegen. Für Heimwege den Stammdaten-Vorschlag `Erlaubte Heimwege` nutzen: Eltern sehen bei Betreuungsangeboten mit Tagesauswahl nur die gewählten Betreuungstage und wählen pro Betreuungstag mindestens einen Heimweg, zum Beispiel zu Fuß, Bus, Abholung oder mit anderem Kind. Bei `mit anderem Kind` ergänzen Eltern im Pflichtfeld, mit wem das Kind nach Hause geht.",
          "Bei `Abholzeiten` kannst du optional `Feste Auswahlzeiten` hinterlegen. Ohne Zeiten geben Eltern die Uhrzeit frei ein; sobald Zeiten hinterlegt sind, wählen Eltern pro Wochentag nur noch aus dieser Liste.",
          "Den Namen einer Vorlage änderst du entweder beim `Bearbeiten` direkt oben im Editor oder über das Aktionsmenü (`⋮`) -> `Umbenennen`; dort kannst du eine Vorlage auch `Löschen`. Der Name gilt für alle Versionen der Vorlage; bereits abgeschickte Anmeldungen bleiben unverändert.",
          "Im Abschnitt `Rechtstexte und Einwilligungen` legst du je Vorlage fest, welche Zustimmungen und Hinweise Eltern sehen: Die Standardblöcke kommen aus den Einstellungen, können je Vorlage per Schalter ein- oder ausgeblendet und über das Stift-Symbol abweichend bearbeitet werden. Bei `AGB / Teilnahmebedingungen` wählst du in der Vorlage wie in den Einstellungen zwischen `Text eingeben` und `PDF-Datei hochladen`; diese Auswahl gilt nur für diese Formularvorlage. Über `Eigene Zustimmung hinzufügen` ergänzt du zusätzliche Einwilligungen, etwa für Ausflüge oder Schwimmbadbesuche.",
          "Mit `Vorschau` prüfen, wie das Formular für Eltern aussieht.",
          "Die Vorlage wirkt erst, wenn du sie in einer `Anmeldephase` als Formular auswählst.",
        ],
        callout: {
          title: "Formular und Phase gehören zusammen",
          body: "Ein Formular wirkt nicht für sich allein: Eine Phase nutzt entweder das `Basisformular` oder eine ausgewählte Vorlage. Ohne ausdrückliche Auswahl gilt automatisch das Basisformular. Die Rechtstexte werden mit der Vorlage gespeichert: Spätere Änderungen an den Rechtstexten in den Einstellungen übernimmst du in der Vorlage manuell. Eltern öffnen längere Rechtstexte im Formular über `Details`. Wenn du die `Datenschutzinformation` in einer Vorlage deaktivierst, stelle sicher, dass Eltern die Datenschutzhinweise auf anderem Weg erhalten, etwa über den Elternbrief.",
          tone: "blue",
        },
        screenshot: "Anmeldeformulare mit Basisformular und eigenen Vorlagen.",
        image: "/help/screens/anmeldeformulare.webp",
      },
      {
        id: "nachrichten",
        title: "Nachrichten",
        icon: MessageSquare,
        summary:
          "Der zentrale Posteingang für die Kommunikation mit den Eltern, wie ein Chat. Mit jeder Bezugsperson läuft pro Kind genau eine fortlaufende Unterhaltung (ohne Betreff); so wird die E-Mail-Kommunikation überflüssig.",
        steps: [
          "In der Seitenleiste den Bereich `Eltern` aufklappen und `Nachrichten` öffnen. Ein rotes Abzeichen am Bereich `Eltern` zeigt die Zahl der ungelesenen Eltern-Nachrichten (und offenen Anfragen); aufgeklappt steht es direkt am Punkt `Nachrichten`.",
          "Der Posteingang listet alle Unterhaltungen, die du sehen darfst (als Admin alle, sonst die Kinder deiner Gruppen), neueste zuerst. Jede Zeile zeigt die Bezugsperson mit Beziehung zum Kind und die letzte Nachricht. Über `Nur ungelesen` lässt sich die Liste eingrenzen.",
          "Eine Zeile öffnet das Chat-Fenster mit dem kompletten Verlauf. Über `Zum Kinderprofil` gelangst du von dort zur Kinderdetailansicht.",
          "Im Chat direkt antworten: Text eingeben und auf `Senden` tippen.",
          "Über `Neue Nachricht` selbst eine Unterhaltung starten: Kind suchen und Bezugsperson wählen. Damit öffnet sich das Chat-Fenster; den eigentlichen Text schreibst du dort und tippst auf `Senden`. Gibt es mit der Person schon eine Unterhaltung, wird sie fortgesetzt.",
          "Antworten erscheinen sofort in der Eltern-App der jeweiligen Bezugsperson; dort als `OGS` der Schule, ohne einzelnen Mitarbeitenden-Namen.",
          "Neben Nachrichten erscheinen im Verlauf auch automatische Hinweise, etwa wenn Eltern eine Krankmeldung abgeben, eine Abholzeit für einen Tag ändern oder eine Änderungsanfrage stellen. Diese Einträge sind reine Information ohne Schaltflächen; Anfragen bearbeitest du als Admin unter `Änderungsanfragen` (siehe nächster Abschnitt).",
        ],
        callout: {
          title: "Voraussetzung",
          body: "Die Funktion muss unter `Einstellungen` > `Betrieb` > `Eltern-OGS-Nachrichten` aktiviert sein. Jede Bezugsperson sieht nur ihre eigenen Unterhaltungen.",
          tone: "blue",
        },
        screenshot:
          "Nachrichten-Posteingang als Unterhaltungs-Liste mit Bezugsperson, Beziehung, letzter Nachricht und Ungelesen-Abzeichen.",
        image: "/help/screens/nachrichten.webp",
      },
      {
        id: "eltern-anfragen",
        title: "Anfragen der Eltern bearbeiten",
        icon: ClipboardCheck,
        summary:
          "Bezugspersonen können über die Eltern-App strukturierte Anfragen stellen: Änderungen an den Stammdaten (Name, Geburtsdatum, Gehzeiten) und an den dauerhaften Betreuungszeiten. Anfragen entstehen im Elternportal auf der Stammdaten-Seite des Kindes und werden zentral auf der Seite `Änderungsanfragen` entschieden.",
        steps: [
          "Eltern sehen im Elternportal die aktuellen Betreuungszeiten ihres Kindes (Bringzeit, Abholzeit, Abholart je Wochentag) und reichen dort über `Änderung anfragen` einen Vorschlag ein.",
          "Neue Anfragen erscheinen im Nachrichten-Verlauf des Kindes als Hinweis, sind dort aber nicht bedienbar.",
          "In der Seitenleiste den Bereich `Eltern` aufklappen und `Änderungsanfragen` öffnen. Dort stehen alle offenen Anfragen, getrennt nach `Stammdaten` und `Betreuungszeiten`, jeweils mit dem Vergleich `aktuell -> gewünscht`.",
          "Mit `Freigeben` wird die Änderung übernommen: Bei Betreuungszeiten wird der Wochenplan des Kindes direkt aktualisiert.",
          "Passt die Anfrage nicht, eine kurze `Begründung` eintragen und auf `Ablehnen` tippen. Der Grund wird der Bezugsperson angezeigt.",
          "Nach der Entscheidung wird die Bezugsperson in ihrer App über das Ergebnis informiert (Hinweis im Nachrichten-Verlauf und Status auf der Stammdaten-Seite).",
        ],
        callout: {
          title: "Wer darf entscheiden",
          body: "Die Seite `Änderungsanfragen` steht Admins sowie Mitarbeitenden mit Bearbeitungsrecht für die Kinderdaten zur Verfügung. Die für die Gruppe eines Kindes zuständige Aufsicht sieht dabei nur Anfragen zu Kindern aus ihren eigenen Gruppen. Solange eine Anfrage offen ist, können Eltern sie auf der Stammdaten-Seite zurückziehen.",
          tone: "orange",
        },
        screenshot:
          "Admin-Seite Änderungsanfragen, Bereich Betreuungszeiten: Wochenplan-Vergleich aktuell zu gewünscht mit den Schaltflächen Freigeben und Ablehnen.",
        image: "/help/screens/offene-anfragen.webp",
      },
      {
        id: "elternmitteilungen",
        title: "Elternmitteilungen",
        icon: Megaphone,
        summary:
          "Mitteilungen an ausgewählte Elterngruppen senden (Rundinformationen statt Einzelnachrichten). Eltern sehen sie als Neuigkeiten im Elternportal. Anders als die Nachrichten ist das eine Einbahn-Information ohne Antwortmöglichkeit; für Rückfragen nutzen Eltern den normalen Nachrichten-Chat.",
        steps: [
          "In der Seitenleiste den Bereich `Eltern` aufklappen, `Elternmitteilungen` öffnen und auf `Elternmitteilung` tippen.",
          "Schritt `Inhalt`: Titel und Text eingeben. Optional: einen Link ergänzen, Priorität `Wichtig` setzen, ein Ablaufdatum wählen (danach wird die Mitteilung ausgeblendet), `Lesebestätigung erforderlich` und `Eltern zusätzlich per E-Mail benachrichtigen` aktivieren. Die E-Mail enthält nur den Titel und einen Link ins Elternportal.",
          "Schritt `Empfänger`: Zielgruppe wählen: ganze Schule, einzelne Klassen, Gruppen, AGs/Betreuungsangebote, einzelne Kinder oder Eltern mit offener Anmeldung. Mehrere Zielgruppen lassen sich kombinieren; ein Elternteil erhält die Mitteilung höchstens einmal.",
          "Mit `Als Entwurf speichern` für später sichern oder mit `Veröffentlichen` direkt an die Eltern geben. Veröffentlichte Mitteilungen erscheinen sofort im Elternportal der erreichten Eltern.",
          "Entwürfe lassen sich aus der Liste bearbeiten und über `Veröffentlichen` freigeben. Nach dem Veröffentlichen ist keine Bearbeitung mehr möglich; über `Zurückziehen` wird eine Mitteilung wieder zum Entwurf und aus dem Elternportal entfernt.",
          "Ein Tipp auf eine Mitteilung öffnet die Detailansicht mit dem vollständigen Text und der Statistik: wie viele der erreichten Eltern sie gelesen und (falls verlangt) bestätigt haben, inklusive Liste, welche Bezugsperson noch aussteht.",
        ],
        callout: {
          title: "Voraussetzung",
          body: "Die Funktion muss unter `Einstellungen` > `Betrieb` > `Elternmitteilungen (Neuigkeiten)` aktiviert sein. Eltern sehen nur Mitteilungen, die für ihre eigenen Kinder bestimmt sind.",
          tone: "blue",
        },
        screenshot:
          "Übersicht der Elternmitteilungen mit Status (Entwurf, veröffentlicht, abgelaufen) und der Aktion „Neue Elternmitteilung“.",
      },
      {
        id: "essensplan",
        title: "Essensplan",
        icon: UtensilsCrossed,
        summary:
          "Die Woche als Plan: Montag bis Freitag nebeneinander, pro Tag ein oder mehrere Gerichte mit optionalem Hinweis. Eltern sehen den Plan für die aktuelle und nächste Woche im Elternportal.",
        steps: [
          "In der Seitenleiste den Bereich `Eltern` aufklappen und `Essensplan` öffnen.",
          "Mit den Pfeilen `‹` und `›` zwischen den Kalenderwochen blättern; die laufende Woche ist mit `Diese Woche` markiert, mit `Heute` springst du dorthin zurück.",
          "Pro Tag das `Gericht` eintragen; bei Bedarf einen kurzen `Hinweis` ergänzen (z. B. vegetarisch).",
          "Mehrere Gerichte pro Tag über `+ Gericht` hinzufügen (z. B. Menü 1 und Menü 2); überflüssige Zeilen mit dem `×` entfernen.",
          "Über das Tagesmenü (`⋯`) einen Tag `kopieren`, in einen anderen Tag `einfügen` oder `leeren`; mit `Vorwoche übernehmen` den kompletten Plan der Vorwoche übernehmen.",
          "Änderungen unten mit `Speichern` sichern (ungespeicherte Änderungen werden angezeigt und beim Verlassen abgefragt).",
        ],
        callout: {
          title: "Voraussetzung",
          body: "Die Funktion muss unter `Einstellungen` > `Betrieb` > `Essensplan` aktiviert sein. Ist sie aus, erscheint der Punkt weder im Team- noch im Elternportal.",
          tone: "blue",
        },
        screenshot:
          "Essensplan als Wochen-Board (Mo–Fr nebeneinander), oben Wochennavigation mit Pfeilen und „Heute“, je Tag eine Liste von Gerichten mit Hinweis.",
        image: "/help/screens/essensplan.webp",
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
  {
    id: "info-displays",
    title: "Info-Displays",
    description:
      "Ein Info-Display ist ein Dashboard für große Bildschirme im Eingangsbereich: Es zeigt live die Raumbelegung, laufende und kommende Aktivitäten sowie die nächsten Abholzeiten (nur als Anzahl, ohne Kindernamen). Es läuft in jedem Browser, ein Login am Fernseher ist nicht nötig.",
    icon: MonitorPlay,
    tone: "blue",
    steps: [
      {
        id: "info-displays-erstellen",
        title: "Display erstellen und am Fernseher einrichten",
        icon: MonitorPlay,
        summary:
          "Admins erstellen pro Bildschirm ein Display und erhalten dafür einen geheimen Link. Der Link wird einmal am Fernseher oder Smartboard geöffnet, danach aktualisiert sich das Dashboard von selbst.",
        steps: [
          "In der Seitenleiste `Info-Displays` öffnen und `Neues Display` klicken.",
          "Einen Namen vergeben, der den Standort beschreibt, z. B. `Eingangsbereich`.",
          "Nach dem Erstellen erscheint der Link genau einmal: jetzt kopieren oder den QR-Code direkt mit dem Zielgerät scannen.",
          "Den Link im Browser des Fernsehers oder Smartboards öffnen. Fertig — das Dashboard lädt seine Daten automatisch alle paar Sekunden neu.",
        ],
        callout: {
          title: "Link geheim halten",
          body: "Wer den Link kennt, sieht das Dashboard. Es zeigt bewusst keine Kindernamen, sondern nur Zahlen und Raum- bzw. Aktivitätsnamen. Trotzdem gilt: den Link nur auf Geräten der OGS öffnen und bei Verdacht auf Weitergabe einfach einen neuen Link erstellen.",
          tone: "blue",
        },
        screenshot:
          "Seite Info-Displays mit der Display-Liste und dem Button Neues Display.",
        image: "/help/screens/info-displays.webp",
      },
      {
        id: "info-displays-verwalten",
        title: "Link erneuern, Display deaktivieren oder löschen",
        icon: KeyRound,
        summary:
          "Jedes Display lässt sich jederzeit umbenennen, vorübergehend deaktivieren oder ganz entfernen. Der alte Link wird dabei sofort ungültig.",
        steps: [
          "In der Zeile des Displays das Menü (⋮) öffnen.",
          "`Neuen Link erstellen` wählen, wenn der alte Link verloren ging oder in falsche Hände geraten sein könnte. Der neue Link wird wieder genau einmal angezeigt, der alte funktioniert sofort nicht mehr.",
          "`Deaktivieren` blendet das Dashboard aus, ohne das Display zu löschen — der Bildschirm zeigt dann nur noch einen Hinweis. `Aktivieren` schaltet es wieder ein.",
          "`Löschen` entfernt das Display dauerhaft; am Fernseher muss danach ein neuer Link eines anderen Displays geöffnet werden.",
        ],
        screenshot:
          "Zeilenmenü eines Displays mit den Aktionen Umbenennen, Neuen Link erstellen, Deaktivieren und Löschen.",
      },
    ],
  },
  {
    id: "einstellungen",
    title: "Einstellungen",
    description:
      "Hier stellen Admins ein, wie sich die App im Alltag verhält. Manche Funktionen (etwa die Online-Anmeldung oder die Aktivitäts-Indikatoren) sind erst sichtbar, wenn sie hier eingeschaltet wurden; andere wie der Betreuungsplan lassen sich hier abschalten.",
    icon: SlidersHorizontal,
    tone: "gray",
    steps: [
      {
        id: "einstellungen-ueberblick",
        title: "Einstellungen im Überblick",
        icon: SlidersHorizontal,
        summary:
          "Die Einstellungen sind in Reiter (Tabs) gegliedert. Jeder Reiter bündelt Optionen zu einem Thema. Änderungen werden automatisch gespeichert.",
        steps: [
          "In der Seitenleiste `Einstellungen` öffnen.",
          "Oben den passenden Reiter wählen: `Betrieb` (Alltagsverhalten, z. B. Betreuungsplan oder Aktivitäts-Indikatoren), `Geräte` (NFC-Tablets, PIN, Auswahl-Buttons), `Anmeldung` (Online-Anmeldung der Eltern), `Datenschutz` (Aufbewahrung und Sichtbarkeit von Daten), `Sicherheit` sowie `Personalisierung` (Erscheinungsbild).",
          "Schalter (an/aus) und Auswahlfelder werden sofort gespeichert; Text-, Zahl- und Zeitfelder kurz nach der Eingabe. Ein grüner Rahmen bestätigt das Speichern, ein roter weist auf einen Fehler hin.",
          "Steht neben einer Einstellung das Abzeichen `Standard`, ist noch der voreingestellte Wert aktiv. Nach einer Änderung erscheint `Zurücksetzen`, um wieder den Standard herzustellen.",
        ],
        screenshot:
          "Einstellungen mit der Reiter-Leiste (Betrieb, Geräte, Anmeldung, Datenschutz, System, Sicherheit, Personalisierung) und der Sektion Aktivitäts-Indikatoren im Reiter Betrieb.",
        image: "/help/screens/einstellungen.webp",
      },
      {
        id: "einstellungen-indikatoren",
        title: "Aktivitäts-Indikatoren einrichten",
        icon: ClipboardCheck,
        summary:
          "Aktivitäts-Indikatoren zeigen in der `Kindersuche` und in den Gruppenansichten mit einem Haken, ob ein Kind heute bereits in einem bestimmten Bereich war, zum Beispiel in der Mensa oder bei den Hausaufgaben. Standardmäßig ist die Funktion aus.",
        steps: [
          "`Einstellungen` -> `Betrieb` öffnen und zur Sektion `Indikatoren` scrollen.",
          "`Aktivitäts-Indikatoren` einschalten.",
          "In `Indikator 1` bis `Indikator 3` jeweils einen Suchbegriff eintragen, z. B. `Mensa` und `Hausaufgaben`. Bis zu drei Begriffe sind möglich.",
          "Der Begriff wird mit den Namen der heute besuchten Räume und Aktivitäten abgeglichen: passt er, erscheint auf der Kinderkarte ein grüner Haken, sonst ein grauer Kreis.",
        ],
        callout: {
          title: "Begriff muss zum Namen passen",
          body: "Der Haken erscheint nur, wenn der Indikator-Begriff im Namen eines heute besuchten Raums oder einer Aktivität vorkommt. Damit `Hausaufgaben` greift, muss es also einen entsprechend benannten Raum oder eine Aktivität geben. Ist die Funktion ausgeschaltet, werden auf den Karten gar keine Indikatoren angezeigt.",
          tone: "blue",
        },
        screenshot:
          "Sektion Indikatoren im Reiter Betrieb mit eingeschalteten Aktivitäts-Indikatoren und den Begriffen Mensa und Hausaufgaben.",
        image: "/help/screens/einstellungen.webp",
      },
      {
        id: "einstellungen-zustaendigkeit",
        title: "Wer ändert welche Einstellungen?",
        icon: KeyRound,
        summary:
          "Nicht jede Einstellung müssen Sie selbst verwalten. Ein Teil wird vom moto-Team betreut und erscheint für Schul-Admins bewusst gar nicht.",
        steps: [
          "Was Sie selbst anpassen: alltägliche Regeln Ihrer Schule, zum Beispiel Abmeldezeiten, Aktivitäts-Indikatoren, ob mit festen Gruppen oder in offener Betreuung gearbeitet wird, ob Betreuungsplan-Zahlen sichtbar sind, die Geräte-PIN und die Tablet-Buttons. `Offene Betreuung` erweitert nur den operativen Zugriff für Mitarbeitende mit dem passenden Recht.",
          "Was das moto-Team betreut: technische und grundlegende Einstellungen, darunter die Freischaltung der Web-Anwesenheit. Bei ausgeschalteter Web-Anwesenheit verschwinden An-/Abmeldeaktionen in der Web-App; NFC- und Systemvorgänge bleiben aktiv. Diese Einstellungen sind für Schul-Admins ausgeblendet.",
          "Der Reiter `System` ist überwiegend Sache des moto-Teams; als Admin sehen Sie dort in der Regel nur die automatische Datenbereinigung.",
          "Soll eine ausgeblendete Einstellung geändert werden, wenden Sie sich an das moto-Team.",
        ],
        screenshot:
          "Reiter System aus Admin-Sicht: nur die Datenbereinigung ist sichtbar, vom moto-Team betreute Optionen sind ausgeblendet.",
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
        ],
      },
    ],
  },
  {
    id: "nfc-mitarbeiter-stempeln",
    title: "Arbeitszeit am Tablet stempeln",
    description:
      "Mitarbeitende können mit ihrem persönlichen NFC-Armband ein- und ausstempeln sowie Pausen erfassen. Dafür ist keine Aktivität oder Gruppe nötig.",
    icon: Clock3,
    tone: "blue",
    steps: [
      {
        id: "nfc-mitarbeiter-arbeitszeit",
        title: "Ein-, ausstempeln und Pause erfassen",
        summary:
          "Über `Mitarbeiter-Stempeln` liest das Tablet den aktuellen Stand aus und bietet nur die Aktionen an, die gerade möglich sind.",
        steps: [
          "Im `Menü` auf `Mitarbeiter-Stempeln` tippen und `Armband scannen` wählen.",
          "Das persönliche Armband an den NFC-Sensor halten. Kinderarmbänder und nicht zugewiesene Armbänder werden abgewiesen.",
          "Beim Einstempeln `Vor Ort` oder `Homeoffice` wählen und auf `Einstempeln` tippen.",
          "Im Zustand `Eingestempelt` entweder `Pause starten` oder `Ausstempeln` wählen.",
          "Im Zustand `In Pause` die Pause mit `Pause beenden` fortsetzen oder direkt ausstempeln.",
          "Verlangt der Dienstplan eine Begründung für die Abweichung, den Grund eingeben und den Stempelvorgang erneut bestätigen.",
        ],
        callout: {
          title: "Persönliches Armband erforderlich",
          body: "Die Zeiterfassung funktioniert nur mit einem aktiven NFC-Armband, das dem jeweiligen Mitarbeitenden zugewiesen ist. Die allgemeine Geräte-PIN identifiziert die Person nicht. Fehlt die Zuweisung, wenden Sie sich an Ihre OGS-Leitung.",
          tone: "orange",
        },
        checklist: [
          "Nach dem Einstempeln steht der Status auf `Eingestempelt`.",
          "Eine laufende Pause wird als `In Pause` angezeigt.",
          "Ab mehr als sechs Stunden Arbeitszeit erscheint der Hinweis zur erforderlichen Pause, solange sie noch nicht vollständig genommen wurde.",
          "Die Einträge erscheinen parallel in der Zeiterfassung der Web-App mit der Quelle `NFC`.",
        ],
        screenshot:
          "Tablet-Seite Mitarbeiter-Stempeln mit Name, aktuellem Zustand, Arbeitsort-Auswahl und den jeweils erlaubten Aktionen.",
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
          "PIN, Auswahl-Buttons und Detailmeldungen bei vollen Räumen oder Aktivitäten legen Sie im Browser unter `Einstellungen` fest.",
        steps: [
          "Im Browser die `Einstellungen` öffnen und den Tab `Geräte` wählen.",
          "Unter `OGS Geräte-PIN` die 4-stellige PIN setzen, mit der sich das Team am Tablet anmeldet.",
          "Mit `Raumwechsel-Button anzeigen`, `Schulhof-Button anzeigen` und `Toilette-Button anzeigen` festlegen, welche Ziele beim Auschecken (`Wohin geht ...?`) erscheinen.",
          "Mit `Details bei voller Aktivität anzeigen` und `Details bei vollem Raum anzeigen` steuern Sie, ob das Tablet bei erreichten Kapazitäten den Namen und die Belegung nennt oder nur einen allgemeinen Hinweis zeigt.",
          "Änderungen werden automatisch gespeichert und beim nächsten Start vom Tablet übernommen.",
        ],
        callout: {
          title: "Weniger ist mehr",
          body: "`Schulhof` und `Toilette` legen automatisch einen passenden Raum an. Aktivieren Sie nur die Buttons und Detailmeldungen, die Ihre Einrichtung wirklich nutzt - weniger Auswahl und klar dosierte Fehlermeldungen sind für Kinder und Team am Tablet einfacher.",
          tone: "blue",
        },
        screenshot:
          "Einstellungen, Tab `Geräte` mit OGS Geräte-PIN, Button-Schaltern und Kapazitätsdetail-Schaltern.",
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
