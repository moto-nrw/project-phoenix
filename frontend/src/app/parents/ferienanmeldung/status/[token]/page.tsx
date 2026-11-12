"use client";

// Stand einer Ferienanmeldung (#6676): der geheime Link aus der Eingangsmail.
// Eltern brauchen kein Konto.

import { use } from "react";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";
import { HolidayRegistrationStatusView } from "~/components/holiday-registration/holiday-registration-status-view";
import { useAnalyticsSchoolPolicy } from "~/lib/hooks/use-analytics-school-policy";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function HolidayRegistrationStatusPage({ params }: PageProps) {
  // A token status page establishes no school policy: it collects nothing.
  useAnalyticsSchoolPolicy("", false);
  const { token } = use(params);
  return (
    <PublicEnrollmentPageShell>
      <HolidayRegistrationStatusView token={token} />
    </PublicEnrollmentPageShell>
  );
}
