"use client";

import { use } from "react";
import { EnrollmentStatusView } from "~/components/enrollment/enrollment-status-view";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

/**
 * Parents-portal mirror of the tenant /enroll/status/[token] page. The
 * post-submit redirect from the parent enrollment form lands here
 * (proxy rewrites parents.{TENANT_DOMAIN}/enroll/status/{token} →
 * /parents/enroll/status/{token}), and the status link in submission
 * confirmation emails resolves to the same URL because PARENTS_URL is
 * the host backend uses for parent-facing email links.
 *
 * The status token alone is the auth signal — the page is reachable
 * without a parent session, mirroring the tenant version.
 */
export default function ParentEnrollmentStatusPage({ params }: PageProps) {
  const { token } = use(params);
  return (
    <main className="mx-auto max-w-3xl p-4 sm:p-6">
      <EnrollmentStatusView token={token} />
    </main>
  );
}
