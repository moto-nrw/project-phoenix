import { HELP_TOPICS, type HelpTopicId } from "~/lib/help-topics";
import type {
  SchoolSetupState,
  SchoolSetupStep,
  SchoolSetupStepKey,
} from "~/lib/school-setup-api";

/** Was der Assistent zu einem Schritt zeigt (#2832). */
export interface SchoolSetupStepContent {
  title: string;
  description: string;
  /** Hinweise, die eine Entscheidung verändern. */
  hints: readonly string[];
  /** Seite, auf der die Person den Schritt erledigt. Fehlt beim ersten Schritt. */
  href?: string;
  actionLabel?: string;
  helpTopic?: HelpTopicId;
  /**
   * Schritt, der erst erledigt sein muss, bevor die Tour hier Sinn ergibt,
   * mit dem Satz, der das sagt. Überspringen reicht nicht: ohne Kind gibt es
   * keine Eltern einzuladen.
   */
  requires?: { step: SchoolSetupStepKey; reason: string };
}

export const SCHOOL_SETUP_STEP_CONTENT: Readonly<
  Record<SchoolSetupStepKey, SchoolSetupStepContent>
> = {
  basics: {
    title: "So arbeitet Ihre OGS",
    description: "Ihre Antworten bestimmen, welche Schritte folgen.",
    hints: [],
  },
  team: {
    // Singular mit „erste/n“: Ein Datensatz hakt den Schritt ab (#2832).
    // Der Satz darunter sagt, dass die übrigen danach genauso gehen.
    title: "Erste Person ins Team einladen",
    description: "Laden Sie eine Person per E-Mail ein.",
    hints: ["Die übrigen laden Sie danach genauso ein."],
    href: "/database/personal",
    actionLabel: "Zum Personal",
    helpTopic: HELP_TOPICS.leadInviteStaff,
  },
  rooms: {
    title: "Ersten Raum anlegen",
    description: "Legen Sie einen Raum an, in dem Kinder betreut werden.",
    hints: ["Die übrigen legen Sie danach genauso an."],
    href: "/database/rooms",
    actionLabel: "Zu den Räumen",
    helpTopic: HELP_TOPICS.leadRooms,
  },
  groups: {
    title: "Erste Gruppe anlegen",
    description: "Legen Sie eine Ihrer festen Gruppen an.",
    hints: [
      "Die übrigen legen Sie danach genauso an.",
      "Gruppenleitung kann nur werden, wer die Einladung schon angenommen hat.",
      "Die Gruppenleitung tragen Sie deshalb oft später nach.",
    ],
    href: "/database/groups",
    actionLabel: "Zu den Gruppen",
    helpTopic: HELP_TOPICS.leadGroups,
  },
  students: {
    title: "Erstes Kind anlegen",
    description: "Legen Sie ein Kind an.",
    hints: [
      "Haben Sie eine Liste? Mit „Importieren“ kommen alle auf einmal.",
      "Die Betreuungszeiten tragen Sie direkt beim Kind ein.",
    ],
    href: "/database/students",
    actionLabel: "Zu den Kindern",
    helpTopic: HELP_TOPICS.leadCreateStudent,
  },
  guardians: {
    title: "Erste Eltern einladen",
    description: "Laden Sie die Eltern eines Kindes in die Eltern-App ein.",
    hints: [
      "Die übrigen laden Sie danach genauso ein.",
      "Beim Kind tragen Sie die Eltern mit E-Mail-Adresse ein und laden sie ein.",
      "Für viele Kinder auf einmal: In der Kinderliste „Auswählen“, dann „Eltern einladen“.",
    ],
    href: "/database/students",
    actionLabel: "Zu den Kindern",
    helpTopic: HELP_TOPICS.leadInviteGuardians,
    requires: {
      step: "students",
      reason: "Dafür braucht es zuerst mindestens ein Kind in moto.",
    },
  },
};

/** Die Schritte, die für diese Schule gelten, in Reihenfolge. */
export function applicableSteps(state: SchoolSetupState): SchoolSetupStep[] {
  return state.steps.filter((step) => step.applies);
}

