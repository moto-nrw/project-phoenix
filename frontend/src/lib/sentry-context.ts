/**
 * Who a Sentry event affects, within the data boundary of #3590: the internal
 * account ID as `user.id` and nothing else about the user, a coarse `role`
 * tag, and `school_id` only while the session is bound to exactly one school.
 * Used by the browser (per portal) and by the BFF route wrappers (per request).
 */

import * as Sentry from "@sentry/nextjs";

export type SentryPortal = "tenant" | "parent" | "school" | "operator";

/** Values of the `role` tag: the portal's role model, never a school's role name. */
type SentryRole =
  "staff" | "admin" | "carrier" | "guardian" | "lehrkraft" | "operator";

/** The session fields the context reads; every portal's Session.user fits. */
export interface SentrySessionUser {
  readonly id?: string;
  readonly scope?: string;
  readonly tenantId?: number;
  readonly isAdmin?: boolean;
  readonly isPreview?: boolean;
}

export interface SentrySessionContext {
  readonly user: { readonly id: string } | null;
  readonly role: SentryRole | null;
  readonly schoolId: string | null;
}

const SIGNED_OUT: SentrySessionContext = {
  user: null,
  role: null,
  schoolId: null,
};

/**
 * The OGS portal knows staff, leadership and carrier sessions. Role names are
 * school data, so only the scope and the admin flag count. The read-only
 * staff preview is an admin at work, as in the analytics (#2893).
 */
function tenantRole(user: SentrySessionUser): SentryRole {
  if (user.scope === "org") return "carrier";
  return user.isAdmin === true || user.isPreview === true ? "admin" : "staff";
}

function portalRole(portal: SentryPortal, user: SentrySessionUser): SentryRole {
  switch (portal) {
    case "tenant":
      return tenantRole(user);
    case "parent":
      return "guardian";
    case "school":
      return "lehrkraft";
    case "operator":
      return "operator";
  }
}

/**
 * Only OGS and school sessions are bound to one school. Parent tokens carry
 * no school (a guardian can have children in several), operator tokens are
 * platform-wide: there the tag is absent, never a placeholder.
 */
function boundSchoolId(
  portal: SentryPortal,
  user: SentrySessionUser,
): string | null {
  if (portal !== "tenant" && portal !== "school") return null;
  if (user.scope === "parent" || user.scope === "platform") return null;
  const tenantId = user.tenantId;
  return typeof tenantId === "number" && tenantId > 0 ? String(tenantId) : null;
}

export function sentrySessionContext(
  portal: SentryPortal,
  user: SentrySessionUser | null | undefined,
): SentrySessionContext {
  if (!user?.id) return SIGNED_OUT;
  return {
    user: { id: user.id },
    role: portalRole(portal, user),
    schoolId: boundSchoolId(portal, user),
  };
}

/**
 * Writes the context to the current isolation scope: in the browser that is
 * the page, in the BFF the request. Every field is written each time, so a
 * missing value clears what an earlier session left behind.
 */
export function applySentrySessionContext(context: SentrySessionContext): void {
  Sentry.setUser(context.user);
  Sentry.setTags({
    role: context.role ?? undefined,
    school_id: context.schoolId ?? undefined,
  });
}
