"use client";

import useSWR from "swr";

import {
  getChildFeatures,
  listAnnouncements,
  listMyChildren,
  type Child,
} from "~/lib/parent-api";

// Resolve whether the parent has any discoverable parent-news. Mirrors
// useParentMealPlanEnabled for the linked-child case (list children, one
// representative child per tenant, read per-child features) so the
// parents-portal Neuigkeiten nav/panel appear when at least one linked school
// has operations.parent_news_enabled turned on — matching the backend feed,
// which excludes disabled tenants entirely.
//
// A guardian can also be targeted via the pending_enrollment audience with NO
// linked child yet: the backend feed (newsEnabledTenants) unions enrollment
// requests, so a probe built from children alone would hide the entry from an
// applicant who already has a targeted announcement. There is no per-tenant
// feature endpoint to probe those enrollment-only tenants, so we fall back to
// the feed itself — any item means the parent has news to open, even with no
// child to read features on.
async function fetchAnyNewsEnabled(
  knownChildren?: readonly Child[],
): Promise<boolean> {
  const [children, announcements] = await Promise.all([
    knownChildren ?? listMyChildren(),
    listAnnouncements(),
  ]);
  // Any live feed item (linked-child OR pending-enrollment) is discoverable
  // news — show the entry rather than dead-ending the applicant on a hidden nav.
  if (announcements.length > 0) {
    return true;
  }
  // One representative student per tenant — features are school-scoped.
  const repByTenant = new Map<string, string>();
  for (const child of children) {
    if (!repByTenant.has(child.tenant_id)) {
      repByTenant.set(child.tenant_id, child.student_id);
    }
  }
  // Let a failed feature lookup reject the whole fetch rather than swallowing
  // it: SWR then surfaces the error and retries with backoff. Catching here
  // would cache a transient 500/network/session failure as `false`, hiding the
  // nav entry for an eligible parent until a full reload.
  const features = await Promise.all(
    [...repByTenant.values()].map((studentId) => getChildFeatures(studentId)),
  );
  return features.some((f) => f.parent_news_enabled === true);
}

/**
 * True once at least one of the parent's linked schools broadcasts parent
 * announcements. Only fetches when `enabled` (parent mode) so the staff/operator
 * portals never hit the parent endpoints. Returns false while loading and when
 * no linked school runs news, so the nav/panel stays hidden until the feature
 * is confirmed available rather than leading to an empty page.
 */
export function useParentNewsEnabled(
  enabled: boolean,
  children?: readonly Child[] | null,
): boolean {
  const { data } = useSWR(
    enabled && children !== null
      ? children === undefined
        ? "parent-news-enabled"
        : [
            "parent-news-enabled",
            children.map((child) => child.student_id).join(","),
          ]
      : null,
    () => fetchAnyNewsEnabled(children ?? undefined),
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    },
  );
  return data === true;
}
