import type {
  SetupTour,
  SetupTourStop,
} from "~/components/school-setup/setup-tours";
import type { SchoolSetupNextTopic } from "~/components/school-setup/school-setup-steps";
import { HELP_TOPICS, type HelpTopicId } from "~/lib/help-topics";
import {
  STAFF_ONBOARDING_STEP_KEYS,
  type StaffOnboardingState,
  type StaffOnboardingStepKey,
} from "~/lib/staff-onboarding-api";

/** Was die Checkliste zu einem Schritt zeigt (#3748). */
export interface StaffOnboardingStepContent {
  title: string;
  description: string;
  /** Hinweise, die eine Entscheidung verändern. */
  hints: readonly string[];
  helpTopic: HelpTopicId;
}

export const STAFF_ONBOARDING_STEP_CONTENT: Readonly<
  Record<StaffOnboardingStepKey, StaffOnboardingStepContent>
> = {
  groups: {
    title: "Meine Gruppe ansehen",
    description:
      "Sehen Sie, welche Kinder in Ihrer Gruppe sind und wer da ist.",
    hints: [],
    helpTopic: HELP_TOPICS.ownGroups,
  },
  students: {
    title: "Ein Kind finden und Angaben ansehen",
    description: "Suchen Sie ein Kind und öffnen Sie seine Angaben.",
    hints: ["Dort stehen auch Abholberechtigte und Notfallkontakte."],
    helpTopic: HELP_TOPICS.studentSearch,
  },
  attendance: {
    title: "Ein Kind an- und abmelden",
    description: "Tragen Sie ein, wann ein Kind kommt und geht.",
    hints: [],
    helpTopic: HELP_TOPICS.webAttendance,
  },
  calendar: {
    title: "Meine Termine ansehen",
    description: "Sehen Sie Ihre Termine und Einsätze im Kalender.",
    hints: [],
    helpTopic: HELP_TOPICS.mySchedule,
  },
  supervision: {
    title: "Eine Aufsicht starten",
    description: "Übernehmen Sie einen Raum oder einen geplanten Termin.",
    hints: [],
    helpTopic: HELP_TOPICS.activeSupervision,
  },
  work_time: {
    title: "Arbeitszeit erfassen",
    description: "Stempeln Sie sich zu Arbeitsbeginn ein und am Ende aus.",
    hints: [],
    helpTopic: HELP_TOPICS.trackWorkTime,
  },
};

/** Was an der Schule und für die Person gilt; entscheidet, welche Schritte es gibt. */
export interface StaffOnboardingContext {
  /** Feste Gruppen und mindestens eine eigene Gruppe. */
  hasOwnGroup: boolean;
  /** An- und Abmelden geht in moto, nicht nur am Tablet. */
  webAttendance: boolean;
  /** Die Person darf ihren Kalender sehen (`calendar:own`). */
  canSeeCalendar: boolean;
  /** Die Schule erfasst Räume (detaillierte Anwesenheit). */
  roomsTracked: boolean;
}

export interface StaffOnboardingStep {
  key: StaffOnboardingStepKey;
  done: boolean;
  skipped: boolean;
}

function applies(
  key: StaffOnboardingStepKey,
  context: StaffOnboardingContext,
): boolean {
  switch (key) {
    case "groups":
      return context.hasOwnGroup;
    case "attendance":
      return context.webAttendance;
    case "calendar":
      return context.canSeeCalendar;
    case "supervision":
      return context.roomsTracked;
    case "students":
    case "work_time":
      return true;
  }
}

/** Die Schritte, die für diese Person gelten, in Reihenfolge. */
export function staffOnboardingSteps(
  state: StaffOnboardingState,
  context: StaffOnboardingContext,
): StaffOnboardingStep[] {
  return STAFF_ONBOARDING_STEP_KEYS.filter((key) => applies(key, context)).map(
    (key) => ({
      key,
      done: state.doneSteps.includes(key),
      skipped: state.skippedSteps.includes(key),
    }),
  );
}

/** Der erste offene Schritt, oder `null`, wenn alles erledigt ist. */
export function firstOpenStaffStep(
  steps: readonly StaffOnboardingStep[],
): StaffOnboardingStepKey | null {
  return steps.find((step) => !step.done && !step.skipped)?.key ?? null;
}

/**
 * Die Kindakte `/students/<id>` (mit Schul-Präfix im Pfad-Routing), auf die
 * die Tour aus der Kindersuche wechselt. Nicht die Suche selbst.
 */
