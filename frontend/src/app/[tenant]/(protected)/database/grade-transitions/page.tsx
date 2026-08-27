"use client";

import { useMemo } from "react";
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

  return (
    <div className="w-full">
      <BackButton referrer="/database" />

      <PageIntro
        kicker="Datenverwaltung"
        title="Jahrgangswechsel"
        description="Kinder zum Schuljahreswechsel in die nächste Klasse versetzen und Abgänge festhalten."
        className="mb-6"
      />

      <DesktopOnlyNotice description="Der Jahrgangswechsel ist für die Arbeit am Computer optimiert. Bitte öffnen Sie diese Seite auf einem Laptop oder Desktop-Rechner." />

      <div className="hidden lg:block">
        {status === "loading" ? (
          <SkeletonRegion label="Jahrgangswechsel wird geladen">
            <ListSkeleton rows={6} />
          </SkeletonRegion>
        ) : canRead ? (
          <GradeTransitionsManager permissions={permissions} />
        ) : (
          <ForbiddenPage message="Sie verfügen nicht über die notwendigen Berechtigungen, um Jahrgangswechsel anzusehen." />
        )}
      </div>
    </div>
  );
}
