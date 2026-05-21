"use client";

import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { Loading } from "~/components/ui/loading";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentPhasesPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <PhasesEditor />
    </div>
  );
}
