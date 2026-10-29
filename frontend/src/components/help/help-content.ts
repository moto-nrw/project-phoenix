import * as articles from "./help-articles.generated";
import type { HelpTopic, HelpTopicGroup } from "./help-types";
export type { HelpTopic, HelpTopicGroup } from "./help-types";
import {
  hiddenHelpTopicIds,
  helpAudienceIncludes,
  type HelpGroupMode,
  type HelpPresenceMode,
} from "~/lib/help-topic-availability";
import type { HelpRole } from "~/lib/help-topics";

// Die Rolle ist ein Navigationsbegriff, kein Inhalt: sie liegt in
// `help-topics.ts`, damit die Seitenleiste eine Hilfe-Adresse bauen kann,
// ohne die Artikeltexte zu laden. Hier nur weitergereicht, damit die
// bestehenden Importe aus dieser Datei weiter stimmen.
export type { HelpRole };
export type { HelpGroupMode, HelpPresenceMode };

export const HELP_ROLES = [
  "caregiver",
  "lead",
  "parent",
  "teacher",
  "traeger",
] as const satisfies readonly HelpRole[];

/** Sieht `role` diesen Artikel? Einzige Stelle, die `audience` auswertet. */
export function helpTopicMatchesRole(
  topic: HelpTopic,
  role: HelpRole,
): boolean {
  return helpAudienceIncludes(topic.audience, role);
}

/**
 * Die Themen des laufenden Betriebs. Die Reihenfolge einer Gruppe in
 * Seitenleiste und Gruppenseite folgt dieser Liste, deshalb stehen die zwei
 * Leitungsthemen zum NFC-Gerät hier und nicht in `leadTopics`: ohne
 * gesetzte `Geräte-PIN` kommt am Tablet niemand weiter, und stünden sie
 * am Ende, müsste die Leitung zuerst nach unten springen. Die Betreuung
 * sieht beide nicht.
 */
function caregiverTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly HelpTopic[] {
  return [
    articles.myScheduleTopic(),
    articles.carePlanTopic(presenceMode),
    articles.dayPlanTopic(presenceMode),
    articles.studentSearchTopic(presenceMode, groupMode),
    articles.editStudentTopic(),
    articles.webAttendanceTopic(presenceMode),
    articles.changeLocationTopic(presenceMode),
    articles.absencesTopic(),
    articles.dayLogTopic(groupMode),
    articles.emergencyTopic(presenceMode),
    articles.ownGroupsTopic(presenceMode, groupMode),
    articles.transferGroupTopic(groupMode),
    articles.holidayGuestsTopic(),
    articles.roomsTopic(presenceMode),
    articles.activeSupervisionTopic(presenceMode),
    articles.homeworkFeedbackTopic(),
    articles.manageActivityTopic(presenceMode, nfcEnabled),
    articles.parentMessageTopic(),
    articles.parentGroupMessageTopic(),
    articles.parentRequestsTopic(groupMode),
    articles.teamChatTopic(),
    articles.staffSurveysTopic(),
    articles.findStaffTopic(presenceMode),
    articles.sharedFilesTopic(),
    articles.trackWorkTimeTopic(),
    articles.correctWorkTimeTopic(),
    articles.workTimeHintsTopic(),
    articles.ownShiftPlanTopic(),
    articles.vacationTopic(),
    articles.ownAbsenceTopic(),

    // Nur für die Leitung, und bewusst vor den fünf Themen zum täglichen
    // Umgang: Gerät prüfen und PIN setzen kommen davor.
    articles.devicesTopic(),
    articles.nfcSettingsTopic(presenceMode),
    articles.tabletLoginTopic(nfcEnabled),
    articles.tagAssignmentTopic(nfcEnabled),
    articles.nfcWorkTimeTopic(nfcEnabled),
    articles.nfcSupervisionTopic(presenceMode, nfcEnabled),
    articles.nfcCheckInTopic(presenceMode, nfcEnabled),
    articles.missingMenuTopic(presenceMode, groupMode, nfcEnabled),
    articles.missingChildOrGroupTopic(groupMode),
    articles.attendanceProblemTopic(presenceMode),
    articles.nfcProblemTopic(nfcEnabled),
    articles.loginProblemTopic(nfcEnabled),
  ];
}

function leadTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly HelpTopic[] {
  return [
    // --- moto fuer die OGS einrichten ---
    articles.firstStepsTopic(),
    articles.goLiveTopic(presenceMode, groupMode, nfcEnabled),
    articles.dataManagementTopic(),
    articles.roomsCatalogTopic(presenceMode),
    articles.inviteStaffTopic(),
    articles.groupsCatalogTopic(groupMode),
    articles.activitiesCatalogTopic(),
    articles.createStudentTopic(),
    articles.careTimesTopic(),
    articles.tabletSetupTopic(),

    // --- Kinder verwalten ---
    articles.manageStudentTopic(),
    articles.studentNotesTopic(),
    articles.endCareTopic(),
    articles.deleteStudentTopic(),
    articles.gradeTransitionTopic(),
    articles.classListEntriesTopic(),

    // --- Eltern und Anfragen ---
    // Zuerst der Zugang: ohne Konto sieht eine Familie weder Mitteilung
    // noch Elternbrief noch Essensplan.
    articles.inviteGuardiansTopic(),
    articles.parentAnnouncementTopic(),
    articles.parentLetterTopic(),
    articles.parentSurveyTopic(),
    articles.leadSurveysTopic(),
    articles.parentDeclarationTopic(),
    articles.mealPlanTopic(),
    articles.mealOrderingTopic(),
    articles.mealCatererTopic(),
    articles.bankDetailsTopic(),

    // --- Anmeldungen ---
    articles.enrollmentSetupTopic(),
    articles.enrollmentFormTopic(),
    articles.enrollmentReviewTopic(),
    articles.enrollmentExportTopic(),
    articles.enrollmentCleanupTopic(),

    // --- Personalverwaltung ---
    articles.staffRecordTopic(),
    articles.targetOverrideTopic(),
    articles.staffPermissionsTopic(),
    articles.workTimeReviewTopic(),
    articles.payrollTopic(),
    articles.removeStaffTopic(),
    articles.leadTeacherAccessTopic(),

    // --- Betreuung und Team planen ---
    articles.calendarPeriodsTopic(),
    articles.leadCarePlanTopic(),
    articles.leadRosterDeviationTopic(),
    articles.leadCancelBlockTopic(),
    articles.dutyRosterTopic(),
    articles.substitutionPlanTopic(),
    articles.dayListsTopic(),
    articles.staffNoticesTopic(),

    // --- Auswerten und exportieren ---
    articles.absenceReportTopic(),
    articles.statisticsTopic(),
    articles.careTimeComparisonTopic(),
    articles.holidayWeeksTopic(),
    articles.holidayGuestGroupTopic(),
    articles.exportsTopic(),

    // --- Einstellungen und Geräte ---
    articles.settingsTopic(),
    articles.settingsGuideTopic(),
    articles.modulesTopic(),
    articles.parentVisibilityTopic(),
    articles.infoDisplaysTopic(),
    articles.leadBirthdaysTopic(),

    // --- Wenn etwas nicht klappt ---
    articles.leadMissingMenuTopic(),
  ];
}

// Eltern-Portal. Geruest nach der echten Navigation der Eltern-App
// (`PARENT_PRIMARY_NAV` und `PARENT_MORE_NAV`): Start, Kinder, Nachrichten,
// Kalender, dahinter Elternbriefe, Mittagessen, Einstellungen und Anmeldung.
//
// Die Eltern-App ist ein eigenes Portal mit eigener Adresse. Themen der
// Betreuung werden hier NICHT geteilt: Eltern melden sich woanders an, sehen
// andere Seiten und duerfen vieles nur anfragen statt aendern.
//
// Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App geprueft,
// deshalb bleiben es Entwuerfe.
const PARENT_DRAFT_TOPICS: readonly HelpTopic[] = [
  // --- Einstieg und Konto ---
  articles.parentAccountTopic(),
  articles.parentLoginTopic(),
  articles.parentFirstStepsTopic(),
  articles.parentInstallAppTopic(),
  articles.parentNotificationsTopic(),

  // --- Mein Kind und die Betreuung ---
  articles.parentChildOverviewTopic(),
  articles.parentChildDataTopic(),
  articles.parentGuardiansTopic(),
  articles.parentReportAbsenceTopic(),
  articles.parentPickupChangeTopic(),
  articles.parentCareChangeTopic(),
  articles.parentDepartureTopic(),

  // --- Nachrichten und Infos ---
  articles.parentMessagesTopic(),
  articles.parentGroupMessagesTopic(),
  articles.parentNewsTopic(),
  articles.parentSurveysTopic(),
  articles.parentCalendarTopic(),
  articles.parentMealPlanTopic(),
  articles.parentDocumentsTopic(),

  // --- Anmeldung ---
  articles.parentEnrollTopic(),
  articles.parentEnrollStatusTopic(),
  articles.parentHolidayRegistrationTopic(),

  // --- Wenn etwas nicht klappt ---
  articles.parentAccountProblemTopic(),
  articles.parentChildMissingTopic(),
  articles.parentFeatureMissingTopic(),
] as const;

const TRAEGER_TOPICS: readonly HelpTopic[] = [
  articles.traegerInvitationTopic(),
  articles.traegerLoginTopic(),
  articles.traegerSchoolSwitchTopic(),
  articles.traegerTestSystemTopic(),
  articles.traegerAccountsTopic(),
  articles.traegerHolidayPeriodTopic(),
  articles.traegerHolidayFormTopic(),
  articles.traegerHolidayRegistrationTopic(),
  articles.traegerHolidayRegistrationReviewTopic(),
  articles.traegerHolidayContributionTopic(),
  articles.traegerBillingRunTopic(),
  articles.traegerAccessLogTopic(),
  articles.traegerBriefkopfTopic(),
  articles.traegerSettingsTopic(),
];

