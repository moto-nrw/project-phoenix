"use client";

import { useCallback, useState } from "react";
import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { TenantPage } from "~/components/ui/tenant-page";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
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
    <TenantPage
      title="Anmeldeformulare"
      stats={statusLine}
      statsLoading={statusLine === null}
      loading={!isReady}
    >
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <EnrollmentFormEditor
          onTemplateCountChange={handleTemplateCountChange}
        />
      </div>
    </TenantPage>
  );
}
