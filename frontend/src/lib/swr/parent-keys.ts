import type { Child } from "~/lib/parent-api";

function relationships(children: readonly Child[]): string {
  return children
    .map(
      (child) =>
        `${encodeURIComponent(child.tenant_id)}:${encodeURIComponent(child.student_id)}`,
    )
    .sort()
    .join(",");
}

/**
 * Keys of the parents portal shell (#5007). Kept apart from portal-keys.ts,
 * which bundles as a whole, so the parents' and the public enrollment
 * routes do not carry every portal's factories.
 */
export const parentKeys = {
  mealParticipationFamily: (schoolId: string, studentId: string) =>
    `parent-meal-participation-${encodeURIComponent(schoolId)}-${encodeURIComponent(studentId)}-`,
  mealParticipation: (
    schoolId: string,
    studentId: string,
    from: string,
    to: string,
    asOf: string,
  ) =>
    `parent-meal-participation-${encodeURIComponent(schoolId)}-${encodeURIComponent(studentId)}-${from}-${to}-${asOf}`,

  shellChildren: () => "parent-shell-children",
  isChildResource: (key: string) =>
    key === "parent-shell-children" ||
    key.startsWith("parent-features-") ||
    key.startsWith("parent-today-"),
  isMessagesResource: (key: string) =>
    key === "parent-messages" || key === "parent-group-threads",
  isNewsResource: (key: string) => key === "parent-announcements",
  isResource: (key: string) =>
    key.startsWith("parent-") && !key.startsWith("parent-profile"),
  relationshipSet: (children: readonly Child[]) => relationships(children),
  features: (children: readonly Child[], day: string) =>
    `parent-features-${relationships(children)}-${day}`,
  today: (children: readonly Child[], day: string) =>
    `parent-today-${relationships(children)}-${day}`,
  calendar: (day: string) => `parent-calendar-${day}-90`,
  messages: () => "parent-messages",
  groupThreads: () => "parent-group-threads",
  announcements: () => "parent-announcements",
  profile: () => "parent-profile",
  /** The conversation of one child with one audience (#5011, #5200). */
  conversation: (studentId: string, audience: string) =>
    `parent-conversation-${encodeURIComponent(studentId)}-${encodeURIComponent(audience)}`,
} as const;
