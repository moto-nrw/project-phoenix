import { encodeMultiValueParam } from "~/lib/multi-value-param";
import {
  SEARCH_STUDENTS_KEY_PREFIX,
  searchStudentsGroupScope,
} from "./search-students-key";
import { previewRow, previewText, type ListPreview } from "./list-preview";
import { resourceKey } from "./resource-key";
import { groupKeys } from "./groups-keys";

export { announcementKeys, dashboardKeys } from "./shell-keys";
export { parentKeys } from "./parent-keys";

const operatorSummaryPrefixes = [
  "operator-school-summaries",
  "operator-organization-summaries",
  "operator-organization-school-summaries-",
] as const;

/** Unscoped family factories. Only the shared hooks turn these into cache keys. */
export const operatorProvisioningKeys = {
  summaryChanges: (key: string): boolean =>
    operatorSummaryPrefixes.some((prefix) => key.startsWith(prefix)),
  personChanges: (key: string): boolean =>
    key.startsWith("operator-school-persons-") ||
    key.startsWith("operator-org-persons-") ||
    operatorSummaryPrefixes.some((prefix) => key.startsWith(prefix)),
  accountFamilies: [
    "operator-all-accounts",
    "operator-school-accounts-",
    "operator-org-accounts-",
    ...operatorSummaryPrefixes,
  ],
  deviceFamilies: [
    "operator-all-devices",
    "operator-school-devices-",
    "operator-org-devices-",
    ...operatorSummaryPrefixes,
  ],
  schoolDevices: (id: string | undefined) =>
    `operator-school-devices-${encodeURIComponent(id ?? "")}`,
  orgDevices: (id: string | undefined) =>
    `operator-org-devices-${encodeURIComponent(id ?? "")}`,
  allDevices: () => "operator-all-devices",
  organizations: () => "operator-organizations",
  schoolSummaries: () => "operator-school-summaries",
  schoolPersons: (id: string | undefined) =>
    `operator-school-persons-${encodeURIComponent(id ?? "")}`,
  schoolAccounts: (id: string | undefined) =>
    `operator-school-accounts-${encodeURIComponent(id ?? "")}`,
  orgAccounts: (id: string | undefined) =>
    `operator-org-accounts-${encodeURIComponent(id ?? "")}`,
  allAccounts: () => "operator-all-accounts",
  schools: () => "operator-schools",
  organizationSummaries: () => "operator-organization-summaries",
  organizationSchoolSummaries: (id: string | undefined) =>
    `operator-organization-school-summaries-${encodeURIComponent(id ?? "")}`,
  orgPersons: (id: string | undefined) =>
    `operator-org-persons-${encodeURIComponent(id ?? "")}`,
  orgTraegerMembers: (id: string | undefined) =>
    `operator-org-traeger-members-${encodeURIComponent(id ?? "")}`,
  orgTraegerInvitations: (id: string | undefined) =>
    `operator-org-traeger-invitations-${encodeURIComponent(id ?? "")}`,
  schoolPwaUsage: (id: string | undefined) =>
    `operator-school-pwa-usage-${encodeURIComponent(id ?? "")}`,
  orgErasureOrder: (id: string | undefined) =>
    `operator-org-erasure-order-${encodeURIComponent(id ?? "")}`,
  orgBelegRetention: (id: string | undefined) =>
    `operator-org-beleg-retention-${encodeURIComponent(id ?? "")}`,
  orgTraegerSettings: (id: string | undefined) =>
    `operator-org-traeger-settings-${encodeURIComponent(id ?? "")}`,
  provisioningStats: () => "operator-provisioning-stats",
  caterers: () => "operator-caterers",
  unregisteredTags: (
    resolved: "all" | "resolved" | "unresolved",
    organizationId: string,
    schoolId: string,
  ) =>
    `operator-unregistered-tags-${JSON.stringify([resolved, organizationId, schoolId])}`,
} as const;

export const operatorAnnouncementKeys = {
  list: () => "operator-announcements",
  views: (id: string) =>
    `operator-announcement-views-${encodeURIComponent(id)}`,
  stats: (id: string | undefined) =>
    `announcement-stats-${encodeURIComponent(id ?? "")}`,
} as const;

export const operatorInvitationKeys = {
  list: () => "operator-invitations",
} as const;

export const staffMergeKeys = {
  preview: (id: string | undefined) =>
    `operator-staff-person-merges-${encodeURIComponent(id ?? "")}`,
} as const;

