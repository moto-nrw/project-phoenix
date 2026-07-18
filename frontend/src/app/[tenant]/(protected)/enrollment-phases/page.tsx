"use client";

import { Suspense } from "react";
import { PhasesEditor } from "~/components/enrollment/phases-editor";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentPhasesPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <Suspense fallback={<Loading fullPage={false} />}>
          <PhasesEditor />
        </Suspense>
      </div>
    </div>
  );
}
