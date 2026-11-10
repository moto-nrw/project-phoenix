"use client";

// Feldzuordnung der Sollstellung (#4693): Spalten, Trennzeichen, Datum,
// Zeichensatz und Dateiname der CSV für die Kämmerei, dazu die erste
// Dateinummer. Ohne eigene Zuordnung gilt die Vorgabe von moto. Nur das Recht
// „Finanz-Export“ sieht die Seite.

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { ExportFileNumberCard } from "~/components/traeger/export-file-number-card";
import { SollMappingEditor } from "~/components/traeger/soll-mapping-editor";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ExportSettings,
  fetchExportSettings,
  resetSollMapping,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerSollMappingPage" });

const EXPORTS_PATH = "/traeger/abrechnung/exporte";

const INFO =
  "Die Kämmerei liest die Sollstellung mit ihrem eigenen Programm. Legen Sie hier fest, welche Spalten die Datei in welcher Reihenfolge hat. Die Änderung gilt für jede neue Datei.";

export default function TraegerSollMappingPage() {
  const router = useRouter();
  const [settings, setSettings] = useState<ExportSettings | null>(null);
  // A new key starts the editor again from the stored mapping.
  const [version, setVersion] = useState(0);
  const [resetting, setResetting] = useState(false);
  const [confirmReset, setConfirmReset] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setSettings(await fetchExportSettings());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_soll_mapping_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Feldzuordnung",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const reset = async () => {
    setResetting(true);
    try {
      setSettings(await resetSollMapping());
      setVersion((current) => current + 1);
      setConfirmReset(false);
      success("Die Feldzuordnung ist wieder die Vorgabe von moto.");
    } catch (err) {
      logger.error("traeger_soll_mapping_reset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmReset(false);
      void errors.show(err, { object: "die Feldzuordnung" });
    } finally {
      setResetting(false);
    }
  };

  return (
    <TenantPage
      title="Feldzuordnung"
      stats={
        settings ? (
          <TenantPageStats
            items={[
              { value: settings.mapping.columns.length, label: "Spalten" },
              {
                value: settings.custom
                  ? "Eigene Zuordnung"
                  : "Vorgabe von moto",
                label: "",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!settings && !pageLoad.error}
      info={INFO}
      back
      backHref={traegerPath(EXPORTS_PATH)}
      backLabel="Zurück zu den Exporten"
      actions={
        settings?.custom ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setConfirmReset(true)}
          >
            Vorgabe wiederherstellen
          </Button>
        ) : null
      }
      error={pageLoad.error}
      loading={!settings && !pageLoad.error}
    >
      {settings ? (
        <>
          <ExportFileNumberCard settings={settings} onChanged={setSettings} />
          <SollMappingEditor
            key={version}
            settings={settings}
            onCancel={() => router.push(traegerPath(EXPORTS_PATH))}
            onSaved={(saved) => {
              setSettings(saved);
              setVersion((current) => current + 1);
            }}
          />
          <ConfirmationModal
            isOpen={confirmReset}
            onClose={() => setConfirmReset(false)}
            onConfirm={() => void reset()}
            title="Vorgabe wiederherstellen?"
            confirmText="Ja, wiederherstellen"
            confirmVariant="warning"
            isConfirmLoading={resetting}
          >
            <p>
              Ihre Spalten und Einstellungen gehen verloren. Neue Dateien haben
              dann die Spalten der Vorgabe von moto.
            </p>
          </ConfirmationModal>
        </>
      ) : null}
    </TenantPage>
  );
}