export const messageKeys = {
  teamChatPreviewSources: (key: string) => key.startsWith("team-chat-inbox-"),
  teamChatPreview(data: unknown, id: string): ListPreview | undefined {
    // Only identity shown in the inbox, never the last message or unread data.
    const row = previewRow(data, id);
    const name = previewText(
      row?.kind === "group" ? row.group_name : row?.counterpart_name,
    );
    return name ? { id, name } : undefined;
  },
  groupPreview(data: unknown, id: string): ListPreview | undefined {
    const row = previewRow(data, id);
    const name = previewText(row?.title);
    return name ? { id, name } : undefined;
  },
  teamChatThread: (id: string | undefined) =>
    `team-chat-thread-${encodeURIComponent(id ?? "")}`,
  groupMessageTargets: (id: string | undefined) =>
    `group-message-targets-${encodeURIComponent(id ?? "")}`,
  groupMessageAudience: (id: string | undefined, targetId: string) =>
    resourceKey("group-message-audience-", { type: id, targetId }),
  teamChatInbox: (onlyUnread: boolean) =>
    `team-chat-inbox-${encodeURIComponent(String(onlyUnread))}`,
  /** Every filter of the team-chat inbox. */
  teamChatInboxFamily: () => "team-chat-inbox-",
  messagesInbox: (onlyUnread: boolean) =>
    `messages-inbox-${encodeURIComponent(String(onlyUnread))}`,
  /** Every filter of the parent-message inbox. */
  messagesInboxFamily: () => "messages-inbox-",
  groupMessages: () => "group-messages",
  messageThread: (id: string | undefined) =>
    `message-thread-${encodeURIComponent(id ?? "")}`,
  groupMessage: (id: string | undefined) =>
    `group-message-${encodeURIComponent(id ?? "")}`,
} as const;

/**
 * The per-child cache families the SSE hook invalidates by prefix. Every key
 * of a family starts with its prefix and continues with the child's id.
 */
export const STUDENT_KEY_FAMILIES = {
  detail: "student-detail-",
  carePlanDay: "care-plan-day-",
  carePlanWeek: "care-plan-week-",
  // Also the start of the absence overview's "student-status-days-overview-":
  // a student update refreshes that list too.
  statusDays: "student-status-days-",
  partialAbsences: "student-partial-absences-",
  statusDayOverview: "student-status-days-overview-",
  pickupData: "pickup-data-",
  arrivalData: "arrival-data-",
} as const;

export const studentKeys = {
  search: searchKey,
  schoolClasses: () => "search-student-school-classes",
  statisticsReport: (params: object) =>
    resourceKey("student-statistics-report-", params),
  detail: (id: string) =>
    `${STUDENT_KEY_FAMILIES.detail}${encodeURIComponent(id)}`,
  notes: (id: string) => `student-notes-${encodeURIComponent(id)}`,
  noteActivities: (id: string) =>
    `student-note-activities-${encodeURIComponent(id)}`,
  permanentNotes: (id: string) =>
    `student-permanent-notes-${encodeURIComponent(id)}`,
  enrollments: (id: string) => `student-enrollments-${encodeURIComponent(id)}`,
  holidayGuest: (id: string) => `holiday-guest:${encodeURIComponent(id)}`,
  carePlanDay: (id: string, date: string) =>
    `${STUDENT_KEY_FAMILIES.carePlanDay}${encodeURIComponent(id)}-${date}`,
  carePlanWeek: (id: string, from: string, to: string) =>
    `${STUDENT_KEY_FAMILIES.carePlanWeek}${encodeURIComponent(id)}-${from}-${to}`,
  statusDays: (id: string, from: string, to: string) =>
    `${STUDENT_KEY_FAMILIES.statusDays}${encodeURIComponent(id)}-${from}-${to}`,
  statusDayOverview: (params: {
    from: string;
    to: string;
    page: number;
    query: string;
    status: string;
    groupId: string;
  }) =>
    `${STUDENT_KEY_FAMILIES.statusDayOverview}${params.from}-${params.to}-${params.page}-${params.query}-${params.status}-${params.groupId}`,
  partialAbsences: (id: string, from: string, to: string) =>
    `${STUDENT_KEY_FAMILIES.partialAbsences}${encodeURIComponent(id)}-${from}-${to}`,
  previewSources: (key: string) =>
    key.startsWith(SEARCH_STUDENTS_KEY_PREFIX) ||
    key.startsWith(groupKeys.liveFamily) ||
    key.startsWith("room-students-") ||
    key.startsWith(STUDENT_KEY_FAMILIES.statusDayOverview) ||
    key.startsWith("student-statistics-report-"),
  preview(data: unknown, id: string): ListPreview | undefined {
    const row = previewRow(data, id);
    if (!row) return undefined;
    const name =
      previewText(row.name) ??
      [
        previewText(row.first_name),
        previewText(row.second_name) ?? previewText(row.last_name),
      ]
        .filter(Boolean)
        .join(" ");
    if (!name) return undefined;
    return {
      id,
      name,
      context:
        [previewText(row.school_class), previewText(row.group_name)]
          .filter(Boolean)
          .join(" · ") || undefined,
    };
  },
  studentThreads: (id: string | undefined) =>
    `student-threads-${encodeURIComponent(id ?? "")}`,
  changeHistory: (id: string) =>
    `student-change-history-${encodeURIComponent(id)}`,
  pickupData: (id: string) =>
    `${STUDENT_KEY_FAMILIES.pickupData}${encodeURIComponent(id)}`,
  name: (id: string) => `student-name-${encodeURIComponent(id)}`,
  documents: (id: string) => `student-documents-${encodeURIComponent(id)}`,
  arrivalData: (id: string) =>
    `${STUDENT_KEY_FAMILIES.arrivalData}${encodeURIComponent(id)}`,
} as const;

