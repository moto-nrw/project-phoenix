"use client";

import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { Loading } from "~/components/ui/loading";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentFormPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <PageHeaderWithSearch title="Anmeldeformulare" />
        <EnrollmentFormEditor />
      </div>
    </div>
  );
}
