/**
 * Keys of the parents portal shell (#5007). Kept apart from portal-keys.ts,
 * which bundles as a whole, so the parents' and the public enrollment
 * routes do not carry every portal's factories.
 */
export const parentKeys = {
  newsEnabled: (id?: string) =>
    id === undefined
      ? "parent-news-enabled"
      : `parent-news-enabled-${encodeURIComponent(id)}`,
  mealPlanEnabled: (id?: string) =>
    id === undefined
      ? "parent-meal-plan-enabled"
      : `parent-meal-plan-enabled-${encodeURIComponent(id)}`,
  shellChildren: () => "parent-shell-children",
  profile: () => "parent-profile",
  /** The conversation of one child with one audience (#5011, #5200). */
  conversation: (studentId: string, audience: string) =>
    `parent-conversation-${encodeURIComponent(studentId)}-${encodeURIComponent(audience)}`,
} as const;