export const STAFF_CHILD_RECORD =
  /^(?!.*\/database\/)(?:\/[^/]+)?\/students\/(?!search$)[^/]+$/;

/** Ein Ziel in der Navigation, am Computer und auf dem Handy. */
interface NavEntry {
  /** Der Eintrag in der Seitenleiste (und gleich benannt im „Mehr“-Menü). */
  target: string;
  label: string;
  /** Der Eintrag in der unteren Leiste auf dem Handy, falls er dort steht. */
  mobile?: { target: string; label: string };
}

const MORE = '[data-setup-tour="nav-more"]';

/**
 * Der Weg zur Seite. Am Computer: erst die Gruppe der Seitenleiste
 * aufklappen, dann den Eintrag. Auf dem Handy: der Eintrag in der unteren
 * Leiste, oder erst „Mehr“ und dann der Eintrag im Menü. Was auf dem
 * jeweiligen Gerät nicht da ist, entfällt, weil die nächste Stelle schon zu
 * sehen ist.
 */
function viaNavigation(
  group: { key: string; label: string },
  entry: NavEntry,
): SetupTourStop[] {
  const entryTarget = entry.mobile
    ? `${entry.target}, ${entry.mobile.target}`
    : entry.target;
  const stops: SetupTourStop[] = [
    {
      target: `[data-setup-tour="nav-group-${group.key}"]`,
      title: `${group.label} öffnen`,
      text: `Wählen Sie in der Seitenleiste „${group.label}“.`,
      advance: "click",
      nav: true,
      // Auf dem Handy gibt es keine Seitenleiste, aber die untere Leiste.
      skipWhenVisible: `${entryTarget}, ${MORE}`,
    },
  ];
  if (!entry.mobile) {
    stops.push({
      target: MORE,
      title: "Mehr öffnen",
      text: "Wählen Sie unten „Mehr“.",
      advance: "click",
      nav: true,
      // Am Computer steht der Eintrag schon in der Seitenleiste.
      skipWhenVisible: entry.target,
    });
  }
  stops.push({
    target: entryTarget,
    title: `${entry.label} öffnen`,
    text: `Wählen Sie „${entry.label}“.`,
    ...(entry.mobile
      ? {
          textWhenVisible: {
            selector: entry.mobile.target,
            text: `Wählen Sie unten „${entry.mobile.label}“.`,
          },
        }
      : {}),
    advance: "click",
    nav: true,
  });
  return stops;
}

/**
 * Ein Reiter der Kindakte: erst anklicken, dann zeigt die Tour seinen Inhalt.
 * Fehlt der Reiter (manche sehen ihn nur mit einem Recht), entfallen beide
 * Stationen still.
 */
function childTab(value: string, label: string, text: string): SetupTourStop[] {
  return [
    {
      target: `[role="tab"][data-tab-value="${value}"]`,
      pathPattern: STAFF_CHILD_RECORD,
      title: label,
      text: `Wählen Sie „${label}“.`,
      advance: "click",
      optional: true,
      skipCount: 2,
    },
    {
      // Die erste Karte des offenen Reiters, nicht die ganze Seite.
      target: '[role="tabpanel"][data-state="active"] > :first-child',
      pathPattern: STAFF_CHILD_RECORD,
      title: label,
      text,
      advance: "next",
    },
  ];
}

const TAGESBETRIEB = { key: "tagesbetrieb", label: "Tagesbetrieb" };
const TEAM = { key: "team", label: "Team" };
const ALL_CHILDREN: NavEntry = {
  target: '[data-setup-tour="nav-/students/search"]',
  label: "Alle Kinder",
  mobile: {
    target: '[data-setup-tour="mobile-nav-/students/search"]',
    label: "Suchen",
  },
};
const SEARCH = 'input[placeholder^="Name suchen"]';
const CHECKIN_TOGGLE = 'button[aria-label="Kinder an- und abmelden"]';
const CHECKIN_BAR = '[aria-label="An- und Abmelde-Modus"]';

/** Die geführten Touren der ersten Schritte (#3748). */
export const STAFF_ONBOARDING_TOURS: Readonly<
  Record<StaffOnboardingStepKey, SetupTour>
