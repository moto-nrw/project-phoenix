import {
  HELP_TOPICS,
  type HelpRole,
  type HelpTopicId,
} from "~/lib/help-topics";

// Ob es zu einer Seite eine Anleitung für die Rolle gibt, fragt das
// Fragezeichen im Seitenkopf auf jeder Seite. Die Antwort braucht nur, wer
// einen Artikel sieht und welche Einstellung ihn ausblendet, nicht die
// Artikeltexte: `help-content.ts` wiegt komprimiert über 80 kB und lag über
// den Seitenkopf in jeder Route (Bundle-Ratchet #2939). Diese Datei hält
// deshalb Zielgruppe und Ausblendregeln; `help-content.ts` nutzt dieselben
// Regeln, `help-topic-availability.test.ts` prüft, dass beide übereinstimmen.

export type HelpPresenceMode = "detailed" | "binary" | "unknown";
export type HelpGroupMode = "fixed_groups" | "open_care" | "unknown";

/**
 * Wer einen Artikel in Seitenleiste und Suche sieht. Eine Liste, wo mehrere
 * Rollen denselben Ablauf haben. `"all"` heißt jede Rolle.
 */
export type HelpAudience = "all" | HelpRole | readonly HelpRole[];

/** Sieht `role` einen Artikel mit dieser Zielgruppe? */
export function helpAudienceIncludes(
  audience: HelpAudience,
  role: HelpRole,
): boolean {
  if (audience === "all") return true;
  if (Array.isArray(audience)) return audience.includes(role);
  return audience === role;
}

// Themen, die es ohne NFC in der OGS schlicht nicht gibt: das Tablet, die
// Armbaender und der Bereich `Aktivitaeten`, den moto nur mit NFC anzeigt.
// Sagt eine OGS ausdruecklich "kein NFC", verschwinden sie aus Seitenleiste
// und Suche, statt eine Nichtverfuegbarkeit zu erklaeren.
//
// Nur bei einem ausdruecklichen `false`. Ist die Arbeitsweise unbekannt
// (`null`), bleiben die Themen stehen und erklaeren, wovon sie abhaengen --
// sonst verschwiegen wir jemandem ein Thema, den wir nie gefragt haben.
const NFC_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.tabletLogin,
  HELP_TOPICS.tagAssignment,
  HELP_TOPICS.nfcWorkTime,
  HELP_TOPICS.nfcSupervision,
  HELP_TOPICS.nfcCheckIn,
  HELP_TOPICS.nfcProblem,
  HELP_TOPICS.manageActivity,
  // Ohne NFC hat die OGS keine Geraete, die sie verwalten koennte -- und der
  // Aktivitaetenkatalog fehlt ihr ebenfalls: beide Seiten stehen in
  // `NFC_ONLY_HREFS` (dashboard/sidebar.tsx).
  HELP_TOPICS.leadDevices,
  HELP_TOPICS.leadActivities,
  HELP_TOPICS.leadTabletSetup,
  HELP_TOPICS.leadNfcSettings,
]);

// Dasselbe fuer die einfache Anwesenheit: haelt eine OGS nur fest, ob ein
// Kind da ist, gibt es weder Raeume noch Aufsichten noch Aktivitaeten -- und
// damit auch keinen Aufenthaltsort, den man aendern koennte. Das An- und
// Abmelden steht als eigenes Thema in der Liste.
//
// Wieder nur bei einem ausdruecklichen "binary". Bei unbekannter
// Arbeitsweise bleiben die Themen stehen und erklaeren, wovon sie abhaengen.
const DETAILED_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.rooms,
  HELP_TOPICS.activeSupervision,
  // Die Lernzeit führt man nur in `Aktuelle Aufsicht`.
  HELP_TOPICS.homeworkFeedback,
  HELP_TOPICS.manageActivity,
  HELP_TOPICS.changeLocation,
  // Der Tagesplan liegt hinter dem BinaryModeGuard
  // (`app/[tenant]/(protected)/tagesplan/page.tsx`).
  HELP_TOPICS.dayPlan,
]);

// Und dasselbe fuer die offene Betreuung: ohne feste eigene Gruppen gibt es
// weder `Gruppen` noch eine Gruppe, die man uebergeben koennte.
const FIXED_GROUPS_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.ownGroups,
  HELP_TOPICS.transferGroup,
]);

