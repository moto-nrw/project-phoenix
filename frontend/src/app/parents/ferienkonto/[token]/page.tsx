"use client";

// Elternkonto aus dem Link der Ferienanmeldung (#6673, ADR 0216). Eltern
// brauchen keine Sitzung: Der Link ist die Berechtigung.

import { use } from "react";
import { HolidayParentSetupView } from "~/components/auth/holiday-parent-setup";
import { useAnalyticsSchoolPolicy } from "~/lib/hooks/use-analytics-school-policy";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function HolidayParentSetupPage({ params }: PageProps) {
  // The link names a Träger, no school: no school policy permits collection.
  useAnalyticsSchoolPolicy("", false);
  const { token } = use(params);
  return <HolidayParentSetupView token={token} />;
}
