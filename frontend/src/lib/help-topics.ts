import { matchesPathPrefix } from "~/lib/section-navigation";

/**
 * Wer die Anleitung liest. Steht hier und nicht bei den Inhalten, damit die
 * Navigation eine Hilfe-Adresse bauen kann, ohne die Artikeltexte zu laden.
 */
export type HelpRole = "caregiver" | "lead" | "parent" | "teacher";

/** Die drei Einstellungen, die einen Ablauf in der Anleitung veraendern. */
export interface HelpUrlContext {
  readonly role: HelpRole;
  readonly nfcEnabled: boolean;
  readonly presenceMode: "detailed" | "binary";
  readonly groupMode: "fixed_groups" | "open_care";
  /** Interner Pfad fuer „Zurück zur App". */
  readonly returnTo: string;
}

/**
 * Die Adresse eines Hilfe-Ziels mit dem Kontext, den die App bereits kennt.
 *
 * Ohne `role` fragt `/help` zuerst, fuer wen die Anleitung ist -- eine
 * Frage, deren Antwort die angemeldete Sitzung laengst hat. Jeder Einstieg
 * aus der App heraus gibt sie deshalb mit: das Fragezeichen im Seitenkopf
 * genauso wie der Eintrag `Hilfe` unten in der Seitenleiste.
 *
 * Ohne `topic` fuehrt die Adresse auf die Themenliste der Rolle.
 */
export function buildHelpHref(
  context: HelpUrlContext,
  topic?: HelpTopicId,
): string {
  const query = new URLSearchParams({
    role: context.role,
    nfc_enabled: String(context.nfcEnabled),
    presence_mode: context.presenceMode,
    group_mode: context.groupMode,
    return_to: context.returnTo,
  });

  const path = topic ? `/help/${encodeURIComponent(topic)}` : "/help";
  return `${path}?${query.toString()}`;
}