/** Artikel, die eine Einstellung der OGS ausblendet. */
export function hiddenHelpTopicIds(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): ReadonlySet<HelpTopicId> {
  const hidden = new Set<HelpTopicId>();
  if (nfcEnabled === false) {
    for (const id of NFC_ONLY_TOPIC_IDS) hidden.add(id);
  }
  if (presenceMode === "binary") {
    for (const id of DETAILED_ONLY_TOPIC_IDS) hidden.add(id);
  }
  if (groupMode === "open_care") {
    for (const id of FIXED_GROUPS_ONLY_TOPIC_IDS) hidden.add(id);
  }
  return hidden;
}

/**
 * Zielgruppe jedes Artikels, gleich dem `audience` des Artikels in
 * `help-content.ts`. Ein neuer Artikel braucht hier einen Eintrag; der Typ
 * verlangt ihn, der Test prüft den Wert.
 */
export const HELP_TOPIC_AUDIENCE = {
  [HELP_TOPICS.acceptInvitation]: ["caregiver", "lead"],
  [HELP_TOPICS.login]: ["caregiver", "lead"],
  [HELP_TOPICS.installApp]: ["caregiver", "lead"],
  [HELP_TOPICS.appOverview]: ["caregiver", "lead"],
  [HELP_TOPICS.caregiverFirstSteps]: "caregiver",
  [HELP_TOPICS.mySchedule]: ["caregiver", "lead"],
  [HELP_TOPICS.carePlan]: ["caregiver", "lead"],
  [HELP_TOPICS.dayPlan]: "caregiver",
  [HELP_TOPICS.studentSearch]: ["caregiver", "lead"],
  [HELP_TOPICS.editStudent]: "caregiver",
  [HELP_TOPICS.webAttendance]: "caregiver",
  [HELP_TOPICS.holidayGuests]: ["caregiver", "lead"],
  [HELP_TOPICS.changeLocation]: "caregiver",
  [HELP_TOPICS.dayLog]: ["caregiver", "lead"],
  [HELP_TOPICS.ownGroups]: ["caregiver", "lead"],
  [HELP_TOPICS.transferGroup]: ["caregiver", "lead"],
  [HELP_TOPICS.rooms]: ["caregiver", "lead"],
  [HELP_TOPICS.activeSupervision]: ["caregiver", "lead"],
  [HELP_TOPICS.homeworkFeedback]: ["caregiver", "lead"],
  [HELP_TOPICS.manageActivity]: "caregiver",
  [HELP_TOPICS.absences]: "caregiver",
  [HELP_TOPICS.emergency]: ["caregiver", "lead"],
  [HELP_TOPICS.parentMessage]: ["caregiver", "lead"],
  [HELP_TOPICS.parentGroupMessage]: ["caregiver", "lead"],
  [HELP_TOPICS.parentRequests]: ["caregiver", "lead"],
  [HELP_TOPICS.teamChat]: ["caregiver", "lead"],
  [HELP_TOPICS.staffSurveys]: ["caregiver", "lead"],
  [HELP_TOPICS.findStaff]: ["caregiver", "lead"],
  [HELP_TOPICS.sharedFiles]: ["caregiver", "lead"],
  [HELP_TOPICS.trackWorkTime]: ["caregiver", "lead"],
  [HELP_TOPICS.ownShiftPlan]: ["caregiver", "lead"],
  [HELP_TOPICS.correctWorkTime]: ["caregiver", "lead"],
  [HELP_TOPICS.workTimeHints]: ["caregiver", "lead"],
  [HELP_TOPICS.vacation]: ["caregiver", "lead"],
  [HELP_TOPICS.ownAbsence]: ["caregiver", "lead"],
  [HELP_TOPICS.tabletLogin]: ["caregiver", "lead"],
  [HELP_TOPICS.tagAssignment]: ["caregiver", "lead"],
  [HELP_TOPICS.nfcWorkTime]: ["caregiver", "lead"],
  [HELP_TOPICS.nfcSupervision]: ["caregiver", "lead"],
  [HELP_TOPICS.nfcCheckIn]: ["caregiver", "lead"],
  [HELP_TOPICS.missingMenu]: "caregiver",
  [HELP_TOPICS.missingChildOrGroup]: "caregiver",
  [HELP_TOPICS.attendanceProblem]: "caregiver",
  [HELP_TOPICS.nfcProblem]: ["caregiver", "lead"],
  [HELP_TOPICS.loginProblem]: ["caregiver", "lead"],
  [HELP_TOPICS.errorMessages]: "all",
  [HELP_TOPICS.analyticsConsent]: ["caregiver", "lead", "parent", "teacher"],
  [HELP_TOPICS.dataManagement]: "lead",
  [HELP_TOPICS.enrollments]: "lead",
  [HELP_TOPICS.settings]: "lead",
  [HELP_TOPICS.settingsGuide]: "lead",
  [HELP_TOPICS.leadFirstSteps]: "lead",
  [HELP_TOPICS.leadGoLive]: "lead",
  [HELP_TOPICS.leadRooms]: "lead",
  [HELP_TOPICS.leadGroups]: "lead",
  [HELP_TOPICS.leadActivities]: "lead",
  [HELP_TOPICS.leadCreateStudent]: "lead",
  [HELP_TOPICS.leadCareTimes]: "lead",
  [HELP_TOPICS.leadManageStudent]: "lead",
  [HELP_TOPICS.studentNotes]: ["caregiver", "lead"],
  [HELP_TOPICS.leadInviteGuardians]: "lead",
  [HELP_TOPICS.leadEndCare]: "lead",
  [HELP_TOPICS.leadDeleteStudent]: "lead",
  [HELP_TOPICS.leadGradeTransition]: "lead",
  [HELP_TOPICS.leadClassListEntries]: "lead",
  [HELP_TOPICS.leadParentAnnouncement]: "lead",
  [HELP_TOPICS.leadParentLetter]: "lead",
  [HELP_TOPICS.leadParentSurvey]: "lead",
  [HELP_TOPICS.leadSurveys]: "lead",
  [HELP_TOPICS.leadParentDeclaration]: "lead",
  [HELP_TOPICS.leadMealPlan]: "lead",
  [HELP_TOPICS.leadMealOrdering]: "lead",
  [HELP_TOPICS.leadMealCaterer]: "lead",
  [HELP_TOPICS.leadBankDetails]: "lead",
  [HELP_TOPICS.leadEnrollmentSetup]: "lead",
  [HELP_TOPICS.leadEnrollmentForm]: "lead",
  [HELP_TOPICS.leadEnrollmentExport]: "lead",
  [HELP_TOPICS.leadEnrollmentCleanup]: "lead",
  [HELP_TOPICS.leadInviteStaff]: "lead",
  [HELP_TOPICS.leadStaffRecord]: "lead",
  [HELP_TOPICS.leadTargetOverride]: "lead",
  [HELP_TOPICS.leadRemoveStaff]: "lead",
  [HELP_TOPICS.leadStaffPermissions]: "lead",
  [HELP_TOPICS.leadTeacherAccess]: "lead",
  [HELP_TOPICS.leadStaffNotices]: "lead",
  [HELP_TOPICS.leadCalendarPeriods]: "lead",
  [HELP_TOPICS.leadCarePlan]: "lead",
  [HELP_TOPICS.leadRosterDeviation]: "lead",
  [HELP_TOPICS.leadCancelBlock]: "lead",
  [HELP_TOPICS.leadDutyRoster]: "lead",
  [HELP_TOPICS.leadSubstitutionPlan]: "lead",
  [HELP_TOPICS.leadDayLists]: "lead",
  [HELP_TOPICS.leadHolidayWeeks]: "lead",
  [HELP_TOPICS.leadHolidayGuestGroup]: "lead",
  [HELP_TOPICS.leadWorkTimeReview]: "lead",
  [HELP_TOPICS.leadAbsenceReport]: "lead",
  [HELP_TOPICS.leadStatistics]: "lead",
  [HELP_TOPICS.leadExports]: "lead",
  [HELP_TOPICS.leadPayroll]: "lead",
  [HELP_TOPICS.leadParentVisibility]: "lead",
  [HELP_TOPICS.leadTabletSetup]: "lead",
  [HELP_TOPICS.leadNfcSettings]: "lead",
  [HELP_TOPICS.leadDevices]: "lead",
  [HELP_TOPICS.leadInfoDisplays]: "lead",
  [HELP_TOPICS.leadBirthdays]: "lead",
  [HELP_TOPICS.leadMissingMenu]: "lead",
  [HELP_TOPICS.leadModules]: "lead",
  [HELP_TOPICS.parentAccount]: "parent",
  [HELP_TOPICS.parentLogin]: "parent",
  [HELP_TOPICS.parentFirstSteps]: "parent",
  [HELP_TOPICS.parentInstallApp]: "parent",
  [HELP_TOPICS.parentNotifications]: "parent",
  [HELP_TOPICS.parentChildOverview]: "parent",
  [HELP_TOPICS.parentChildData]: "parent",
  [HELP_TOPICS.parentGuardians]: "parent",
  [HELP_TOPICS.parentReportAbsence]: "parent",
  [HELP_TOPICS.parentPickupChange]: "parent",
  [HELP_TOPICS.parentCareChange]: "parent",
  [HELP_TOPICS.parentDeparture]: "parent",
  [HELP_TOPICS.parentMessages]: "parent",
  [HELP_TOPICS.parentGroupMessages]: "parent",
  [HELP_TOPICS.parentNews]: "parent",
  [HELP_TOPICS.parentSurveys]: "parent",
  [HELP_TOPICS.parentCalendar]: "parent",
  [HELP_TOPICS.parentMealPlan]: "parent",
  [HELP_TOPICS.parentDocuments]: "parent",
  [HELP_TOPICS.parentEnroll]: "parent",
  [HELP_TOPICS.parentEnrollStatus]: "parent",
  [HELP_TOPICS.parentHolidayRegistration]: "parent",
  [HELP_TOPICS.parentAccountProblem]: "parent",
  [HELP_TOPICS.parentChildMissing]: "parent",
  [HELP_TOPICS.parentFeatureMissing]: "parent",
  [HELP_TOPICS.teacherAccess]: "teacher",
  [HELP_TOPICS.teacherLogin]: "teacher",
  [HELP_TOPICS.traegerLogin]: "traeger",
  [HELP_TOPICS.traegerTestSystem]: "traeger",
  [HELP_TOPICS.traegerInvitation]: "traeger",
  [HELP_TOPICS.traegerAccounts]: "traeger",
  [HELP_TOPICS.traegerHolidayPeriod]: "traeger",
  [HELP_TOPICS.traegerHolidayRegistration]: "traeger",
  [HELP_TOPICS.traegerHolidayForm]: "traeger",
  [HELP_TOPICS.traegerHolidayRegistrationReview]: "traeger",
  [HELP_TOPICS.traegerHolidayContribution]: "traeger",
  [HELP_TOPICS.traegerBillingRun]: "traeger",
  [HELP_TOPICS.traegerAccessLog]: "traeger",
  [HELP_TOPICS.traegerSettings]: "traeger",
  [HELP_TOPICS.traegerBriefkopf]: "traeger",
  [HELP_TOPICS.traegerSchoolSwitch]: "traeger",
  [HELP_TOPICS.teacherSettings]: "teacher",
  [HELP_TOPICS.teacherClassDay]: "teacher",
  [HELP_TOPICS.teacherClassList]: "teacher",
  [HELP_TOPICS.teacherMorningAttendance]: "teacher",
  [HELP_TOPICS.teacherHomework]: "teacher",
  [HELP_TOPICS.teacherClassHandover]: "teacher",
  [HELP_TOPICS.teacherChildDetails]: "teacher",
  [HELP_TOPICS.teacherArrivalChange]: "teacher",
  [HELP_TOPICS.teacherClassTrip]: "teacher",
  [HELP_TOPICS.teacherAbsenceReport]: "teacher",
  [HELP_TOPICS.teacherDayHint]: "teacher",
  [HELP_TOPICS.teacherSharedNotes]: "teacher",
  [HELP_TOPICS.teacherSupervision]: "teacher",
  [HELP_TOPICS.teacherStartSupervision]: "teacher",
  [HELP_TOPICS.teacherSupervisionRoster]: "teacher",
  [HELP_TOPICS.teacherMessages]: "teacher",
  [HELP_TOPICS.teacherNotices]: "teacher",
  [HELP_TOPICS.teacherCalendar]: "teacher",
  [HELP_TOPICS.teacherParentLetters]: "teacher",
  [HELP_TOPICS.teacherLoginProblem]: "teacher",
  [HELP_TOPICS.teacherNoClasses]: "teacher",
  [HELP_TOPICS.teacherFeatureMissing]: "teacher",
} as const satisfies Readonly<Record<HelpTopicId, HelpAudience>>;

/** Gibt es den Artikel `topic` für `role` bei diesen Einstellungen? */
export function isHelpTopicAvailable(
  topic: HelpTopicId,
  role: HelpRole,
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): boolean {
  if (hiddenHelpTopicIds(presenceMode, groupMode, nfcEnabled).has(topic)) {
    return false;
  }
  return helpAudienceIncludes(HELP_TOPIC_AUDIENCE[topic], role);
}
