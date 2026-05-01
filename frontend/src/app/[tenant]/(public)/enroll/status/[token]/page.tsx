"use client";

import { use } from "react";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";

interface PageProps {
  readonly params: Promise<{ tenant: string; token: string }>;
}

export default function EnrollmentStatusPage({ params }: PageProps) {
  const { token } = use(params);
  return (
    <main className="mx-auto max-w-3xl p-4 sm:p-6">
      <EnrollmentStatusView token={token} />
    </main>
  );
}