export const HELP_TOPICS = {
  acceptInvitation: "einladung-annehmen-und-konto-einrichten",
  login: "bei-moto-anmelden",
  installApp: "moto-als-app-hinzufuegen",
  appOverview: "sich-in-moto-zurechtfinden",
  mySchedule: "meine-termine-und-einsaetze",
  carePlan: "betreuungsplan-ansehen",
  dayPlan: "tagesplan-ansehen",
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

  // OGS-Leitung und Verwaltung. Gerüst nach der Informationsarchitektur
  // (#2229, Abschnitt 7); die Abläufe sind noch nicht Schritt für Schritt
  // gegen die App geprüft. Themen, die eine Leitung genauso erledigt wie eine
  // Betreuungskraft (Anmelden, Kindersuche, Elternnachrichten, Tagesauswertung,
  // Dateien), stehen bewusst NICHT hier: sie tragen mehrere Rollen im Feld
  // `audience` statt eines zweiten, fast gleichen Artikels.
  leadGoLive: "moto-fuer-den-ersten-betreuungstag-vorbereiten",
  leadRooms: "raeume-anlegen",
  leadGroups: "gruppen-anlegen",
  leadActivities: "aktivitaeten-anlegen",
  leadCreateStudent: "kinder-anlegen",
  leadCareTimes: "betreuungszeiten-eintragen",
  leadManageStudent: "angaben-eines-kindes-verwalten",
  leadInviteGuardians: "eltern-einladen",
  leadEndCare: "betreuung-eines-kindes-beenden",
  leadDeleteStudent: "kind-dauerhaft-loeschen",
  leadGradeTransition: "kinder-in-den-naechsten-jahrgang-uebernehmen",
  leadClassListEntries: "kinder-ohne-ogs-betreuung-erfassen",
  leadParentAnnouncement: "elternmitteilung-veroeffentlichen",
  leadParentLetter: "elternbrief-versenden",
  leadParentSurvey: "elternumfrage-erstellen",
  leadMealPlan: "essensplan-veroeffentlichen",
  leadBankDetails: "bankverbindungen-einsehen",
  leadEnrollmentSetup: "anmeldung-vorbereiten",
  leadEnrollmentForm: "anmeldeformular-anlegen",
  leadEnrollmentExport: "anmeldungen-exportieren",
  leadEnrollmentCleanup: "fehlerhafte-anmeldung-loeschen",
  leadInviteStaff: "mitarbeitende-anlegen-und-einladen",
  leadStaffRecord: "personalakte-fuehren",
  leadTargetOverride: "sonderarbeitszeit-eintragen",
  leadRemoveStaff: "person-aus-dem-team-entfernen",
  leadStaffPermissions: "mitarbeitende-und-rechte-verwalten",
  leadTeacherAccess: "lehrkraft-zugang-vorbereiten",
  leadStaffNotices: "tagesinformationen-schreiben",
  leadCalendarPeriods: "schuljahr-und-ferien-eintragen",
  leadCarePlan: "betreuungsplan-erstellen",
  leadDutyRoster: "dienstplan-erstellen",
  leadSubstitutionPlan: "vertretung-planen",
  leadDayLists: "tageslisten-erstellen",
  leadWorkTimeReview: "arbeitszeiten-des-teams-pruefen",
  leadAbsenceReport: "abwesenheiten-auswerten",
  leadStatistics: "statistik-oeffnen",
  leadExports: "liste-exportieren",
  leadPayroll: "abrechnung-vorbereiten",
  leadParentVisibility: "festlegen-was-eltern-sehen",
  leadTabletSetup: "nfc-tablet-aufstellen",
  leadNfcSettings: "einstellen-was-das-tablet-anzeigt",
  leadDevices: "nfc-geraete-verwalten",
  leadInfoDisplays: "info-display-verwalten",
  leadMissingMenu: "person-sieht-einen-menuepunkt-nicht",

  // Eltern-Portal. Das Gerüst folgt der echten Navigation der Eltern-App
  // (`PARENT_PRIMARY_NAV` und `PARENT_MORE_NAV` in
  // components/parent/shell/parent-nav-items.ts), nicht dem Entwurf in der
  // Informationsarchitektur (#2229, Abschnitt 8) -- der kannte Kalender,
  // Mittagessen und Elternbriefe noch nicht. Die Abläufe sind noch nicht
  // Schritt für Schritt gegen die App geprüft.
  parentAccount: "eltern-konto-einrichten",
  parentLogin: "als-elternteil-anmelden",
  parentInstallApp: "moto-als-app-fuer-eltern",
  parentNotifications: "benachrichtigungen-einstellen",
  parentChildOverview: "mein-kind-und-den-heutigen-tag-ansehen",
  parentChildData: "angaben-zu-meinem-kind",
  parentGuardians: "kontakte-und-abholung-verwalten",
  parentReportAbsence: "abwesenheit-meines-kindes-melden",
  parentPickupChange: "abholzeit-fuer-einen-tag-aendern",
  parentCareChange: "aenderung-der-betreuung-anfragen",
  parentDeparture: "weg-nach-hause-aendern",
  parentMessages: "als-elternteil-nachrichten-lesen",
  parentNews: "elternbriefe-lesen",
  parentCalendar: "als-elternteil-den-kalender-ansehen",
  parentMealPlan: "mittagessen-ansehen",
  parentEnroll: "mein-kind-anmelden",
  parentEnrollStatus: "anmeldung-weiter-bearbeiten",
  parentAccountProblem: "eltern-konto-laesst-sich-nicht-einrichten",
  parentChildMissing: "mein-kind-wird-nicht-angezeigt",
  parentFeatureMissing: "funktion-ist-fuer-mich-nicht-verfuegbar",

  // Portal "moto schule" (#2207). Das Gerüst folgt der echten Navigation des
  // Schul-Portals (`SCHOOL_PRIMARY_NAV` in
  // components/school/shell/school-nav-items.ts): Klassenansicht, Meine
  // Aufsichten, Nachrichten, Tagesinformationen. Die Abläufe sind noch nicht
  // Schritt für Schritt gegen die App geprüft.
  teacherAccess: "zugang-zu-moto-schule-einrichten",
  teacherLogin: "bei-moto-schule-anmelden",
  teacherSettings: "als-lehrkraft-benachrichtigungen-einstellen",
  teacherClassDay: "meinen-klassentag-ansehen",
  teacherClassList: "die-liste-einer-klasse-lesen",
  teacherChildDetails: "als-lehrkraft-angaben-zu-einem-kind-ansehen",
  teacherArrivalChange: "ankunftszeit-meiner-klasse-aendern",
  teacherSupervision: "meine-aufsichten-ansehen",
  teacherStartSupervision: "eine-aufsicht-starten",
  teacherSupervisionRoster: "kinder-meiner-aufsicht-sehen",
  teacherMessages: "als-lehrkraft-nachrichten-lesen",
  teacherNotices: "als-lehrkraft-tagesinformationen-lesen",
  teacherLoginProblem: "anmeldung-bei-moto-schule-klappt-nicht",
  teacherNoClasses: "mir-wird-keine-klasse-angezeigt",
  teacherFeatureMissing: "als-lehrkraft-fehlt-mir-ein-bereich",
} as const;

