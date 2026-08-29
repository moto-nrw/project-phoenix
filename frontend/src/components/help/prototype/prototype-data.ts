import {
  BellRing,
  Database,
  Eye,
  FileText,
  ListChecks,
  Search,
  Settings,
  ShieldAlert,
  TabletSmartphone,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type PrototypeRole = "caregiver" | "lead";
export type PrototypePresenceMode = "detailed" | "binary";
export type PrototypeSchoolyard = "enabled" | "disabled";

export interface PrototypeTopic {
  readonly id: string;
  readonly title: string;
  readonly question: string;
  readonly summary: string;
  readonly group: "alltag" | "leitung" | "nfc";
  readonly audience: "all" | PrototypeRole;
  readonly icon: LucideIcon;
  readonly steps: readonly string[];
  readonly note?: string;
  readonly image?: string;
  readonly imageAlt?: string;
  readonly related: readonly string[];
}

export const BASE_PROTOTYPE_TOPICS: readonly PrototypeTopic[] = [
  {
    id: "kindersuche",
    title: "Ein Kind finden",
    question: "Wie finde ich ein Kind?",
    summary: "Suchen Sie nach Namen, Klasse, Gruppe oder Anwesenheit.",
    group: "alltag",
    audience: "all",
    icon: Search,
    steps: [
      "Öffnen Sie `Alle Kinder`.",
      "Geben Sie den Namen in das Suchfeld ein.",
      "Wählen Sie bei Bedarf einen Filter.",
      "Tippen Sie auf das Kind für weitere Angaben.",
    ],
    note: "Mit `Zurücksetzen` entfernen Sie alle gewählten Filter.",
    image: "/help/screens/kindersuche.webp",
    imageAlt: "Kinderliste mit Suche, Filtern und Angaben zur Anwesenheit.",
    related: ["kinderdetailansicht", "abwesenheiten", "notfall"],
  },
  {
    id: "kinderdetailansicht",
    title: "Angaben zu einem Kind ansehen",
    question: "Wo sehe ich alle Angaben zu einem Kind?",
    summary: "Die Kinderakte bündelt Kontakte, Zeiten und wichtige Hinweise.",
    group: "alltag",
    audience: "all",
    icon: FileText,
    steps: [
      "Öffnen Sie `Alle Kinder`.",
      "Suchen Sie das Kind.",
      "Tippen Sie auf die Karte des Kindes.",
      "Wählen Sie oben den passenden Bereich.",
    ],
    note: "Bearbeitungen sind nur mit der passenden Berechtigung möglich.",
    image: "/help/screens/kinderdetailansicht.webp",
    imageAlt: "Kinderakte mit Anwesenheit, Kontakten und weiteren Bereichen.",
    related: ["kindersuche", "abwesenheiten", "notfall"],
  },
  {
    id: "meine-gruppen",
    title: "Die eigenen Gruppen sehen",
    question: "Welche Kinder betreue ich heute?",
    summary: "Hier sehen Sie Ihre Gruppen und die geplanten Kinder.",
    group: "alltag",
    audience: "caregiver",
    icon: Users,
    steps: [
      "Öffnen Sie `Meine Gruppen`.",
      "Wählen Sie oben den gewünschten Tag.",
      "Tippen Sie auf eine Gruppe.",
      "Prüfen Sie Kinder, Zeiten und Hinweise.",
    ],
    image: "/help/screens/meine-gruppen.webp",
    imageAlt: "Übersicht der eigenen Gruppen mit Kinderzahlen.",
    related: ["aktuelle-aufsicht", "kindersuche"],
  },
  {
    id: "aktuelle-aufsicht",
    title: "Die laufende Aufsicht prüfen",
    question: "Wer betreut gerade welche Kinder?",
    summary:
      "Die Übersicht zeigt laufende Gruppen, Räume und Betreuungskräfte.",
    group: "alltag",
    audience: "all",
    icon: Eye,
    steps: [
      "Öffnen Sie `Aktuelle Aufsicht`.",
      "Suchen Sie die gewünschte Gruppe.",
      "Prüfen Sie Raum, Team und Kinderzahl.",
      "Öffnen Sie die Gruppe für weitere Angaben.",
    ],
    image: "/help/screens/aktuelle-aufsicht.webp",
    imageAlt: "Laufende Aufsichten mit Raum, Team und Kinderzahl.",
    related: ["meine-gruppen", "kindersuche", "nfc-kinder-einchecken"],
  },
  {
    id: "abwesenheiten",
    title: "Eine Abwesenheit eintragen",
    question: "Wie melde ich ein Kind krank oder entschuldigt?",
    summary: "Tragen Sie den Grund und den passenden Zeitraum ein.",
    group: "alltag",
    audience: "all",
    icon: BellRing,
    steps: [
      "Öffnen Sie die Kinderakte.",
      "Wählen Sie `Krank melden` oder `Entschuldigen`.",
      "Wählen Sie Tag oder Zeitraum.",
      "Prüfen Sie die Angaben und speichern Sie.",
    ],
    note: "Eine Entschuldigung kann auch ab einer Uhrzeit gelten.",
    related: ["kindersuche", "kinderdetailansicht"],
  },
  {
    id: "notfall",
    title: "Im Notfall Kontakte finden",
    question: "Wo finde ich wichtige Kontakte im Notfall?",
    summary:
      "Die Notfallansicht zeigt Kontakte und wichtige Gesundheitsangaben.",
    group: "alltag",
    audience: "all",
    icon: ShieldAlert,
    steps: [
      "Öffnen Sie `Notfall`.",
      "Suchen Sie das Kind.",
      "Öffnen Sie den Eintrag.",
      "Nutzen Sie die angezeigten Kontakte und Hinweise.",
    ],
    note: "Nutzen Sie diese Ansicht nur in einer akuten Situation.",
    image: "/help/screens/notfall.webp",
    imageAlt: "Notfallansicht mit Kontakt- und Gesundheitsangaben.",
    related: ["kindersuche", "kinderdetailansicht"],
  },
  {
    id: "datenverwaltung",
    title: "Schuldaten verwalten",
    question: "Wo verwalte ich die Daten der Einrichtung?",
    summary: "Leitungen finden hier Importe, Exporte und Schuljahreswechsel.",
    group: "leitung",
    audience: "lead",
    icon: Database,
    steps: [
      "Öffnen Sie `Datenverwaltung`.",
      "Wählen Sie die gewünschte Aufgabe.",
      "Lesen Sie den Hinweis vor der Änderung.",
      "Prüfen Sie die Angaben vor dem Speichern.",
    ],
    note: "Dieser Bereich ist nur mit Leitungsrechten sichtbar.",
    image: "/help/screens/datenverwaltung.webp",
    imageAlt: "Datenverwaltung mit Importen, Exporten und weiteren Aufgaben.",
    related: ["anmeldungen-pruefen", "einstellungen-ueberblick"],
  },
  {
    id: "anmeldungen-pruefen",
    title: "Anmeldungen prüfen",
    question: "Wie prüfe ich eine neue Anmeldung?",
    summary: "Vergleichen Sie neue Angaben vor der Übernahme.",
    group: "leitung",
    audience: "lead",
    icon: ListChecks,
    steps: [
      "Öffnen Sie `Anmeldungen`.",
      "Wählen Sie eine offene Anmeldung.",
      "Prüfen Sie neue und vorhandene Angaben.",
      "Übernehmen oder lehnen Sie die Anmeldung ab.",
    ],
    note: "Die Entscheidung verändert die Daten des Kindes.",
    related: ["datenverwaltung", "einstellungen-ueberblick"],
  },
  {
    id: "einstellungen-ueberblick",
    title: "Einstellungen der Einrichtung ändern",
    question: "Wo ändere ich die Einstellungen der Einrichtung?",
    summary: "Leitungen steuern hier Funktionen und Abläufe der Einrichtung.",
    group: "leitung",
    audience: "lead",
    icon: Settings,
    steps: [
      "Öffnen Sie `Einstellungen`.",
      "Wählen Sie links einen Bereich.",
      "Ändern Sie nur die benötigte Einstellung.",
      "Speichern Sie die Änderung.",
    ],
    note: "Änderungen können sofort für das ganze Team gelten.",
    image: "/help/screens/einstellungen.webp",
    imageAlt: "Einstellungen mit Bereichen für den Betrieb der Einrichtung.",
    related: ["datenverwaltung", "anmeldungen-pruefen"],
  },
] as const;

const DETAILED_NFC_TOPICS: readonly PrototypeTopic[] = [
  {
    id: "nfc-kinder-einchecken",
    title: "Ein Kind am Tablet einchecken",
    question: "Wie checkt sich ein Kind am Tablet ein?",
    summary: "Das Armband meldet das Kind in der laufenden Aufsicht an.",
    group: "nfc",
    audience: "all",
    icon: TabletSmartphone,
    steps: [
      "Starten Sie zuerst eine Aufsicht mit einem Raum.",
      "Das Kind hält sein Armband an das Lesegerät.",
      "Das Tablet zeigt Name, Abholzeit und Raum.",
      "Danach erscheint wieder die laufende Aufsicht.",
    ],
    note: "Diese Anleitung gilt für die detaillierte Anwesenheit.",
    image: "/help/screens/nfc-kind-eingecheckt.webp",
    imageAlt: "Bestätigung nach dem Einchecken mit Raum und Abholzeit.",
    related: ["nfc-kinder-auschecken", "aktuelle-aufsicht"],
  },
  {
    id: "nfc-kinder-auschecken",
    title: "Ein Ziel am Tablet wählen",
    question: "Wie wechselt ein Kind den Raum oder geht nach Hause?",
    summary: "Nach dem Scan wählt das Kind sein nächstes Ziel.",
    group: "nfc",
    audience: "all",
    icon: TabletSmartphone,
    steps: [
      "Das eingecheckte Kind scannt sein Armband erneut.",
      "Das Tablet fragt nach dem nächsten Ziel.",
      "Das Kind wählt Raum, Schulhof, Toilette oder Zuhause.",
      "Das Tablet bestätigt die Auswahl.",
    ],
    note: "Die sichtbaren Ziele hängen von den Geräte-Einstellungen ab.",
    image: "/help/screens/nfc-auschecken.webp",
    imageAlt: "Tablet mit den Zielen Raum, Schulhof, Toilette und Zuhause.",
    related: ["nfc-kinder-einchecken", "aktuelle-aufsicht"],
  },
] as const;

function binaryNfcTopics(
  schoolyard: PrototypeSchoolyard,
): readonly PrototypeTopic[] {
  const schoolyardEnabled = schoolyard === "enabled";
  return [
    {
      id: "nfc-kinder-einchecken",
      title: "Ein Kind als anwesend eintragen",
      question: "Wie meldet sich ein Kind am Eingang an?",
      summary: "Das Armband setzt die Anwesenheit auf `Anwesend`.",
      group: "nfc",
      audience: "all",
      icon: TabletSmartphone,
      steps: [
        "Das Kind hält sein Armband an das Lesegerät.",
        "Das Tablet zeigt die Anmeldung an.",
        "Der Status des Kindes wechselt auf `Anwesend`.",
        "Eine Auswahl von Raum oder Aktivität entfällt.",
      ],
      note: "Diese Anleitung gilt für die einfache Anwesenheit.",
      image: "/help/screens/nfc-kind-eingecheckt.webp",
      imageAlt: "Bestätigung nach der Anmeldung am Tablet.",
      related: ["nfc-kinder-auschecken", "kindersuche"],
    },
    {
      id: "nfc-kinder-auschecken",
      title: "Ein Kind am Eingang abmelden",
      question: "Wie meldet sich ein Kind am Eingang ab?",
      summary: schoolyardEnabled
        ? "Das Kind wählt `Schulhof` oder `Nach Hause`."
        : "Das Kind meldet sich mit einem weiteren Scan ab.",
      group: "nfc",
      audience: "all",
      icon: TabletSmartphone,
      steps: schoolyardEnabled
        ? [
            "Das anwesende Kind scannt sein Armband erneut.",
            "Das Tablet zeigt `Schulhof` und `Nach Hause`.",
            "Das Kind wählt das passende Ziel.",
            "Das Tablet bestätigt die Auswahl.",
          ]
        : [
            "Das anwesende Kind scannt sein Armband erneut.",
            "Das Tablet fragt nach der Abmeldung.",
            "Das Kind wählt `Nach Hause`.",
            "Das Tablet bestätigt die Abmeldung.",
          ],
      note: schoolyardEnabled
        ? "Raumwechsel und Toilette gibt es in diesem Modus nicht."
        : "Raumwechsel, Schulhof und Toilette gibt es hier nicht.",
      image: "/help/screens/nfc-auschecken.webp",
      imageAlt: schoolyardEnabled
        ? "Tablet mit den Zielen Schulhof und Nach Hause."
        : "Tablet mit der Auswahl Nach Hause.",
      related: ["nfc-kinder-einchecken", "kindersuche"],
    },
  ];
}

export function getPrototypeTopics(
  presenceMode: PrototypePresenceMode,
  schoolyard: PrototypeSchoolyard,
): readonly PrototypeTopic[] {
  return [
    ...BASE_PROTOTYPE_TOPICS,
    ...(presenceMode === "detailed"
      ? DETAILED_NFC_TOPICS
      : binaryNfcTopics(schoolyard)),
  ];
}

export const PROTOTYPE_GROUP_LABELS = {
  alltag: "Im OGS-Alltag",
  leitung: "Für die Leitung",
  nfc: "NFC und Tablets",
} as const;
