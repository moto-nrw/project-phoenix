"use client";

import { useParams } from "next/navigation";
import { TenantPage } from "~/components/ui/tenant-page";

/**
 * Route-level loading UI: the report page's own frame while it loads, so the
 * child profile's skeleton does not flash in between.
 */
export default function AccidentReportLoading() {
  const params = useParams();
  return (
    <TenantPage
      title="Unfallanzeige"
      back
      backHref={`/students/${String(params.id)}?tab=dokumente`}
      backLabel="Zurück zum Kinderprofil"
      loading
    />
  );
}
