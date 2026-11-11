"use client";

// The download of one holiday registration as a PDF (#6691). The entry with
// the health information exists only for a person who may see it on the
// screen; the backend checks the right again and records every download.

import { useState } from "react";
import { ExportMenu, type ExportMenuOption } from "~/components/ui/export-menu";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { downloadHolidayRegistrationPdf } from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayRegistrationPdfMenu" });

const HEALTH_KEY = "pdf-health";

function options(canSeeHealth: boolean): readonly ExportMenuOption[] {
  return [
    { key: "pdf", format: "pdf" },
    ...(canSeeHealth
      ? [
          {
            key: HEALTH_KEY,
            format: "pdf" as const,
            label: "PDF mit Gesundheitsangaben",
          },
        ]
      : []),
  ];
}

export function HolidayRegistrationPdfMenu({
  periodId,
  registrationId,
  canSeeHealth,
}: Readonly<{
  periodId: string;
  registrationId: string;
  canSeeHealth: boolean;
}>) {
  const [busy, setBusy] = useState<string | null>(null);
  const { show } = useApiErrorDisplay();

  const download = async (key: string) => {
    setBusy(key);
    try {
      const file = await downloadHolidayRegistrationPdf(
        periodId,
        registrationId,
        key === HEALTH_KEY,
      );
      downloadBlob(
        file.blob,
        file.filename ?? `ferienanmeldung-${registrationId}.pdf`,
      );
    } catch (err) {
      logger.error("traeger_holiday_registration_pdf_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, { object: "die PDF-Datei" });
    } finally {
      setBusy(null);
    }
  };

  return (
    <ExportMenu
      ariaLabel="Anmeldung herunterladen"
      options={options(canSeeHealth)}
      busyKey={busy}
      onSelect={(option) => void download(option.key)}
    />
  );
}
