"use client";

import { useCallback, useState } from "react";
import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { TenantPage } from "~/components/ui/tenant-page";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

export default function EnrollmentFormPage() {
  const { isReady } = useRequirePermission("config:manage");
  // Statuszeile des Seitenkopfs: die Vorlagen, die der Editor ohnehin lädt.
  const [templateCount, setTemplateCount] = useState<number | null>(null);
  // Nach einem Ladefehler gibt es keine Zahl; die Statuszeile bleibt dann
  // leer statt dauerhaft als Skelett zu laden.
  const [loadFailed, setLoadFailed] = useState(false);
  const handleTemplateCountChange = useCallback(
    (count: number | null, failed: boolean) => {
      setTemplateCount(count);
      setLoadFailed(failed);
    },
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
      statsLoading={statusLine === null && !loadFailed}
      loading={!isReady}
    >
      <DesktopOnlyNotice />
      {/* Flex-Spalte, damit die letzte Fläche bis zur Unterkante wächst
          (`.moto-tenant-body`). */}
      <div className="hidden lg:flex lg:flex-col">
        <EnrollmentFormEditor
          onTemplateCountChange={handleTemplateCountChange}
        />
      </div>
    </TenantPage>
  );
}
