"use client";

import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentFormPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Anmeldeformulare" />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <div className="hidden lg:block">
            <EnrollmentFormEditor />
          </div>
        </>
      ) : (
        <SkeletonRegion label="Anmeldeformular wird geladen" className="mt-4">
          <FormSkeleton fields={6} />
        </SkeletonRegion>
      )}
    </div>
  );
}