export const timetableKeys = {
  offeringGaps: (periodId: string) =>
    `offering-gaps-${encodeURIComponent(periodId)}`,
  // Schulferien (#5345): der Hinweis und die Liste eines Fensters.
  schoolVacationHints: (today?: string) =>
    today
      ? `timetable-school-vacation-hints-${today}`
      : "timetable-school-vacation-hints",
  schoolVacationsFamily: "school-vacations-",
  schoolVacations: (from: string, to: string) =>
    `${timetableKeys.schoolVacationsFamily}${from}-${to}`,
  bookingLeftovers: (periodId: string) =>
    `booking-leftovers-${encodeURIComponent(periodId)}`,
} as const;

// School portal (Lehrkräfte).
export const schoolPortalKeys = {
  classDayClasses: () => "class-day-my-classes",
  classDayReport: (schoolClass: string, date: string) =>
    resourceKey("school-class-day-report-", { schoolClass, date }),
  schoolDay: (date: string) => `school-day-${date}`,
  parentConferences: () => "school-parent-conferences",
  absenceList: (schoolClass: string, from = "", to = "") =>
    `school-absence-list-${schoolClass}-${from}-${to}`,
} as const;

export const weekProtocolKeys = {
  list: (groupId?: string | null) =>
    `week-protocols-${encodeURIComponent(groupId ?? "all")}`,
  detail: (id: string) => `week-protocol-${encodeURIComponent(id)}`,
} as const;

export const payrollKeys = {
  status: () => "payroll-status",
  dispatch: () => "payroll-dispatch",
  sftpStatus: () => "payroll-sftp-status",
  rules: () => "payroll-rules",
} as const;

export const databaseKeys = {
  counts: () => "/api/database/counts",
  // The SSE hook revalidates every key containing this name.
  calendarPeriods: () => "database-calendar-periods-list",
} as const;

export const homeLayoutKeys = {
  key: () => "home-layout",
} as const;

export const schoolSetupKeys = {
  key: () => "school-setup",
} as const;

export const staffOnboardingKeys = {
  key: () => "staff-onboarding",
} as const;

export const operatorBillingKeys = {
  keyDay: () => "operator-billing-key-day",
  counts: () => "operator-billing-key-date-counts",
} as const;

/** Staff announcement lists never seed the relationship-filtered parents portal. */
export const parentAnnouncementKeys = {
  list: () => "parent-announcements-list",
  detail: (id: string) => `parent-announcement-${id}`,
  pollEvaluation: (id: string) =>
    `parent-announcement-poll-evaluation-${encodeURIComponent(id)}`,
  preview(data: unknown, id: string) {
    const row = previewRow(data, id);
    const name = previewText(row?.title);
    if (!row || !name) return undefined;
    return {
      id,
      name,
      // List-visible status/type only enable an authenticated request, never UI or rights.
      poll:
        row.response_type === "single_choice" ||
        row.response_type === "multi_choice",
      status: previewText(row.status),
    };
  },
} as const;

export const displayKeys = { list: () => "info-displays" } as const;
/** Dokumente, die der Träger für die Schule freigegeben hat (#4692). */
export const schoolTraegerDocumentKeys = {
  list: () => "traeger-documents-school",
} as const;
export const timeTrackingKeys = {
  current: () => "time-tracking-current",
  // Eigene Schichten auf „Mein Dienstplan“ (#3821). Beide teilen das Präfix
  // `time-tracking-own-shifts-week-` aus PLAN_CACHE_KEY_PREFIXES und den
  // SSE-Präfixen: eine Planänderung frischt so auch die Suche nach der
  // nächsten Schicht auf (#5639).
  ownShiftsWeek: (from: string, to: string) =>
    `time-tracking-own-shifts-week-${from}-${to}`,
  ownShiftsAhead: (from: string, to: string) =>
    `time-tracking-own-shifts-week-ahead-${from}-${to}`,
} as const;

