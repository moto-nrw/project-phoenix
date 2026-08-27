"use client";

import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageIntro } from "~/components/ui/page-intro";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";

export default function AdminEnrollmentsPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="w-full space-y-6">
      <PageIntro
        kicker="Anmeldungen"
        title="Überblick"
        description="Einrichtung, laufende Anmeldephasen und eingegangene Anmeldungen auf einen Blick."
      />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <PhaseExpiryWarnings />
          <div className="hidden lg:block">
            <AdminEnrollmentsList />
          </div>
        </>
      ) : (
        <SkeletonRegion label="Anmeldungen werden geladen">
          <ListSkeleton rows={6} avatar={false} />
        </SkeletonRegion>
      )}
    </div>
  );
}
