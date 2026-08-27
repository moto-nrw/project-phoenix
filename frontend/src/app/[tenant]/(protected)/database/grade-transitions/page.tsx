"use client";

import { useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import {
  GradeTransitionsManager,
  type TransitionPermissions,
} from "~/components/database/grade-transitions/grade-transitions-manager";
import { BackButton } from "~/components/ui/back-button";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { PageIntro } from "~/components/ui/page-intro";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { Skeleton } from "~/components/ui/skeleton";
import { hasPermission } from "~/lib/auth-utils";

export default function GradeTransitionsPage() {
  const { data: session, status } = useSession();

  const permissions = useMemo<TransitionPermissions>(
    () => ({
      canCreate: hasPermission(session, "grade_transitions:create"),
      canUpdate: hasPermission(session, "grade_transitions:update"),
      canDelete: hasPermission(session, "grade_transitions:delete"),
      canApply: hasPermission(session, "grade_transitions:apply"),
      canPurge: hasPermission(session, "users:delete"),
    }),
    [session],
  );

  const canRead = hasPermission(session, "grade_transitions:read");

  // Statuszeile des Seitenkopfs: die Liste, die der Manager ohnehin lädt.
  const [summary, setSummary] = useState<{
    total: number;
    applied: number;
    latestYear: string | null;
  } | null>(null);
  const handleSummaryChange = useCallback(
    (
      next: {
        total: number;
        applied: number;
        latestYear: string | null;
      } | null,
    ) => setSummary(next),
    [],
  );
  const statusLine = (() => {
    if (!canRead) return "Kein Zugriff auf Jahrgangswechsel";
    if (!summary) return null;
    if (summary.total === 0) return "Noch kein Jahrgangswechsel angelegt";
    const parts: string[] = [];
    if (summary.latestYear) parts.push(`Schuljahr ${summary.latestYear}`);
    parts.push(`${summary.total} Wechsel`);
    parts.push(`${summary.applied} angewendet`);
    return parts.join(" · ");
  })();

  return (
    <div className="w-full">
      <BackButton referrer="/database" />

      <PageIntro
        kicker="Datenverwaltung"
        title="Jahrgangswechsel"
        description={statusLine ?? <Skeleton className="h-4 w-56" />}
        className="mb-6"
      />

      <DesktopOnlyNotice description="Der Jahrgangswechsel ist für die Arbeit am Computer optimiert. Bitte öffnen Sie diese Seite auf einem Laptop oder Desktop-Rechner." />

      <div className="hidden lg:block">
        {status === "loading" ? (
          <SkeletonRegion label="Jahrgangswechsel wird geladen">
            <ListSkeleton rows={6} />
          </SkeletonRegion>
        ) : canRead ? (
          <GradeTransitionsManager
            permissions={permissions}
            onSummaryChange={handleSummaryChange}
          />
        ) : (
          <ForbiddenPage message="Sie verfügen nicht über die notwendigen Berechtigungen, um Jahrgangswechsel anzusehen." />
        )}
      </div>
    </div>
  );
}