/** Träger metadata only. Registrations, people and audit logs are never cached. */
export const traegerKeys = {
  schools: () => "traeger-schools",
  school: (id: string) => `traeger-school-${encodeURIComponent(id)}`,
  schoolsAlerts: () => "traeger-schools-alerts",
  keyDateCounts: () => "traeger-key-date-counts",
  occupancy: () => "traeger-occupancy",
  periodsFamily: "traeger-holiday-period-",
  periods: () => "traeger-holiday-period-list",
  period: (id: string) =>
    `traeger-holiday-period-detail-${encodeURIComponent(id)}`,
  settings: () => "traeger-settings-schema",
  // Kriterien der Platzvergabe (#4700): Namen und Fragen, keine Antworten.
  holidayCriteria: () => "traeger-holiday-criteria",
  // Schließtage und Feiertage eines Jahres (#4702), keine Personendaten.
  closingDays: (from: string, to: string) =>
    `traeger-closing-days-${from}-${to}`,
  // Offene Hinweise an Eltern zu Schließtagen in Ferienwochen (#6434):
  // Wochen und Zahlen, keine Namen.
  holidayClosures: () => "traeger-holiday-closures-pending",
  billingRuns: () => "traeger-billing-run-list",
  // Geteilte Dokumente (#4692): Titel, Dateiname und Freigaben, keine Inhalte.
  documents: () => "traeger-document-list",
  documentAudiences: () => "traeger-document-audiences",
  periodPreview(data: unknown, id: string): ListPreview | undefined {
    const row = previewRow(data, id);
    const name = previewText(row?.name);
    return name ? { id, name } : undefined;
  },
} as const;

/** Exact list identity shared with intent; preserve the SSE group segment. */
function searchKey(params: {
  groupIds: readonly string[];
  search: string;
  schoolClasses: readonly string[];
  roomId: string;
  dayStatus: string;
  date: string;
  isToday: boolean;
  photoState: string;
  bus: string;
  photoConsent: string;
  companions: boolean;
  years: readonly string[];
  pickupStatus: string;
}): string {
  return `${SEARCH_STUDENTS_KEY_PREFIX}${searchStudentsGroupScope(params.groupIds)}-${params.search}-${encodeMultiValueParam([...params.schoolClasses])}-${params.roomId}-${params.dayStatus}-${params.date}-${params.isToday ? "today" : "planning"}-${params.photoState}-${params.bus}-${params.photoConsent}-${params.companions}-${encodeMultiValueParam([...params.years])}-${params.pickupStatus}`;
}

const catererOrdersFamily = "caterer-orders-";

/** Caterer sessions span schools; every order response still names its school. */
export const catererKeys = {
  family: catererOrdersFamily,
  containsSchool(key: string, schoolId: string): boolean {
    if (!key.startsWith(catererOrdersFamily)) return false;
    try {
      const params: unknown = JSON.parse(key.slice(catererOrdersFamily.length));
      return (
        params !== null &&
        typeof params === "object" &&
        "schoolIds" in params &&
        Array.isArray(params.schoolIds) &&
        params.schoolIds.includes(schoolId)
      );
    } catch {
      // Not a factory-built order window.
      return false;
    }
  },
  schools: () => "caterer-schools",
  orders: (schoolIds: readonly string[], from: string, to: string) =>
    resourceKey(catererOrdersFamily, {
      schoolIds: [...schoolIds].sort(),
      from,
      to,
    }),
} as const;

/**
 * Die Gesundheitsangaben aus der Ferienanmeldung bei einem eigenen Kind der
 * ausrichtenden OGS (#4722) und die Hinweise „Gast aus …“ an den
 * Feriengästen der Listen (#5630).
 */
export const holidayKeys = {
  /**
   * Die Wochen, die die Schule ausrichtet: Seite „Ferienwochen“ und die
   * Markierung in Betreuungs- und Dienstplan teilen sich den Eintrag.
   */
  hostedWeeks: () => "holiday-weeks",
  studentHealth: (studentId: string) =>
    `holiday-student-health:${encodeURIComponent(studentId)}`,
  /** Betreuung, Notfall und Abholung aus der Ferienanmeldung (#6678). */
  studentRegistration: (studentId: string) =>
    `holiday-student-registration:${encodeURIComponent(studentId)}`,
  /** Zuordnung der eigenen Kinder einer Ferienwoche (#6678). */
  ownChildren: (weekId: string) =>
    `holiday-own-children:${encodeURIComponent(weekId)}`,
  /** Feriengruppen, deren Woche noch nicht begonnen hat (#6680). */
  guestGroupHints: () => "holiday-guest-group-hints",
  /** Die Feriengruppen der Gruppenverwaltung (ADR 0120, Nachtrag 2026-11-11). */
  guestGroups: "holiday-guest-groups",
  /** Je Satz von Feriengästen ein Schlüssel: ein neuer Gast fragt neu. */
  guestBadges: (studentIds: readonly string[]) =>
    `holiday-guest-badges:${[...studentIds]
      .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))
      .join(",")}`,
};
