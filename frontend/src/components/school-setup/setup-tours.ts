import type { SchoolSetupStepKey } from "~/lib/school-setup-api";

/**
 * Die geführten Touren der ersten Schritte (#2832).
 *
 * Jede Station zeigt auf eine Stelle der Seite, per CSS-Selektor. Stellen
 * ohne eigene stabile ID tragen `data-setup-tour`; Formularfelder werden
 * über ihre vorhandene `id` gefunden. `advance: "click"` wartet, bis die
 * Person die Stelle selbst anklickt; „Weiter“ gibt es dann nicht.
 */
export interface SetupTourStop {
  target: string;
  title: string;
  text: string;
  advance: "click" | "next";
  /**
   * Station in der Seitenleiste auf dem Weg zur Seite. Steht die Person
   * schon auf der Seite, entfallen diese Stationen; fehlt die Seitenleiste
   * (etwa auf dem Handy), öffnet die Tour die Seite selbst.
   */
  nav?: true;
  /** Entfällt, wenn diese Stelle schon sichtbar ist (Bereich schon offen). */
  skipWhenVisible?: string;
  /**
   * Wie viele Stationen dann entfallen, diese eingeschlossen (Standard 1).
   * So entfällt ein ganzer Zweig, etwa das Eintragen einer Person, die schon
   * eingetragen ist.
   */
  skipCount?: number;
  /**
   * Die Markierung endet über dieser Stelle. So zeigt eine Station eine
   * ganze Karte ohne ihren Knopf, der die nächste Station ist.
   */
  endBefore?: string;
  /**
   * Auf welcher Seite die Station steht, wenn nicht auf der Seite der Tour.
   * Gesucht wird erst, wenn diese Seite geladen ist: sonst träfe die Tour
   * einen gleichnamigen Knopf der Seite, die gerade noch zu sehen ist.
   */
  pathPattern?: RegExp;
  /**
   * Klickt die Person diese Stelle an oder öffnet sie die Seite des Zweigs,
   * geht die Tour dort weiter statt zu enden. So zeigt die Kinder-Tour den
   * Import, wenn die Person „Importieren“ wählt.
   */
  branch?: SetupTourBranch;
  /**
   * Geht von selbst weiter, sobald diese Stelle zu sehen ist, etwa wenn nach
   * dem Hochladen die Vorschau erscheint. „Weiter“ bleibt daneben.
   */
  advanceWhenVisible?: string;
  /** Was die Sprechblase sagt, solange die Stelle fehlt. */
  missingText?: string;
}

/**
 * Die Kindakte `/students/<id>` (mit Schul-Präfix im Pfad-Routing), auf die
 * die Eltern-Tour aus der Kinderliste wechselt. Nicht die Unterseiten der
 * Datenverwaltung wie `/database/students/import`.
 */
export const CHILD_RECORD = /^(?!.*\/database\/)(?:\/[^/]+)?\/students\/[^/]+$/;

export interface SetupTour {
  /** Die Seite, auf der die Tour läuft. */
  path: string;
  stops: readonly SetupTourStop[];
}

const create = '[data-setup-tour="create"]';
const formSubmit = '[data-setup-tour="form-submit"]';
/** Ein Formular der Datenverwaltung, erkannt an seinem Speichern-Knopf. */
const DATA_FORM = `form:has(${formSubmit})`;
/** Das Menü neben einer eingetragenen Person, die man einladen kann. */
const GUARDIAN_MENU = '[data-setup-tour="guardian-menu"]';

/**
 * Der Weg über die Seitenleiste: die Gruppe „Verwaltung“ aufklappen, dann
 * „Datenverwaltung“, dann die Seite. Was schon offen ist, entfällt.
 */
function viaSidebar(path: string, label: string): SetupTourStop[] {
  const subItem = `[data-setup-tour="nav-${path}"]`;
  const database = '[data-setup-tour="nav-database"]';
  return [
    {
      target: '[data-setup-tour="nav-group-verwaltung"]',
      title: "Verwaltung öffnen",
      text: "Die Daten Ihrer OGS finden Sie unter „Verwaltung“. Wählen Sie „Verwaltung“.",
      advance: "click",
      nav: true,
      skipWhenVisible: database,
    },
    {
      target: database,
      title: "Datenverwaltung öffnen",
      text: "Hier in der Seitenleiste legen Sie die Daten Ihrer OGS an. Wählen Sie „Datenverwaltung“.",
      advance: "click",
      nav: true,
      skipWhenVisible: subItem,
    },
    {
      target: subItem,
      title: `${label} öffnen`,
      text: `Wählen Sie „${label}“.`,
      advance: "click",
      nav: true,
    },
  ];
}

