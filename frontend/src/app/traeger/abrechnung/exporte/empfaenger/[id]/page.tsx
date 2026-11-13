"use client";

// Ein Empfänger der Finanz-Exporte (#6911, ADR 0221): ob er bereit ist,
// welche Dateien er bekommt, sein Zugang und seine letzten Übergaben.
// Entfernen gibt seine Dateien an den Download zurück.

import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  destinationText,
  EXPORT_RECIPIENT_PATH,
  formatsText,
  HANDOVER_STATE_LABELS,
  missingFieldsText,
} from "~/components/traeger/export-handover-format";
import {
  ExportHandoverHistoryCard,
  ExportRecipientFormCard,
} from "~/components/traeger/export-recipient-cards";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  deleteExportRecipient,
  type ExportRecipient,
  type ExportRecipientPage,
  fetchExportRecipient,
  saveExportRecipient,
} from "~/lib/traeger-export-handovers-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerExportRecipientPage" });

/** Whether the recipient can take a file, and what to do if not. */
function RecipientStateNotice({
  recipient,
}: Readonly<{ recipient: ExportRecipient }>) {
  const assign = (
    <ButtonLink
      href={traegerPath(EXPORT_RECIPIENT_PATH)}
      variant="ghost_tone"
      size="compact"
    >
      Dateien zuordnen
    </ButtonLink>
  );
  if (!recipient.enabled) {
    return (
      <Alert
        type="info"
        title="Die Übergabe ist ausgeschaltet"
        message="Schalten Sie sie unten ein, wenn moto Dateien an diesen Empfänger übergeben soll."
      />
    );
  }
  if (!recipient.ready) {
    return (
      <Alert
        type="warning"
        title="Es fehlen noch Angaben"
        message={`Bis alles ausgefüllt ist, übergibt moto nichts. Es fehlen: ${missingFieldsText(recipient.missingFields)}.`}
      />
    );
  }
  if (recipient.formats.length === 0) {
    return (
      <Alert
        type="info"
        title="Der Empfänger bekommt noch keine Datei"
        message="Wählen Sie in der Übersicht, welche Dateien an ihn gehen."
        action={assign}
      />
    );
  }
  return (
    <Alert
      type="success"
      title="Der Empfänger ist bereit"
      message={`${formatsText(recipient.formats)} ${recipient.formats.length === 1 ? "geht" : "gehen"} an ${destinationText(recipient.host, recipient.remoteDirectory)}.`}
      action={assign}
    />
  );
}

export default function TraegerExportRecipientPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [page, setPage] = useState<ExportRecipientPage | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setPage(await fetchExportRecipient(id));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_export_recipient_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Empfänger",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, id, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const recipient = page?.recipient;
  const last = recipient?.lastHandover;

  return (
    <TenantPage
      title={recipient?.name ?? "Empfänger"}
      back
      backHref={traegerPath(EXPORT_RECIPIENT_PATH)}
      backLabel="Zurück zu den Empfängern"
      stats={
        recipient ? (
          <TenantPageStats
            items={[
              {
                value: recipient.formats.length
                  ? formatsText(recipient.formats)
                  : "Keine Datei",
                label: "bekommt",
              },
              {
                value: last ? HANDOVER_STATE_LABELS[last.state] : "Noch keine",
                label: "letzte Übergabe",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!page && !pageLoad.error}
      error={pageLoad.error}
      loading={!page && !pageLoad.error}
    >
      {page && recipient ? (
        <>
          <RecipientStateNotice recipient={recipient} />
          <ExportRecipientFormCard
            recipient={recipient}
            onSave={async (draft) => {
              const saved = await saveExportRecipient(recipient.id, draft);
              setPage((current) =>
                current ? { ...current, recipient: saved } : current,
              );
            }}
            onDelete={async () => {
              await deleteExportRecipient(recipient.id);
              router.push(traegerPath(EXPORT_RECIPIENT_PATH));
            }}
          />
          <ExportHandoverHistoryCard handovers={page.handovers} />
        </>
      ) : null}
    </TenantPage>
  );
}
