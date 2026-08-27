"use client";

import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { PageIntro } from "~/components/ui/page-intro";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentFormPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="w-full space-y-6">
      <PageIntro
        kicker="Anmeldungen"
        title="Anmeldeformulare"
        description="Basisformular und eigene Vorlagen für die Angaben, die Eltern bei der Anmeldung machen."
      />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <div className="hidden lg:block">
            <EnrollmentFormEditor />
          </div>
        </>
      ) : (
        <SkeletonRegion label="Anmeldeformular wird geladen">
          <FormSkeleton fields={6} />
        </SkeletonRegion>
      )}
    </div>
  );
}
