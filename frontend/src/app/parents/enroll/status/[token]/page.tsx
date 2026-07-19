"use client";

import { Suspense, use } from "react";
import { useSearchParams } from "next/navigation";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";

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
  return (
    <PublicEnrollmentPageShell>
      <EnrollmentStatusView
        token={token}
        justSubmitted={searchParams.get("submitted") === "1"}
        duplicateWarning={searchParams.get("duplicate_warning") === "1"}
      />
    </PublicEnrollmentPageShell>
  );
}
