"use client";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { Skeleton } from "~/components/ui/skeleton";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import type { MotoDuotoneTone } from "~/lib/location-helper";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";

// Mirrors a StatCard tile: real icon + real title render immediately, only
// the data-bound value skeletonizes.
function StatTileSkeleton({
  title,
  icon,
  tone,
}: Readonly<{
  title: string;
  icon: PhosphorIcon;
  tone: MotoDuotoneTone;
}>) {
  return (
    <div className="moto-content-surface relative overflow-hidden rounded-2xl border p-4 shadow-sm backdrop-blur-md md:p-6">
      <div className="mb-3 flex items-start justify-between">
        <div className="p-0.5">
          <MotoDuotoneIcon icon={icon} tone={tone} />
        </div>
      </div>
      <div className="space-y-1">
        <p className="text-xs font-medium text-gray-600 md:text-sm">{title}</p>
        <Skeleton className="h-7 w-16 rounded-full" />
      </div>
    </div>
  );
}

// Mirrors an InfoCard tile: real icon + real title render immediately, only
// the data-bound list body skeletonizes.
function InfoTileSkeleton({
  title,
  concept,
  rows = 3,
}: Readonly<{ title: string; concept: MotoConceptKey; rows?: number }>) {
  return (
    <div className="moto-content-surface relative h-full overflow-hidden rounded-2xl border p-4 shadow-sm backdrop-blur-md md:p-6">
      <div className="mb-4 flex items-center gap-2">
        <div className="rounded-xl bg-gray-100 p-2">
          <MotoConceptIcon concept={concept} size={20} />
        </div>
        <h3 className="text-base font-semibold text-gray-900 md:text-lg">
          {title}
        </h3>
      </div>
      <div className="space-y-2">
        {Array.from({ length: rows }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-xl" />
        ))}
      </div>
    </div>
  );
}

/**
 * Real-chrome loading state for the dashboard (RoleGuard fallback + route
 * loading.tsx): static tile titles/icons render immediately — the same
 * visibility rules as DashboardContent, driven by tenant presence-mode
 * settings that are already resolved via TenantProvider before the
 * session/role check completes — and only the data-bound values / list rows
 * skeletonize. Keep the tile visibility conditions in sync with page.tsx.
 */
export function DashboardSkeleton() {
  const nfcEnabled = useNFCEnabled();
  const openCareGroupMode = useOpenCareGroupMode();
  const presenceMode = usePresenceMode();
  const showActivitySurfaces = nfcEnabled && presenceMode !== "binary";
  const showRoomSurfaces = presenceMode !== "binary";

  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Übersicht wird geladen"
      data-testid="dashboard-skeleton"
      className="-mt-1.5 w-full"
    >
      {/* Der Kopf rendert sofort, nur die Datenregion skeletonisiert. */}
      <PageHeaderWithSearch title="Home" />

      <div className="mb-6 md:mb-8">
        <Skeleton className="h-5 w-72 rounded-full" />
      </div>

      <div className="mb-6 grid grid-cols-2 gap-3 md:mb-8 md:grid-cols-3 md:gap-4 xl:grid-cols-4">
        <StatTileSkeleton
          title="Kinder anwesend"
          icon={MOTO_CONCEPTS.present.icon}
          tone={MOTO_CONCEPTS.present.tone}
        />
        {showRoomSurfaces ? (
          <>
            <StatTileSkeleton
              title="In Räumen"
              icon={MOTO_CONCEPTS.rooms.icon}
              tone={MOTO_CONCEPTS.rooms.tone}
            />
            <StatTileSkeleton
              title="Unterwegs"
              icon={MOTO_CONCEPTS.transit.icon}
              tone={MOTO_CONCEPTS.transit.tone}
            />
          </>
        ) : null}
        <StatTileSkeleton
          title="Schulhof"
          icon={MOTO_CONCEPTS.schoolyard.icon}
          tone={MOTO_CONCEPTS.schoolyard.tone}
        />
        <StatTileSkeleton
          title="Krank"
          icon={MOTO_CONCEPTS.sick.icon}
          tone={MOTO_CONCEPTS.sick.tone}
        />
        <StatTileSkeleton
          title="Entschuldigt"
          icon={MOTO_CONCEPTS.excused.icon}
          tone={MOTO_CONCEPTS.excused.tone}
        />
        <StatTileSkeleton
          title="Zuhause"
          icon={MOTO_CONCEPTS.home.icon}
          tone={MOTO_CONCEPTS.home.tone}
        />
        {showActivitySurfaces ? (
          <StatTileSkeleton
            title="Aktive Aktivitäten"
            icon={MOTO_CONCEPTS.activities.icon}
            tone={MOTO_CONCEPTS.activities.tone}
          />
        ) : null}
        {showRoomSurfaces ? (
          <StatTileSkeleton
            title="Auslastung"
            icon={MOTO_CONCEPTS.utilization.icon}
            tone={MOTO_CONCEPTS.utilization.tone}
          />
        ) : null}
      </div>

      <div className="grid grid-cols-1 items-stretch gap-4 md:gap-6 lg:grid-cols-2 xl:grid-cols-3">
        {showRoomSurfaces ? (
          <InfoTileSkeleton title="Letzte Bewegungen" concept="changeHistory" />
        ) : null}
        {showActivitySurfaces ? (
          <InfoTileSkeleton title="Laufende Aktivitäten" concept="activities" />
        ) : null}
        {!openCareGroupMode ? (
          <InfoTileSkeleton title="Aktive Gruppen" concept="groups" />
        ) : null}
        <InfoTileSkeleton title="Personal heute" concept="staff" rows={2} />
      </div>
    </div>
  );
}
