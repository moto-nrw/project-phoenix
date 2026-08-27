"use client";

import { use } from "react";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";
import { Skeleton } from "~/components/ui/skeleton";
import { BackButton } from "~/components/ui/back-button";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { PageIntro } from "~/components/ui/page-intro";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

interface PageProps {
  readonly params: Promise<{ tenant: string; id: string }>;
}

/**
 * Admin review queue for rolled-over enrollments that could not be
 * carried forward automatically (grade above cap, missing grade, etc).
 */
export default function RolloverReviewPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();

  return (
    <div className="w-full space-y-6">
      <BackButton referrer="/enrollment-phases" />
      {/* Titel und Erklärtext trägt die Kopfkarte der Prüfliste (PageIntro). */}
      {isReady ? (
        <RolloverReviewQueue phaseID={id} />
      ) : (
        <>
          <PageIntro
            kicker="Anmeldungen"
            title="Prüfliste"
            description={<Skeleton className="h-4 w-44" />}
          />
          <SkeletonRegion label="Prüfliste wird geladen">
            <ListSkeleton rows={5} avatar={false} />
          </SkeletonRegion>
        </>
      )}
    </div>
  );
}
