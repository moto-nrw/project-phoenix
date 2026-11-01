"use client";

import { useCallback, type ReactNode } from "react";
import { useSession } from "next-auth/react";
import { usePresenceMode, useTenantSlugSafe } from "~/lib/tenant-context";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { useSidebarMemory } from "~/lib/sidebar-memory";
import { useOptionalSupervision } from "~/lib/supervision-context";
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
  const mode = useShellAuthSafe()?.mode;
  const memory = useSidebarMemory();
  const supervision = useOptionalSupervision();
  const resourcesFor = useCallback(
    async (href: string) => {
      if (!session || !slug) return [];
      const { tenantPrefetchResources } =
        await import("./tenant-prefetch-resources");
      return tenantPrefetchResources(href, {
        session,
        photoState,
        detailedMode,
        teacherMode: mode === "teacher",
        slug,
        groupId: memory.read("sidebar-last-group"),
        groups: supervision?.groups.map((group) => group.id) ?? [],
        hasSupervision: supervision?.isSupervising ?? false,
      });
    },
    [session, slug, mode, photoState, detailedMode, memory, supervision],
  );
  return (
    <ResourcePrefetchProvider resourcesFor={resourcesFor}>
      {children}
    </ResourcePrefetchProvider>
  );
}
