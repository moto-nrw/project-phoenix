import { previewRow, previewText, type ListPreview } from "./list-preview";
import { resourceKey } from "./resource-key";

export { announcementKeys, dashboardKeys } from "./shell-keys";
export { parentKeys } from "./parent-keys";

/** Unscoped family factories. Only the shared hooks turn these into cache keys. */
export const operatorProvisioningKeys = {
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
  schoolPwaUsage: (id: string | undefined) =>
    `operator-school-pwa-usage-${encodeURIComponent(id ?? "")}`,
  orgErasureOrder: (id: string | undefined) =>
    `operator-org-erasure-order-${encodeURIComponent(id ?? "")}`,
  provisioningStats: () => "operator-provisioning-stats",
  unregisteredTags: (
    resolved: "all" | "resolved" | "unresolved",
    organizationId: string,
    schoolId: string,
  ) =>
    `operator-unregistered-tags-${JSON.stringify([resolved, organizationId, schoolId])}`,
} as const;

export const operatorAnnouncementKeys = {
  list: () => "operator-announcements",
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

export const studentKeys = {
  statisticsReport: (params: object) =>
    resourceKey("student-statistics-report-", params),
  detail: (id: string) => `student-detail-${encodeURIComponent(id)}`,
  previewSources: (key: string) =>
    key.startsWith("search-students-") ||
    key.startsWith("ogs-students-") ||
    key.startsWith("room-students-") ||
    key.startsWith("student-status-days-overview-") ||
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
  privacyConsent: (id: string) =>
    `student-privacy-consent-${encodeURIComponent(id)}`,
  // The SSE hook invalidates the "pickup-data-" / "arrival-data-" prefixes.
  pickupData: (id: string) => `pickup-data-${encodeURIComponent(id)}`,
  name: (id: string) => `student-name-${encodeURIComponent(id)}`,
  documents: (id: string) => `student-documents-${encodeURIComponent(id)}`,
  arrivalData: (id: string) => `arrival-data-${encodeURIComponent(id)}`,
} as const;

export const timetableKeys = {
  offeringGaps: (periodId: string) =>
    `offering-gaps-${encodeURIComponent(periodId)}`,
} as const;

// School portal (Lehrkräfte).
export const schoolPortalKeys = {
  classDayClasses: () => "class-day-my-classes",
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
} as const;

export const databaseKeys = {
  counts: () => "/api/database/counts",
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
