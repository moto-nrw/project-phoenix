"use client";

import useSWR from "swr";

import { getChildFeatures, listMyChildren } from "~/lib/parent-api";

// Resolve whether any school the parent has a child at broadcasts parent
// announcements. Mirrors useParentMealPlanEnabled (list children, one
// representative child per tenant, read per-child features) so the
// parents-portal Neuigkeiten nav/panel only appear when at least one linked
// school has operations.parent_news_enabled turned on — matching the backend
// feed, which excludes disabled tenants entirely.
async function fetchAnyNewsEnabled(): Promise<boolean> {
  const children = await listMyChildren();
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
export function useParentNewsEnabled(enabled: boolean): boolean {
  const { data } = useSWR(
    enabled ? "parent-news-enabled" : null,
    fetchAnyNewsEnabled,
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    },
  );
  return data === true;
}