export type HelpTopicId = (typeof HELP_TOPICS)[keyof typeof HELP_TOPICS];

const EXACT_HELP_TOPICS: Readonly<Record<string, HelpTopicId>> = {
  "/activities": HELP_TOPICS.manageActivity,
  "/anfragen": HELP_TOPICS.parentRequests,
  "/calendar": HELP_TOPICS.mySchedule,
  "/dateien": HELP_TOPICS.sharedFiles,
  "/students/search": HELP_TOPICS.studentSearch,
  "/tagesplan": HELP_TOPICS.dayPlan,
  "/ogs-groups": HELP_TOPICS.ownGroups,
  "/active-supervisions": HELP_TOPICS.activeSupervision,
  "/absences": HELP_TOPICS.absences,
  "/day-log": HELP_TOPICS.dayLog,
  "/emergency": HELP_TOPICS.emergency,
  "/rooms": HELP_TOPICS.rooms,
  "/staff": HELP_TOPICS.findStaff,
  "/tagesinformationen": HELP_TOPICS.leadStaffNotices,
  "/time-tracking": HELP_TOPICS.trackWorkTime,
  "/settings": HELP_TOPICS.settings,

  // Leitungsseiten. Sie zeigen heute auf Entwürfe: die Kontexthilfe rendert
  // nur im Schul-Portal (`mode === "teacher"` in dashboard/header.tsx), im
  // Mitarbeiter-Portal ist also noch nichts davon sichtbar.
  "/admin/guardian-approvals": HELP_TOPICS.leadInviteGuardians,
  "/betreuungsplan": HELP_TOPICS.leadCarePlan,
  "/calendar-periods": HELP_TOPICS.leadCalendarPeriods,
  "/dienstplan": HELP_TOPICS.leadDutyRoster,
  "/eltern/bankverbindungen": HELP_TOPICS.leadBankDetails,
  "/info-displays": HELP_TOPICS.leadInfoDisplays,
  "/lists": HELP_TOPICS.leadDayLists,
  "/meal-plan": HELP_TOPICS.leadMealPlan,
  "/parent-announcements": HELP_TOPICS.leadParentAnnouncement,
  "/payroll": HELP_TOPICS.leadPayroll,
  "/statistics": HELP_TOPICS.leadStatistics,
  "/vertretung": HELP_TOPICS.leadSubstitutionPlan,

  // Unterseiten der Datenverwaltung. Ohne diese Einträge fiele jede von
  // ihnen auf das `/database`-Präfix und damit auf den Überblick zurück.
  "/database/activities": HELP_TOPICS.leadActivities,
  "/database/devices": HELP_TOPICS.leadDevices,
  "/database/exports": HELP_TOPICS.leadExports,
  "/database/grade-transitions": HELP_TOPICS.leadGradeTransition,
  "/database/groups": HELP_TOPICS.leadGroups,
  "/database/permissions": HELP_TOPICS.leadStaffPermissions,
  "/database/personal": HELP_TOPICS.leadInviteStaff,
  "/database/roles": HELP_TOPICS.leadStaffPermissions,
  "/database/rooms": HELP_TOPICS.leadRooms,
  "/database/students": HELP_TOPICS.leadCreateStudent,
  "/database/students/ended-care": HELP_TOPICS.leadEndCare,

  // Alte Adressen, die auf eine dokumentierte Seite weiterleiten. Sie stehen
  // in Lesezeichen; der Treffer ist der Artikel des Ziels. `/dashboard` bleibt
  // bewusst draußen: es führt auf die Startseite, und zu der gibt es noch
  // keinen Artikel -- ein allgemeiner wäre geraten, nicht passend.
  "/admin/change-requests": HELP_TOPICS.parentRequests,
  "/invitations": HELP_TOPICS.leadInviteStaff,
  "/planung": HELP_TOPICS.leadCarePlan,
  "/staff/dienstplan": HELP_TOPICS.leadDutyRoster,
  "/timetables": HELP_TOPICS.leadCarePlan,
  "/vertretungsplan": HELP_TOPICS.leadSubstitutionPlan,
};

const PREFIX_HELP_TOPICS: ReadonlyArray<
  readonly [prefix: string, topic: HelpTopicId]
