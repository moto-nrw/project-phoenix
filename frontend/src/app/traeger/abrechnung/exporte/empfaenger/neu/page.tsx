"use client";

// Neuer Empfänger der Finanz-Exporte (#6911, ADR 0222). Er bekommt noch
// keine Datei; welche Datei er bekommt, legt der Träger danach in der
// Übersicht fest.

import { useRouter } from "next/navigation";
import { EXPORT_RECIPIENT_PATH } from "~/components/traeger/export-handover-format";
import { ExportRecipientFormCard } from "~/components/traeger/export-recipient-cards";
import { TenantPage } from "~/components/ui/tenant-page";
import { createExportRecipient } from "~/lib/traeger-export-handovers-api";
import { traegerPath } from "~/lib/traeger-url";

const INFO =
  "Tragen Sie ein, wohin moto Dateien übergeben soll. Danach wählen Sie in der Übersicht, welche Dateien dieser Empfänger bekommt.";

export default function TraegerNewExportRecipientPage() {
  const router = useRouter();
  return (
    <TenantPage
      title="Neuer Empfänger"
      info={INFO}
      back
      backHref={traegerPath(EXPORT_RECIPIENT_PATH)}
      backLabel="Zurück zu den Empfängern"
    >
      <ExportRecipientFormCard
        recipient={null}
        onSave={async (draft) => {
          await createExportRecipient(draft);
          router.push(traegerPath(EXPORT_RECIPIENT_PATH));
        }}
      />
    </TenantPage>
  );
}