function isStepOpen(step: SchoolSetupStep): boolean {
  return !step.done && !step.skipped;
}

/** Der erste offene Schritt, oder `null`, wenn alles erledigt ist. */
export function firstOpenStep(
  state: SchoolSetupState,
): SchoolSetupStepKey | null {
  return applicableSteps(state).find(isStepOpen)?.key ?? null;
}

/** Wie viele der geltenden Schritte erledigt oder übersprungen sind. */
export function setupProgress(state: SchoolSetupState): {
  finished: number;
  total: number;
} {
  const steps = applicableSteps(state);
  return {
    finished: steps.filter((step) => !isStepOpen(step)).length,
    total: steps.length,
  };
}

/**
 * Nach den ersten Einträgen: die Anleitungen, mit denen die Schule die
 * übrigen Daten anlegt, nur für die Schritte, die für sie gelten.
 */
const FILL_TOPICS: Readonly<
  Partial<Record<SchoolSetupStepKey, SchoolSetupNextTopic>>
> = {
  team: {
    title: "Das ganze Team einladen",
    description: "Laden Sie alle Mitarbeitenden ein.",
    link: { topic: HELP_TOPICS.leadInviteStaff },
  },
  rooms: {
    title: "Alle Räume anlegen",
    description: "Legen Sie jeden Raum an, in dem Kinder betreut werden.",
    link: { topic: HELP_TOPICS.leadRooms },
  },
  groups: {
    title: "Alle Gruppen anlegen",
    description: "Tragen Sie auch die Gruppenleitungen nach.",
    link: { topic: HELP_TOPICS.leadGroups },
  },
  students: {
    title: "Alle Kinder übernehmen",
    description: "Mit „Importieren“ kommen alle Kinder aus einer Liste.",
    link: { topic: HELP_TOPICS.leadCreateStudent },
  },
  guardians: {
    title: "Alle Eltern einladen",
    description:
      "Für viele Kinder auf einmal: „Auswählen“, dann „Eltern einladen“.",
    link: { topic: HELP_TOPICS.leadInviteGuardians },
  },
};

export function fillTopics(
  steps: readonly SchoolSetupStep[],
): readonly SchoolSetupNextTopic[] {
  return steps.flatMap((step) => {
    const topic = step.applies ? FILL_TOPICS[step.key] : undefined;
    return topic ? [topic] : [];
  });
}

/**
 * Eine Karte am Abschluss. Sie führt entweder auf eine Anleitung (die
 * übrigen Daten anlegen) oder auf ein Oberthema der Hilfe (danach).
 */
export interface SchoolSetupNextTopic {
  title: string;
  description: string;
  link: { topic: HelpTopicId } | { group: string };
}

/**
 * Die Oberthemen der Hilfe für die Leitung, mit denen es nach den ersten
 * Schritten weitergeht. Die Titel sind die Kategorienamen der Hilfe
 * (`HELP_GROUP_LABELS`); ein Test hält sie gleich. Sie stehen hier, weil der
 * Assistent auf jeder Seite geladen wird und der Hilfe-Inhalt groß ist.
 */
export const NEXT_HELP_GROUPS: readonly SchoolSetupNextTopic[] = [
  {
    title: "Betreuung und Team planen",
    description:
      "Betreuungsplan, Dienstplan, Vertretungen, Schuljahr und Ferien.",
    link: { group: "planung" },
  },
  {
    title: "Personalverwaltung",
    description: "Rechte, Personalakten, Arbeitszeiten und Abrechnung.",
    link: { group: "personal" },
  },
  {
    title: "Eltern und Anfragen",
    description: "Nachrichten, Mitteilungen und Anfragen der Eltern.",
    link: { group: "elternarbeit" },
  },
  {
    title: "Anmeldungen",
    description: "Lassen Sie neue Kinder online zur Betreuung anmelden.",
    link: { group: "anmeldeverwaltung" },
  },
  {
    title: "Einstellungen",
    description: "Betrieb der OGS und was Eltern in moto sehen.",
    link: { group: "konfiguration" },
  },
];
