"use client";

import { use } from "react";
import Link from "next/link";
import { RolloverReviewQueue } from "~/components/enrollment/rollover-review-queue";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
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
  if (!isReady)
    return (
      <SkeletonRegion label="Prüfliste wird geladen">
        <ListSkeleton rows={5} avatar={false} />
      </SkeletonRegion>
    );

  return (
    <div className="-mt-1.5 w-full">
      <MobileBackButton />
      <PageHeaderWithSearch
        title="Prüfliste"
        actionButton={
          <Link
            href="/enrollment-phases"
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Zurück zu den Anmeldephasen
          </Link>
        }
      />
      <nav className="sr-only">
        <Link
          href="/enrollment-phases"
          className="text-moto-blue text-sm font-medium hover:underline"
        >
          Zurück zu den Anmeldephasen
        </Link>
      </nav>
      <RolloverReviewQueue phaseID={id} />
    </div>
  );
}