const IMPORT_SUBMIT = '[data-setup-tour="student-import-submit"]';

/** Zweige einer Tour, auf eigener Seite und mit eigener Zählung. */
export type SetupTourBranch = "student-import";

export const SETUP_TOUR_BRANCHES: Readonly<Record<SetupTourBranch, SetupTour>> =
  {
    "student-import": {
      path: "/database/students/import",
      stops: [
        {
          target: "section:has(#format-select)",
          title: "Vorlage herunterladen",
          text: "Wählen Sie ein Format und dann „Vorlage herunterladen“. Tragen Sie Ihre Kinder in die Datei ein und speichern Sie sie.",
          advance: "next",
        },
        {
          target: 'section:has([aria-label="Import-Modus"])',
          title: "Neue Kinder anlegen",
          text: "Beim ersten Import lassen Sie „Nur neue anlegen“ gewählt.",
          advance: "next",
        },
        {
          target: 'div:has(> input[type="file"])',
          title: "Datei hochladen",
          text: "Ziehen Sie die ausgefüllte Datei hierher oder wählen Sie „Datei auswählen“. Danach zeigt moto eine Vorschau.",
          advance: "next",
          advanceWhenVisible: IMPORT_SUBMIT,
        },
        {
          target: IMPORT_SUBMIT,
          title: "Prüfen und importieren",
          text: "Prüfen Sie die Vorschau. Hat eine Zeile einen Fehler, korrigieren Sie die Datei und laden Sie sie neu hoch. Dann wählen Sie den grünen Knopf.",
          advance: "click",
          missingText:
            "Noch keine Datei hochgeladen. Wählen Sie „Zurück“ und laden Sie Ihre Datei hoch.",
        },
      ],
    },
  };

export const SETUP_TOURS: Readonly<
  Partial<Record<SchoolSetupStepKey, SetupTour>>