> = [
  ["/students", HELP_TOPICS.studentSearch],
  ["/staff", HELP_TOPICS.findStaff],
  ["/messages", HELP_TOPICS.parentMessage],
  ["/parent-announcements", HELP_TOPICS.leadParentAnnouncement],
  ["/rooms", HELP_TOPICS.rooms],
  ["/team-chat", HELP_TOPICS.teamChat],
  // Der erste Treffer gewinnt, nicht der längste: die Unterbereiche der
  // Datenverwaltung müssen vor `/database` stehen.
  ["/database/students/class-list", HELP_TOPICS.leadClassListEntries],
  ["/database/students/import", HELP_TOPICS.leadCreateStudent],
  ["/database/personal", HELP_TOPICS.leadInviteStaff],
  ["/database", HELP_TOPICS.dataManagement],
  ["/admin/enrollments", HELP_TOPICS.enrollments],
  ["/enrollment-phases", HELP_TOPICS.enrollments],
  ["/care-offerings", HELP_TOPICS.enrollments],
  ["/enrollment-form", HELP_TOPICS.leadEnrollmentForm],
];

/**
 * Routen des Elternportals. Auf der Eltern-Subdomain laufen die Seiten
 * ohne `/parents`-Praefix (`/children`), im Tenant-Kontext mit
 * (`/parents/anmeldung/...`). `getParentHelpTopicForPath` schneidet das
 * Praefix deshalb ab, bevor es hier nachschlaegt.
 *
 * `/` bleibt bewusst draussen: die Startseite fasst nur zusammen, was
 * anderswo steht. Ein Artikel dazu waere geraten, nicht passend.
 */
const PARENT_EXACT_HELP_TOPICS: Readonly<Record<string, HelpTopicId>> = {
  "/messages": HELP_TOPICS.parentMessages,
  "/calendar": HELP_TOPICS.parentCalendar,
  "/news": HELP_TOPICS.parentNews,
  "/meal-plan": HELP_TOPICS.parentMealPlan,
  // Die Seite traegt Sprache und Benachrichtigungen. Der Artikel dazu
  // ist `Benachrichtigungen einstellen`.
  "/settings": HELP_TOPICS.parentNotifications,
};

const PARENT_PREFIX_HELP_TOPICS: ReadonlyArray<
  readonly [prefix: string, topic: HelpTopicId]
> = [
  // `/children/123` fuehrt auf dieselbe Seite wie `/children`.
  ["/children", HELP_TOPICS.parentChildOverview],
  // Die Anmeldung laeuft ueber `/anmeldung/<schule>/<phase>`.
  ["/anmeldung", HELP_TOPICS.parentEnroll],
];

/**
 * Routen von moto schule. Auf dem Schul-Host laufen sie ohne `/school`-
 * Praefix; nach dem Proxy-Rewrite kann der interne Pfad das Praefix tragen.
 */
const SCHOOL_EXACT_HELP_TOPICS: Readonly<Record<string, HelpTopicId>> = {
  "/": HELP_TOPICS.teacherClassDay,
  "/klasse": HELP_TOPICS.teacherClassList,
  "/aufsichten": HELP_TOPICS.teacherSupervision,
  "/tagesinformationen": HELP_TOPICS.teacherNotices,
  "/einstellungen": HELP_TOPICS.teacherSettings,
};

const SCHOOL_PREFIX_HELP_TOPICS: ReadonlyArray<
  readonly [prefix: string, topic: HelpTopicId]
> = [["/nachrichten", HELP_TOPICS.teacherMessages]];

/** Finds the help article for a page of the parents portal. */
export function getParentHelpTopicForPath(
  pathname: string,
): HelpTopicId | null {
  const path = pathname.replace(/^\/parents(?=\/|$)/, "") || "/";
  const exactTopic = PARENT_EXACT_HELP_TOPICS[path];
  if (exactTopic) return exactTopic;

  for (const [prefix, topic] of PARENT_PREFIX_HELP_TOPICS) {
    if (matchesPathPrefix(path, prefix)) return topic;
  }
  return null;
}

/** Finds the help article for a page of moto schule. */
export function getSchoolHelpTopicForPath(
  pathname: string,
): HelpTopicId | null {
  const path = pathname.replace(/^\/school(?=\/|$)/, "") || "/";
  const exactTopic = SCHOOL_EXACT_HELP_TOPICS[path];
  if (exactTopic) return exactTopic;

  for (const [prefix, topic] of SCHOOL_PREFIX_HELP_TOPICS) {
    if (matchesPathPrefix(path, prefix)) return topic;
  }
  return null;
}

/** Finds the most specific existing help article for a staff-app page. */
export function getHelpTopicForPath(pathname: string): HelpTopicId | null {
  const exactTopic = EXACT_HELP_TOPICS[pathname];
  if (exactTopic) return exactTopic;

  for (const [prefix, topic] of PREFIX_HELP_TOPICS) {
    if (matchesPathPrefix(pathname, prefix)) return topic;
  }
  return null;
}
