"use client";

import { use } from "react";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function ParentEnrollmentStatusPage({ params }: PageProps) {
  const { token } = use(params);
  return (
    <PublicEnrollmentPageShell>
      <EnrollmentStatusView token={token} />
    </PublicEnrollmentPageShell>
  );
}
