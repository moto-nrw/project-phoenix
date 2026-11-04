"use client";

import { useCallback, type ReactNode } from "react";
import { useSession } from "next-auth/react";
import { usePresenceMode, useTenantSlugSafe } from "~/lib/tenant-context";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { useSidebarMemory } from "~/lib/sidebar-memory";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { tenantPrefetchResources } from "./tenant-prefetch-resources";
import { ResourcePrefetchProvider } from "./resource-prefetch-provider";

/** The tenant alone supplies permissions and exact destination selection. */
export function TenantDataPrefetch({
  children,
}: Readonly<{ children: ReactNode }>) {
  const { data: session } = useSession();
  const slug = useTenantSlugSafe();
  const detailedMode = usePresenceMode() === "detailed";
  const photos = useStudentPhotosEnabled();
  const photoState = photos.isLoading
    ? "photos-loading"
    : photos.enabled
      ? "photos-enabled"
      : "photos-disabled";
  const memory = useSidebarMemory();
  const supervision = useOptionalSupervision();
  const resourcesFor = useCallback(
    async (href: string) => {
      if (!session || !slug) return [];
      return tenantPrefetchResources(href, {
        session,
        photoState,
        detailedMode,
        slug,
        groupId: memory.read("sidebar-last-group"),
        groups: supervision?.groups.map((group) => group.id) ?? [],
        hasSupervision: supervision?.isSupervising ?? false,
      });
    },
    [session, slug, photoState, detailedMode, memory, supervision],
  );
  return (
    <ResourcePrefetchProvider resourcesFor={resourcesFor}>
      {children}
    </ResourcePrefetchProvider>
  );
}
