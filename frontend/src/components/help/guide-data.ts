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
  MessageSquare,
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
      "Vertretungen, Betreuungsplan, Zeiterfassung",
      "Datenverwaltung, Anmeldungen, Feedback, Einstellungen",
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
          "Findet jedes Kind und zeigt seinen aktuellen Status und Aufenthaltsort.",
        steps: [
          "`Kindersuche` öffnen.",
          "Namen oder Namensbestandteil in das Suchfeld eingeben.",
          "Bei Bedarf nach Klasse, Gruppe, Stufe oder Status filtern.",
          "Für aktuelle Klassenlisten im Filter `Klasse` den Klassenverband wählen und über `Exportieren` die Vorlage `Klassenliste` ausgeben. Phasebezogene Listen für Klassenlehrkräfte erstellst du in der jeweiligen `Anmeldephase`.",
          "Die Vorlage `Tagesliste` enthält den `Tagesstatus`, damit `Krank`, `Entschuldigt` und `Klassenfahrt` direkt auf der Liste stehen.",
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
          "Tab `Stammdaten`: Name, Klasse, Gruppe, Geburtstag, Gesundheitsinformationen, Notizen, Foto und Datenschutz ansehen und über `Bearbeiten` ändern. Wenn das Kind über eine Online-Anmeldung übernommen wurde, stehen dort auch kindbezogene Zusatzantworten aus dem Anmeldeformular zur Ansicht.",
          "Tab `Nachrichten`: die Unterhaltung mit einer Bezugsperson zu diesem Kind ansehen und über `Neue Nachricht` der Bezugsperson schreiben. Pro Kind und Bezugsperson gibt es eine fortlaufende Unterhaltung (wie ein Chat, ohne Betreff). Ungelesene Eltern-Nachrichten sind mit einem roten Abzeichen markiert; geschrieben und beantwortet wird im Chat-Fenster.",
          "Tab `Erziehungsberechtigte`: Bezugspersonen mit Kontaktdaten, Abholberechtigung und Notfallkontakten pflegen. Pro Person zeigt ein Status, ob sie ein Konto für das Elternportal hat (`Konto aktiv`, `Einladung offen` oder `Kein Konto`); mit `Einladen` laden Sie eine bereits hinterlegte Bezugsperson zum Elternportal ein, ohne die Daten erneut einzugeben.",
          "Tab `Betreuungszeiten`: die wöchentlichen Ankunfts- und Abholzeiten je Wochentag verwalten und einzelne Tage anpassen. Hat ein Elternteil über das Elternportal eine Ankunfts- oder Abholzeit für einen Tag geändert, ist dieser Tag mit `Von Eltern` markiert; beim Ändern oder Entfernen dieser Zeit fragt die App zur Sicherheit nach, damit die Angabe der Eltern nicht versehentlich überschrieben wird.",
          "Tab `Historie`: die Anwesenheits-Historie der letzten Tage mit Raum-Details nachvollziehen.",
          "Tab `Anmeldungen` (nur Admins): Online-Anmeldungen anzeigen, die dieses Kind ins System gebracht haben. Dort sehen Sie Betreuungsangebote, Gesundheitsangaben, Notfallkontakte, Zustimmungen und Zusatzantworten; Erziehungsberechtigte stehen weiterhin im eigenen Tab. Bei bestätigten Kindern können Betreuungsangebote dort nachträglich mit Begründung korrigiert werden. Die Angaben können außerdem exportiert werden.",
        ],
        callout: {
          title: "Abmeldungen von Eltern",
          body: "Wenn das Elternportal aktiv ist, können Eltern ihr Kind selbst abmelden – wahlweise als `Krank` oder als `Entschuldigt` (z. B. wegen eines Termins), auch für Tage in der Zukunft. Eine Krankmeldung erscheint wie eine des Teams (das Kind wird als krank angezeigt), eine entschuldigte Abmeldung wie eine Entschuldigung des Teams; ein eventueller Grund wird jeweils mitgespeichert.",
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
          body: "Ist `Mit Freigabe durch das Team` gewählt, landen von Eltern angestoßene Einladungen unter `Konto-Anfragen` (nur für Admins). Dort sehen Sie, wer für welches Kind angefragt wurde, und geben die Anfrage mit `Freigeben` frei oder lehnen sie mit `Ablehnen` ab. Erst nach der Freigabe wird der Zugang gewährt.",
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
          "Offene Anfragen finden Sie als Admin unter `Änderungsanfragen` in der Seitenleiste, getrennt nach `Stammdaten` und `Betreuungszeiten`.",
          "Pro Anfrage sehen Sie das Kind und die Änderung (alter → neuer Wert); bei Betreuungszeiten den Wochenplan-Vergleich je Wochentag (Bringzeit, Abholzeit, Abholart).",
          "Mit `Freigeben` wird der neue Wert übernommen – bei Betreuungszeiten direkt in den Wochenplan des Kindes. Mit `Ablehnen` bleibt der bisherige Stand erhalten; bei Betreuungszeiten ist dafür eine Begründung erforderlich, bei Stammdaten optional.",
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
          "Im Reiter `Zeiterfassung` zwischen Woche und Monat wechseln, Soll, Ist, Saldo, Quelle und Hinweise kontrollieren.",
          "Bei einem Arbeitstag das Stift-Symbol nutzen, um Zeiten nachzutragen oder zu korrigieren. Eine Begründung ist erforderlich und landet im Audit-Log.",
          "Im Reiter `Arbeitszeitmodell` eine Vorlage zuweisen oder ein eigenes Modell mit 1 bis 4 Wochen Rotation pflegen.",
          "Im Reiter `Abwesenheiten` Urlaubsanspruch und offene Anträge prüfen, genehmigen oder mit Begründung ablehnen.",
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
        title: "Betreuungsplan",
        icon: CalendarDays,
        summary:
          "Plant Termine, Regeltermine, Räume, Personal und erwartete Kinder im Voraus (nur für Admins).",
        steps: [
          "`Betreuungsplan` öffnen.",
          "In der Wochenansicht eine freie Zelle am gewünschten Tag und zur gewünschten Uhrzeit anklicken (beim Überfahren erscheint `+ Termin`).",
          "`Titel` eintragen und `Raum` wählen. `Datum`, `Start` und `Ende` sind aus der Zelle übernommen und lassen sich anpassen.",
          "Unter `Wiederholt sich` festlegen, wie oft das Angebot stattfindet: `Einmalig`, wöchentlich am gewählten Wochentag, `Jeden Wochentag (Mo–Fr)` oder `Benutzerdefiniert …` für eigene Rhythmen.",
          "Über `Weitere Optionen` `Personal` und `Kinder` zuordnen. Mit `Klasse/Gruppe komplett hinzufügen …` kommt eine ganze Klasse oder Gruppe auf einmal in die Auswahl.",
          "Speichern. Wiederholte Termine trägt Phoenix automatisch für das gesamte Schuljahr ein. Hinweise zu doppelt belegten Räumen, Personal oder Kindern können beim Ausfüllen erscheinen, verhindern das Speichern aber nicht.",
          "Beim Bearbeiten eines Termins aus einer Serie fragt die App, wofür die Änderung gilt: `Nur dieser Termin`, `Dieser und alle folgenden` oder `Alle Termine der Serie`.",
          "Beim Löschen eines Serientermins wählen Sie zwischen `Nur dieser Termin` und `Dieser und alle folgenden`; frühere Termine bleiben erhalten. Einen Regeltermin löschen Sie über `Bearbeiten` -> `Löschen` und wählen dort das `Ab Datum`.",
          "Geplante Termine erscheinen zur Startzeit in der `Aktuellen Aufsicht` unter `Als Nächstes` und werden dort mit `Starten` begonnen.",
        ],
        callout: {
          title: "Betreuungsplan bei Bedarf abschalten",
          body: "Der Betreuungsplan ist standardmäßig sichtbar. Einrichtungen, die ihn nicht nutzen, schalten ihn unter `Einstellungen` -> `Betrieb` mit `Betreuungsplan aktivieren` aus; danach verschwindet er aus der Seitenleiste.",
          tone: "blue",
        },
        screenshot: "Betreuungsplan-Kalender mit geplanten Terminen.",
        image: "/help/screens/stundenplan.webp",
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
          "Pausen mit einer geplanten Dauer starten. Die Pause endet automatisch nach Ablauf oder manuell über `Pause beenden`.",
          "Bei langen Arbeitstagen die Pausenhinweise beachten.",
          "Für Krankheit, Fortbildung oder sonstige Abwesenheit `Abwesend` wählen und die Abwesenheit mit Art, Zeitraum und optionaler Notiz speichern.",
        ],
        callout: {
          title: "Arbeitsort bewusst wählen",
          body: "Die App setzt keinen Arbeitsort voraus. Vor dem Einstempeln muss bewusst `In der OGS` oder `Homeoffice` gewählt werden, damit die Zeiterfassung später eindeutig bleibt.",
          tone: "orange",
        },
        screenshot:
          "Zeiterfassung mit Einstempeln, Pause, Ausstempeln und Abwesenheit melden.",
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
          "Tageszeilen prüfen: Check-in, Check-out, Pause, Soll, Ist, Saldo, Status, Quelle und Hinweise zeigen, ob ein Tag vollständig erfasst wurde.",
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
        ],
        screenshot: "Datenverwaltung mit allen Bereichen und Eintragszahlen.",
        image: "/help/screens/datenverwaltung.webp",
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
          "Für Klassenlehrkräfte unter `Klasse für Klassenliste` den Klassenverband wählen und `Klassenliste exportieren` nutzen. Die Liste enthält den gesamten Klassenverband, auch Kinder ohne bestätigte Anmeldung, und zeigt pro Wochentag die gebuchten Angebote, zum Beispiel `Randstunde` oder `Ganztag`, inklusive Abholzeit, Geh-/Abholweise und Kontaktdaten der Erziehungsberechtigten.",
          "Eine Anmeldung öffnen und Kind, erziehungsberechtigte Personen (Hauptkontakt und weitere erziehungsberechtigte Personen), gewähltes Betreuungsangebot und Formularangaben prüfen.",
          "Wenn eine Familie nach der Frist nachgereicht hat, erscheint die Anmeldung nach Nutzung des Nachzügler-Links ganz normal in dieser Liste. Bei der manuellen Freigabe ist das Kind bereits bestätigt; prüfe anschließend bei Bedarf den Statuslink oder die Kinddetailseite.",
          "Mit `Bestätigen`, `Warteliste` oder `Ablehnen` entscheiden; mit `Zur Prüfung` für später vormerken.",
          "Bei bestätigten Kindern können Betreuungsangebote über `Betreuungsangebote bearbeiten` nachträglich korrigiert werden. Eine Begründung ist Pflicht; die Änderungshistorie zeigt danach, wer was wann angepasst hat.",
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
          "Für die Übergabe an Klassenlehrkräfte nutzt du in der Phase `Klasse für Klassenliste` und `Klassenliste exportieren`; diese Liste ist pro Klasse aufgebaut und zeigt pro Wochentag die bestätigten Angebote der Phase, Abholzeiten, Geh-/Abholweise und Erziehungsberechtigte.",
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
          "`Name`, `Beschreibung` und die möglichen `Wochentage` festlegen.",
          "Unter `Betreuungsplan-Vorlage` den passenden Regeltermin verknüpfen, wenn genehmigte Anmeldungen in dieser Vorlage erwartet werden sollen.",
          "Unter `Betreuungstage & Mitbuchung` festlegen, ob das Angebot als Betreuungstage zählt und ob es mitgebucht wird, wenn Eltern bestimmte andere Angebote wählen.",
          "Bei der Mitbuchung die auslösenden Angebote auswählen und optional auf Klassenstufen eingrenzen.",
          "Optional `Kapazität`, `Preis in Cent` sowie `Mittagessen` oder `Ferienbetreuung` ergänzen.",
          "`Aktiv` setzen - nur aktive Angebote sind für Eltern auswählbar.",
        ],
        callout: {
          title: "Anmeldung und Betreuungsplan verbinden",
          body: "Eltern wählen weiterhin nur Angebot und Tage. Phoenix übernimmt genehmigte Kinder in die verknüpfte Betreuungsplan-Vorlage und materialisiert sie dort an den passenden Angebotstagen. Hinweise zeigen, wenn Angebotstage, Vorlage, erwartete Ankunft oder Klassengruppe nicht zusammenpassen.",
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
          "Nur bei zusätzlichem Bedarf über `Neue Vorlage` eine eigene Formularvorlage mit Zusatzfragen anlegen. Bei `Erlaubte Heimwege` wählen Eltern pro Betreuungstag alle zulässigen Wege aus, zum Beispiel zu Fuß, Bus, Abholung oder mit anderem Kind; mehrere Optionen pro Tag sind möglich. Bei `mit anderem Kind` ergänzen Eltern im Pflichtfeld, mit wem das Kind nach Hause geht.",
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
          "In der Seitenleiste `Nachrichten` öffnen. Ein rotes Abzeichen zeigt ungelesene Eltern-Nachrichten an.",
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
          "In der Seitenleiste `Änderungsanfragen` öffnen. Dort stehen alle offenen Anfragen, getrennt nach `Stammdaten` und `Betreuungszeiten`, jeweils mit dem Vergleich `aktuell -> gewünscht`.",
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
        id: "essensplan",
        title: "Essensplan",
        icon: UtensilsCrossed,
        summary:
          "Die Woche als Plan: Montag bis Freitag nebeneinander, pro Tag ein oder mehrere Gerichte mit optionalem Hinweis. Eltern sehen den Plan für die aktuelle und nächste Woche im Elternportal.",
        steps: [
          "In der Seitenleiste `Essensplan` öffnen.",
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
          "Was Sie selbst anpassen: alltägliche Regeln Ihrer Schule, zum Beispiel Abmeldezeiten, Aktivitäts-Indikatoren, ob mit festen Gruppen gearbeitet wird, die Geräte-PIN und die Tablet-Buttons.",
          "Was das moto-Team betreut: technische und schulübergreifende Einstellungen. Diese sind für Schul-Admins ausgeblendet.",
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