> = {
  team: {
    path: "/database/personal",
    stops: [
      ...viaSidebar("/database/personal", "Personal"),
      {
        target: create,
        title: "Personal einladen",
        text: "Wählen Sie „+ Personal“.",
        advance: "click",
      },
      {
        // Die Karte „Neue Einladung“ (Kopf und Formular), erkannt an ihrem
        // Senden-Knopf; die Markierung endet über dem Knopf.
        target: 'div:has(> form [data-setup-tour="invite-submit"])',
        endBefore: '[data-setup-tour="invite-submit"]',
        title: "Angaben eintragen",
        text: "An die E-Mail-Adresse schickt moto die Einladung. Die Rolle bestimmt, was die Person sehen und ändern darf. Mit dem Namen erkennen Sie die Person in der Liste, auch bevor sie die Einladung annimmt. Die Position ist freiwillig.",
        advance: "next",
      },
      {
        target: '[data-setup-tour="invite-submit"]',
        title: "Einladung senden",
        text: "Wählen Sie „Einladung senden“. Die Person bekommt eine E-Mail.",
        advance: "click",
      },
    ],
  },
  rooms: {
    path: "/database/rooms",
    stops: [
      ...viaSidebar("/database/rooms", "Räume"),
      {
        target: create,
        title: "Raum anlegen",
        text: "Wählen Sie „+ Raum“.",
        advance: "click",
      },
      {
        target: "#name",
        title: "Raumname",
        text: "So heißt der Raum in moto, zum Beispiel „Gruppenraum 1“.",
        advance: "next",
      },
      {
        target: "#category",
        title: "Kategorie",
        text: "Die Kategorie hilft beim Finden, zum Beispiel Turnhalle.",
        advance: "next",
      },
      {
        target: formSubmit,
        title: "Speichern",
        text: "Speichern Sie den Raum. Legen Sie danach die nächsten an.",
        advance: "click",
      },
    ],
  },
  groups: {
    path: "/database/groups",
    stops: [
      ...viaSidebar("/database/groups", "Gruppen"),
      {
        target: create,
        title: "Gruppe anlegen",
        text: "Wählen Sie „+ Gruppe“.",
        advance: "click",
      },
      {
        // Das ganze Formular „Neue Gruppe“ bis über den Speichern-Knopf.
        target: DATA_FORM,
        endBefore: formSubmit,
        title: "Angaben eintragen",
        text: "Geben Sie der Gruppe einen Namen. Gruppenraum und Gruppenleitung können Sie später nachtragen. Als Leitung stehen nur Personen zur Wahl, die ihre Einladung schon angenommen haben.",
        advance: "next",
      },
      {
        target: formSubmit,
        title: "Speichern",
        text: "Speichern Sie die Gruppe.",
        advance: "click",
      },
    ],
  },
  students: {
    path: "/database/students",
    stops: [
      ...viaSidebar("/database/students", "Kinderdaten"),
      {
        target: '[data-setup-tour="student-import"]',
        title: "Viele Kinder auf einmal",
        text: "Haben Sie eine Liste? Mit „Importieren“ übernehmen Sie alle Kinder in einem Schritt. Wählen Sie „Importieren“, dann zeigt die Tour, wie es geht. Für ein einzelnes Kind wählen Sie „Weiter“.",
        advance: "next",
        // Wer hier auf „Importieren“ klickt, sieht, wie der Import geht.
        branch: "student-import",
      },
      {
        target: create,
        title: "Ein Kind anlegen",
        text: "Oder legen Sie ein Kind einzeln an. Wählen Sie „+ Kinder“.",
        advance: "click",
      },
      {
        // Der Scrollbereich des Formulars „Neues Kind“: das Formular, das den
        // „Erstellen“-Knopf trägt, und darin sein erster Block.
        target:
          'form:has([data-setup-tour="student-submit"]) > div:first-child',
        title: "Angaben eintragen",
        text: "Tragen Sie Vorname, Nachname und Klasse ein. Betreuungszeiten können Sie gleich mit eintragen.",
        advance: "next",
      },
      {
        target: '[data-setup-tour="student-submit"]',
        title: "Erstellen",
        text: "Wählen Sie „Erstellen“. Danach steht das Kind in der Liste.",
        advance: "click",
      },
    ],
  },
  guardians: {
    path: "/database/students",
    stops: [
      ...viaSidebar("/database/students", "Kinderdaten"),
      {
        // Die Zeilen der Kinderliste verlinken auf die Kindakte mit ?from=.
        target: 'a[href*="/students/"][href*="?from="]',
        title: "Kind öffnen",
        text: "Eltern laden Sie beim Kind ein. Wählen Sie ein Kind.",
        advance: "click",
      },
      {
        target: '[role="tab"][data-tab-value="erziehungsberechtigte"]',
        pathPattern: CHILD_RECORD,
        title: "Erziehungsberechtigte",
        text: "Hier stehen die Eltern des Kindes. Wählen Sie „Erziehungsberechtigte“.",
        advance: "click",
      },
      {
        target: '[data-setup-tour="guardian-add"]',
        pathPattern: CHILD_RECORD,
        title: "Eltern eintragen",
        text: "Noch niemand eingetragen? Wählen Sie „Hinzufügen“.",
        advance: "click",
        // Steht schon jemand mit E-Mail-Adresse da, entfällt das Eintragen.
        skipWhenVisible: GUARDIAN_MENU,
        skipCount: 3,
      },
      {
        // Der Scrollbereich des Formulars: das Formular mit dem
        // Speichern-Knopf der Erziehungsberechtigten, darin sein erster Block.
        target:
          'form:has([data-setup-tour="guardian-submit"]) > div:first-child',
        pathPattern: CHILD_RECORD,
        title: "Angaben eintragen",
        text: "Tragen Sie Vor- und Nachnamen ein. Wichtig für die Einladung: Tragen Sie auch die E-Mail-Adresse ein. Ohne sie kann moto die Person nicht einladen.",
        advance: "next",
      },
      {
        target: '[data-setup-tour="guardian-submit"]',
        pathPattern: CHILD_RECORD,
        title: "Speichern",
        text: "Speichern Sie die Person. Danach steht sie beim Kind.",
        advance: "click",
      },
      {
        target: GUARDIAN_MENU,
        pathPattern: CHILD_RECORD,
        title: "Menü öffnen",
        text: "Öffnen Sie das Menü neben dem Namen.",
        advance: "click",
      },
      {
        target: '[role="menu"]',
        pathPattern: CHILD_RECORD,
        title: "Einladen",
        text: "Wählen Sie „Einladen“. Die Person bekommt eine E-Mail mit einem Link zur Eltern-App.",
        advance: "click",
      },
    ],
  },
};
