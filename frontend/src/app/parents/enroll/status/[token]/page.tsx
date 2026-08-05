"use client";

import { Suspense, use } from "react";
import { useSearchParams } from "next/navigation";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";
import { useOneTimeQueryFlag } from "~/lib/hooks/use-one-time-query-flag";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function ParentEnrollmentStatusPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <ParentEnrollmentStatusPageContent params={params} />
    </Suspense>
  );
}

function ParentEnrollmentStatusPageContent({ params }: PageProps) {
  const { token } = use(params);
  const searchParams = useSearchParams();
  // One-time: the flag hides the withdraw controls, so it must not survive a
  // refresh or a bookmarked/shared URL (#2156 review).
  const justSubmitted = useOneTimeQueryFlag("submitted");
  return (
    <PublicEnrollmentPageShell>
      <EnrollmentStatusView
        token={token}
        justSubmitted={justSubmitted}
        duplicateWarning={searchParams.get("duplicate_warning") === "1"}
      />
    </PublicEnrollmentPageShell>
  );
}
