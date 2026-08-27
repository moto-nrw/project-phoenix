"use client";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { Skeleton } from "~/components/ui/skeleton";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
    <StatCard
      label={title}
      value=""
      loading
      icon={<MotoDuotoneIcon icon={icon} tone={tone} />}
    />
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
    <SectionCard
      title={title}
      className="h-full"
      leading={
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 shadow-sm">
          <MotoConceptIcon concept={concept} size={20} />
        </span>
      }
    >
      <div className="space-y-2">
        {Array.from({ length: rows }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-xl" />
        ))}
      </div>
    </SectionCard>
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
      className="w-full space-y-6"
    >
      {/* Kopfkarte wie im geladenen Zustand: Gruß als Titel, Statuszeile. */}
      <header className="moto-content-surface rounded-2xl border p-5 shadow-sm">
        <Skeleton className="h-7 w-64 max-w-3/4 rounded-full" />
        <Skeleton className="mt-2 h-4 w-72 max-w-full rounded-full" />
      </header>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 xl:grid-cols-4">
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

      <div className="grid grid-cols-1 items-stretch gap-6 lg:grid-cols-2 xl:grid-cols-3">
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
