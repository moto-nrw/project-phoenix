"use client";

import { useCallback, useState } from "react";
import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { Skeleton } from "~/components/ui/skeleton";
import { PageIntro } from "~/components/ui/page-intro";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function EnrollmentFormPage() {
  const { isReady } = useRequireAdmin();
  // Statuszeile des Seitenkopfs: die Vorlagen, die der Editor ohnehin lädt.
  const [templateCount, setTemplateCount] = useState<number | null>(null);
  const handleTemplateCountChange = useCallback(
    (count: number | null) => setTemplateCount(count),
    [],
  );
  const statusLine =
    templateCount === null
      ? null
      : `1 Basisformular · ${templateCount} ${templateCount === 1 ? "Vorlage" : "Vorlagen"}`;

  return (
    <div className="w-full space-y-6">
      <PageIntro
        kicker="Anmeldungen"
        title="Anmeldeformulare"
        description={statusLine ?? <Skeleton className="h-4 w-52" />}
      />
      {isReady ? (
        <>
          <DesktopOnlyNotice />
          <div className="hidden lg:block">
            <EnrollmentFormEditor
              onTemplateCountChange={handleTemplateCountChange}
            />
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
