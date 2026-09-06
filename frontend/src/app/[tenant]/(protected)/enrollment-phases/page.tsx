"use client";

import { Suspense } from "react";
import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { TenantPage } from "~/components/ui/tenant-page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

/** Kopfkarte samt Aktion trägt der Editor, weil „Neue Anmeldephase“ an seinen
 *  Zustand gebunden ist. Solange die Berechtigung geprüft wird, steht hier das
 *  Gerüst mit demselben Titel. */
function PhasesPageSkeleton() {
  return <TenantPage title="Anmeldephasen" statsLoading loading />;
}

export default function EnrollmentPhasesPage() {
  const { isReady } = useRequireAdmin();

  if (!isReady) return <PhasesPageSkeleton />;

  return (
    <Suspense fallback={<PhasesPageSkeleton />}>
      <PhasesEditor />
    </Suspense>
  );
}
