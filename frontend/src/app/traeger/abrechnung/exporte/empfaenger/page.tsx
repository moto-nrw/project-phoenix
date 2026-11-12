"use client";

// Empfänger der Finanz-Exporte (#6824, ADR 0218): Wohin moto Sollstellung,
// DATEV-Datei und Lastschriftdatei übergibt, mit eigenem Zugang des Trägers.
// Das Passwort bleibt verschlüsselt und kommt nie zurück. Darunter jeder
// Versuch einer Übergabe mit seinem Ergebnis. Nur das Recht „Finanz-Export“
// sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import {
  destinationText,
  missingFieldsText,
} from "~/components/traeger/export-handover-format";
import {
  ExportHandoverHistoryCard,
  ExportRecipientFormCard,
} from "~/components/traeger/export-recipient-cards";
import { Alert } from "~/components/ui/alert";
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

const EXPORTS_PATH = "/traeger/abrechnung/exporte";

const INFO =
  "Hier tragen Sie ein, wohin moto Ihre Exporte übergibt, zum Beispiel an Ihre Kämmerei. moto legt die Datei dort sicher ab und schreibt in das Exportprotokoll, ob sie angekommen ist. Herunterladen können Sie die Dateien weiterhin.";

/** The notice of a recipient that cannot take a file yet. */
function RecipientStateNotice({
  recipient,
}: Readonly<{ recipient: ExportRecipient }>) {
  if (recipient.ready) {
    return (
      <Alert
        type="success"
        title="Der Empfänger ist bereit"
        message={`Übergaben gehen an ${destinationText(recipient.host, recipient.remoteDirectory)}.`}
      />
    );
  }
  if (recipient.configured && !recipient.enabled) {
    return (
      <Alert
        type="info"
        title="Die Übergabe ist ausgeschaltet"
        message="Schalten Sie sie unten ein, wenn moto Dateien an diesen Empfänger übergeben soll."
      />
    );
  }
  return (
    <Alert
      type="warning"
      title={
        recipient.configured
          ? "Es fehlen noch Angaben"
          : "Tragen Sie den Empfänger ein"
      }
      message={`Bis alles ausgefüllt ist, übergibt moto nichts. Es fehlen: ${missingFieldsText(recipient.missingFields)}.`}
    />
  );
}

export default function TraegerExportRecipientPage() {
  const [page, setPage] = useState<ExportRecipientPage | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setPage(await fetchExportRecipient());
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
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const arrived =
    page?.handovers.filter((handover) => handover.state === "transferred")
      .length ?? 0;

  return (
    <TenantPage
      title="Empfänger der Exporte"
      info={INFO}
      back
      backHref={traegerPath(EXPORTS_PATH)}
      backLabel="Zurück zu den Exporten"
      stats={
        page ? (
          <TenantPageStats
            items={[
              {
                value: `${arrived} von ${page.handovers.length}`,
                label: "letzten Übergaben angekommen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!page && !pageLoad.error}
      error={pageLoad.error}
      loading={!page && !pageLoad.error}
    >
      {page ? (
        <>
          <RecipientStateNotice recipient={page.recipient} />
          <ExportRecipientFormCard
            recipient={page.recipient}
            onSave={async (draft) => {
              const recipient = await saveExportRecipient(draft);
              setPage((current) =>
                current ? { ...current, recipient } : current,
              );
            }}
            onDelete={async () => {
              const recipient = await deleteExportRecipient();
              setPage((current) =>
                current ? { ...current, recipient } : current,
              );
            }}
          />
          <ExportHandoverHistoryCard handovers={page.handovers} />
        </>
      ) : null}
    </TenantPage>
  );
}
