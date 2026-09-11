import { matchesPathPrefix } from "~/lib/section-navigation";

export const HELP_TOPICS = {
  acceptInvitation: "einladung-annehmen-und-konto-einrichten",
  login: "bei-moto-anmelden",
  installApp: "moto-als-app-hinzufuegen",
  appOverview: "sich-in-moto-zurechtfinden",
  mySchedule: "meine-termine-und-einsaetze",
  carePlan: "betreuungsplan-ansehen",
  studentSearch: "kindersuche",
  editStudent: "kind-angaben-aendern",
  webAttendance: "kind-an-und-abmelden",
  changeLocation: "aufenthaltsort-aendern",
  dayLog: "betreuungstag-pruefen",
  ownGroups: "meine-gruppen",
  transferGroup: "gruppe-uebergeben",
  rooms: "raeume-und-belegung",
  activeSupervision: "aktuelle-aufsicht",
  manageActivity: "aktivitaet-anlegen-oder-aendern",
  absences: "abwesenheiten",
  emergency: "notfall",
  parentMessage: "eltern-nachricht-schreiben",
  parentRequests: "elternanfragen-bearbeiten",
  teamChat: "team-chat-nutzen",
  findStaff: "person-im-team-finden",
  sharedFiles: "gemeinsame-datei-oeffnen-oder-hochladen",
  trackWorkTime: "arbeitszeit-und-pausen-erfassen",
  correctWorkTime: "arbeitszeit-pruefen-und-korrigieren",
  vacation: "urlaub-beantragen",
  ownAbsence: "eigene-abwesenheit-eintragen",
  tabletLogin: "am-tablet-anmelden",
  tagAssignment: "armband-zuweisen",
  nfcWorkTime: "arbeitszeit-mit-armband-erfassen",
  nfcSupervision: "aufsicht-am-tablet",
  nfcCheckIn: "nfc-kinder-ein-und-auschecken",
  missingMenu: "menuepunkt-fehlt",
  missingChildOrGroup: "kind-oder-gruppe-fehlt",
  attendanceProblem: "kind-laesst-sich-nicht-an-oder-abmelden",
  nfcProblem: "nfc-tablet-funktioniert-nicht",
  loginProblem: "anmeldung-funktioniert-nicht",
  dataManagement: "datenverwaltung",
  enrollments: "anmeldungen-pruefen",
  settings: "einstellungen-ueberblick",
} as const;

export type HelpTopicId = (typeof HELP_TOPICS)[keyof typeof HELP_TOPICS];

const EXACT_HELP_TOPICS: Readonly<Record<string, HelpTopicId>> = {
  "/activities": HELP_TOPICS.manageActivity,
  "/anfragen": HELP_TOPICS.parentRequests,
  "/calendar": HELP_TOPICS.mySchedule,
  "/dateien": HELP_TOPICS.sharedFiles,
  "/students/search": HELP_TOPICS.studentSearch,
  "/ogs-groups": HELP_TOPICS.ownGroups,
  "/active-supervisions": HELP_TOPICS.activeSupervision,
  "/absences": HELP_TOPICS.absences,
  "/day-log": HELP_TOPICS.dayLog,
  "/emergency": HELP_TOPICS.emergency,
  "/rooms": HELP_TOPICS.rooms,
  "/staff": HELP_TOPICS.findStaff,
  "/time-tracking": HELP_TOPICS.trackWorkTime,
  "/settings": HELP_TOPICS.settings,
};

const PREFIX_HELP_TOPICS: ReadonlyArray<
  readonly [prefix: string, topic: HelpTopicId]
> = [
  ["/students", HELP_TOPICS.studentSearch],
  ["/messages", HELP_TOPICS.parentMessage],
  ["/rooms", HELP_TOPICS.rooms],
  ["/team-chat", HELP_TOPICS.teamChat],
  ["/database", HELP_TOPICS.dataManagement],
  ["/admin/enrollments", HELP_TOPICS.enrollments],
  ["/enrollment-phases", HELP_TOPICS.enrollments],
  ["/care-offerings", HELP_TOPICS.enrollments],
  ["/enrollment-form", HELP_TOPICS.enrollments],
];

/** Finds the most specific existing help article for a staff-app page. */
export function getHelpTopicForPath(pathname: string): HelpTopicId | null {
  const exactTopic = EXACT_HELP_TOPICS[pathname];
  if (exactTopic) return exactTopic;

  for (const [prefix, topic] of PREFIX_HELP_TOPICS) {
    if (matchesPathPrefix(pathname, prefix)) return topic;
  }
  return null;
}