> = {
  groups: {
    path: "/ogs-groups",
    stops: [
      ...viaNavigation(TAGESBETRIEB, {
        target: '[data-setup-tour="nav-section-groups"]',
        label: "Meine Gruppen",
        mobile: {
          target: '[data-setup-tour="mobile-nav-/ogs-groups"]',
          label: "Gruppe",
        },
      }),
      {
        // Eine Karte statt der ganzen Seite: Auf dem Handy wäre sonst fast
        // der ganze Bildschirm markiert.
        target: '[data-setup-tour="student-card"]',
        title: "Ihre Gruppe",
        text: "Hier sehen Sie die Kinder Ihrer Gruppe. Jede Karte zeigt, wo das Kind gerade ist und wann es kommt und geht.",
        advance: "next",
        missingText: "In Ihrer Gruppe ist noch kein Kind. Wählen Sie „Weiter“.",
      },
      {
        target: SEARCH,
        title: "Kind suchen",
        text: "Viele Kinder? Geben Sie hier einen Namen ein.",
        advance: "next",
      },
    ],
  },
  students: {
    path: "/students/search",
    stops: [
      ...viaNavigation(TAGESBETRIEB, ALL_CHILDREN),
      {
        target: SEARCH,
        title: "Kind suchen",
        text: "Geben Sie einen Namen ein. Ein Teil des Namens reicht.",
        advance: "next",
      },
      {
        target: '[data-setup-tour="student-card"]',
        title: "Die Karte eines Kindes",
        text: "Schon die Karte zeigt das Wichtigste: wo das Kind gerade ist, wann es kommt und geht und wie es nach Hause kommt. Wählen Sie eine Karte. Dann sehen Sie alle Angaben.",
        advance: "click",
        missingText:
          "Hier steht noch kein Kind. Prüfen Sie die Suche oder die Filter.",
      },
      {
        target: '[role="tablist"]',
        pathPattern: STAFF_CHILD_RECORD,
        title: "Angaben des Kindes",
        text: "Die Angaben sind nach Bereichen sortiert. Die Tour zeigt Ihnen die wichtigsten.",
        advance: "next",
      },
      ...childTab(
        "stammdaten",
        "Stammdaten",
        "Hier stehen Klasse, Gruppe und Geburtsdatum. Wichtig für den Alltag: Gesundheitsinformationen, Notizen und die erlaubten Heimwege.",
      ),
      ...childTab(
        "nachrichten",
        "Nachrichten",
        "Hier schreiben Sie den Eltern dieses Kindes und lesen ihre Antworten.",
      ),
      ...childTab(
        "erziehungsberechtigte",
        "Erziehungsberechtigte",
        "Hier stehen Eltern, Abholberechtigte und Notfallkontakte mit ihren Kontaktangaben.",
      ),
      ...childTab(
        "betreuungsplan",
        "Betreuungsplan",
        "Hier sehen Sie, was für das Kind an einem Tag oder in einer Woche geplant ist.",
      ),
      ...childTab(
        "betreuungszeiten",
        "Betreuungszeiten",
        "Hier stehen die regelmäßigen Ankunfts- und Abholzeiten des Kindes für jeden Wochentag.",
      ),
    ],
  },
  attendance: {
    path: "/students/search",
    stops: [
      ...viaNavigation(TAGESBETRIEB, ALL_CHILDREN),
      {
        target: CHECKIN_TOGGLE,
        title: "An- und abmelden",
        // Der Knopf heißt am Computer „An- & Abmelden“, auf dem Handy
        // „Kinder an- und abmelden“: deshalb ohne Namen.
        text: "Mit diesem Knopf melden Sie Kinder an und ab. Wählen Sie ihn.",
        advance: "click",
        missingText:
          "Diesen Knopf gibt es nur für heute. Wählen Sie oben den heutigen Tag.",
      },
      {
        target: '[data-setup-tour="student-card"][data-checkin-mode]',
        title: "Karte antippen",
        text: "Ein Tipp auf die Karte meldet das Kind an. Ein zweiter Tipp meldet es wieder ab. Der Status steht auf der Karte.",
        advance: "next",
      },
      {
        target: CHECKIN_BAR,
        title: "Fertig",
        text: "Sind Sie fertig, wählen Sie hier „Fertig“. Dann öffnet ein Tipp auf die Karte wieder die Angaben des Kindes.",
        advance: "next",
      },
    ],
  },
  calendar: {
    path: "/calendar",
    stops: [
      ...viaNavigation(TEAM, {
        target: '[data-setup-tour="nav-/calendar"]',
        label: "Mein Kalender",
      }),
      {
        target: '[aria-label="Ansicht wählen"]',
        title: "Tag, Woche oder Monat",
        text: "Wählen Sie, wie viel Sie auf einmal sehen.",
        advance: "next",
      },
      {
        // Die ganze Fläche statt des Rasters: In einer leeren Woche ersetzt
        // die Seite das Raster durch einen Hinweis, und die Tour fände keine
        // Stelle mehr.
        target: ".moto-tenant-body",
        title: "Ihre Termine",
        text: "Hier stehen Ihre Termine, Einsätze und Schichten. Wählen Sie einen Eintrag für mehr Angaben. Ist noch nichts eingetragen, bleibt die Fläche leer. Mit den Pfeilen oben sehen Sie andere Wochen.",
        advance: "next",
      },
    ],
  },
  supervision: {
    path: "/active-supervisions",
    stops: [
      ...viaNavigation(TAGESBETRIEB, {
        target: '[data-setup-tour="nav-section-supervisions"]',
        label: "Aktuelle Aufsicht",
        mobile: {
          target: '[data-setup-tour="mobile-nav-/active-supervisions"]',
          label: "Aufsicht",
        },
      }),
      {
        // „Als Nächstes“, oder ohne Aufsicht die Karte „Keine aktive
        // Raum-Aufsicht“: nie beide, und nie die ganze Seite.
        target:
          '[data-setup-tour="supervision-next"], [data-setup-tour="supervision-empty"]',
        title: "Aufsicht übernehmen",
        text: "Unter „Als Nächstes“ starten Sie Ihren nächsten geplanten Termin. Steht dort nichts, sind Sie noch keinem Betreuungsangebot zugeteilt.",
        // Ohne Aufsicht, offenen Raum und geplanten Termin zeigt die Seite
        // nur „Keine aktive Raum-Aufsicht“, ohne „Als Nächstes“.
        textWhenVisible: {
          selector: '[data-setup-tour="supervision-empty"]',
          text: "Gerade beaufsichtigen Sie keinen Raum, und für Sie ist nichts geplant. Steht ein Termin an, erscheint er hier unter „Als Nächstes“. Dort starten Sie ihn. Erscheint hier nie etwas, sind Sie noch keinem Betreuungsangebot zugeteilt.",
        },
        advance: "next",
      },
    ],
  },
  work_time: {
    path: "/time-tracking",
    stops: [
      ...viaNavigation(TEAM, {
        target: '[data-setup-tour="nav-/time-tracking"]',
        label: "Zeiterfassung",
      }),
      {
        target: '[data-setup-tour="time-clock"]',
        title: "Stempeluhr",
        text: "Wählen Sie zu Arbeitsbeginn den Arbeitsort und dann „Einstempeln“. Pausen und Arbeitsende erfassen Sie hier genauso.",
        // Wer schon eingestempelt ist, sieht keinen Knopf „Einstempeln“.
        textWhenVisible: {
          selector: 'button[aria-label="Ausstempeln"]',
          text: "Sie sind schon eingestempelt. Hier sehen Sie, wie lange Sie heute arbeiten. Mit „Pause starten“ erfassen Sie eine Pause, mit „Ausstempeln“ Ihr Arbeitsende.",
        },
        advance: "next",
      },
    ],
  },
};

/**
 * Nach den ersten Schritten: Anleitungen für Dinge, die seltener anfallen.
 * Titel und Texte folgen den Artikeln der Hilfe.
 */
export const STAFF_NEXT_TOPICS: readonly SchoolSetupNextTopic[] = [
  {
    title: "moto aufs Handy holen",
    description: "So öffnen Sie moto mit einem Tipp wie eine App.",
    link: { topic: HELP_TOPICS.installApp },
  },
  {
    title: "Ein Kind krank melden",
    description: "Tragen Sie ein, wenn ein Kind heute fehlt.",
    link: { topic: HELP_TOPICS.absences },
  },
  {
    title: "Urlaub beantragen",
    description: "Beantragen Sie Urlaub und sehen Sie, was noch übrig ist.",
    link: { topic: HELP_TOPICS.vacation },
  },
  {
    title: "Den Team-Chat nutzen",
    description: "Schreiben Sie Ihren Kolleginnen und Kollegen.",
    link: { topic: HELP_TOPICS.teamChat },
  },
];