const TEACHER_DRAFT_TOPICS: readonly HelpTopic[] = [
  // --- Einstieg ---
  articles.teacherAccessTopic(),
  articles.teacherLoginTopic(),
  articles.teacherSettingsTopic(),

  // --- Meine Klasse ---
  articles.teacherClassDayTopic(),
  articles.teacherClassListTopic(),
  articles.teacherMorningAttendanceTopic(),
  articles.teacherHomeworkTopic(),
  articles.teacherClassHandoverTopic(),
  articles.teacherChildDetailsTopic(),
  articles.teacherArrivalChangeTopic(),
  articles.teacherDayHintTopic(),
  articles.teacherSharedNotesTopic(),
  articles.teacherClassTripTopic(),
  articles.teacherAbsenceReportTopic(),

  // --- Aufsicht ---
  articles.teacherSupervisionTopic(),
  articles.teacherStartSupervisionTopic(),
  articles.teacherSupervisionRosterTopic(),

  // --- Nachrichten und Infos ---
  articles.teacherMessagesTopic(),
  articles.teacherNoticesTopic(),
  articles.teacherParentLettersTopic(),
  articles.teacherCalendarTopic(),

  // --- Wenn etwas nicht klappt ---
  articles.teacherLoginProblemTopic(),
  articles.teacherNoClassesTopic(),
  articles.teacherFeatureMissingTopic(),
] as const;

/** So viele Themen zeigt die Startseite einer Rolle als Kacheln. */
export const HOME_TOPIC_COUNT = 6;

export function getHelpTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode = "fixed_groups",
  nfcEnabled: boolean | null = true,
): readonly HelpTopic[] {
  const topics = [
    articles.invitationTopic(),
    articles.loginTopic(presenceMode, groupMode),
    articles.installAppTopic(),
    articles.appOverviewTopic(),
    articles.caregiverFirstStepsTopic(),
    ...caregiverTopics(presenceMode, groupMode, nfcEnabled),
    ...leadTopics(presenceMode, groupMode, nfcEnabled),
    ...PARENT_DRAFT_TOPICS,
    ...TEACHER_DRAFT_TOPICS,
    ...TRAEGER_TOPICS,
    articles.errorMessagesTopic(),
    articles.analyticsConsentTopic(),
  ];

  const hidden = hiddenHelpTopicIds(presenceMode, groupMode, nfcEnabled);
  if (hidden.size === 0) return topics;

  return topics.filter((topic) => !hidden.has(topic.id));
}

/**
 * Ueberschrift einer Themengruppe fuer eine konkrete Arbeitsweise.
 *
 * Die Ueberschrift darf nur nennen, was in der Gruppe auch steht. Bei
 * einfacher Anwesenheit sind Raeume und Aufsichten ausgeblendet, bei offener
 * Betreuung die Gruppen -- sonst sucht jemand in der Gruppe nach etwas, das
 * dort nie stehen wird.
 */
export function helpGroupLabel(
  group: HelpTopicGroup,
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): string {
  if (group === "gruppen") {
    const hasGroups = groupMode !== "open_care";
    const hasRooms = presenceMode !== "binary";
    if (hasGroups && !hasRooms) return "Gruppen";
    if (!hasGroups && hasRooms) return "Räume und Aufsicht";
    // Fehlt beides, ist die Gruppe leer und die Ueberschrift entfaellt
    // ohnehin -- der Rueckfall bleibt trotzdem sinnvoll.
  }
  return HELP_GROUP_LABELS[group];
}

export const HELP_GROUPS = [
  "einstieg",
  "tagesplanung",
  "kinder",
  "gruppen",
  "team",
  "arbeitszeit",
  "nfc",
  "probleme",
  "einrichten",
  "kinderdaten",
  "elternarbeit",
  "anmeldeverwaltung",
  "personal",
  "planung",
  "auswertung",
  "konfiguration",
  "mein-kind",
  "nachrichten",
  "anmeldung",
  "klasse",
  "aufsicht",
] as const satisfies readonly HelpTopicGroup[];

export const HELP_GROUP_LABELS: Readonly<Record<HelpTopicGroup, string>> = {
  einstieg: "Einstieg",
  tagesplanung: "Tagesplanung",
  kinder: "Kinder und Anwesenheit",
  gruppen: "Gruppen, Räume und Aufsicht",
  team: "Mit Eltern und dem Team arbeiten",
  arbeitszeit: "Meine Arbeitszeit",
  nfc: "NFC-Tablet und Armbänder",
  probleme: "Wenn etwas nicht klappt",
  einrichten: "moto für die OGS einrichten",
  kinderdaten: "Kinder verwalten",
  elternarbeit: "Eltern und Anfragen",
  anmeldeverwaltung: "Anmeldungen",
  personal: "Personalverwaltung",
  planung: "Betreuung und Team planen",
  auswertung: "Auswerten und exportieren",
  konfiguration: "Einstellungen",
  "mein-kind": "Mein Kind und die Betreuung",
  // Traegt bei den Eltern auch Elternbriefe, Kalender und Mittagessen.
  nachrichten: "Nachrichten und Infos",
  anmeldung: "Anmeldung",
  klasse: "Meine Klasse",
  aufsicht: "Aufsicht",
};
