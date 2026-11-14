"use client";

// Import von Kindern und ihren bestehenden Betreuungsverträgen (#4723), zum
// Beispiel aus OGS Connect: Datei wählen, Spalten zuordnen, jede Zeile prüfen
// und die bereiten übernehmen. moto liest die Datei nur und bewahrt sie nicht
// auf. Eine Datei lässt sich nach Korrekturen wieder laden: Kinder, die ihren
// Vertrag schon haben, zeigt moto als „Schon vorhanden“.

import { CheckCircle, Download } from "lucide-react";
import { useSession } from "next-auth/react";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  CONTRACT_IMPORT_SAMPLE,
  contractCountLabel,
  rowsWithoutConditions,
} from "~/components/traeger/care-contract-import-format";
import { CareContractImportLogCard } from "~/components/traeger/care-contract-import-log-card";
import { CareContractImportMappingCard } from "~/components/traeger/care-contract-import-mapping-card";
import { CareContractImportPreviewCard } from "~/components/traeger/care-contract-import-preview-card";
import { importFormatLabel } from "~/components/traeger/sepa-import-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { Dropzone } from "~/components/ui/dropzone";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  MAX_CONTRACT_IMPORT_FILE_BYTES,
  applyContractImport,
  fetchContractImportLog,
  previewContractImport,
  type ContractImportLogEntry,
  type ContractImportMapping,
  type ContractImportResult,
} from "~/lib/traeger-care-contract-import-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerContractImportPage" });

const STEPS = ["Datei wählen", "Spalten zuordnen", "Prüfen und übernehmen"];

const INFO =
  "Hier übernehmen Sie Kinder und Verträge, die Sie schon in einem anderen Programm haben. Jeder Vertrag gilt sofort als abgeschlossen, Eltern bekommen keine Mail. Die Datei bewahrt moto nicht auf.";

const VERTRAEGE = "/traeger/vertraege";
const BEDINGUNGEN = "/traeger/vertraege/bedingungen";

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function doneDescription(done: ContractImportResult): string {
  const { counts } = done;
  const parts = [
    `${counts.duplicate} schon vorhanden`,
    `${counts.conflict + counts.invalid} nicht übernommen`,
  ];
  if (counts.debtorNumbers > 0) {
    parts.push(
      counts.debtorNumbers === 1
        ? "1 Debitorennummer eingetragen"
        : `${counts.debtorNumbers} Debitorennummern eingetragen`,
    );
  }
  return `${parts.join(", ")}. Sie finden die Verträge unter Verträge mit dem Stand „Aktiv“.`;
}

export default function TraegerContractImportPage() {
  const { data: session } = useSession();
  const canManage = hasPermission(session ?? null, "traeger_contracts:manage");
  const [log, setLog] = useState<ContractImportLogEntry[] | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ContractImportResult | null>(null);
  const [verified, setVerified] = useState(false);
  const [reading, setReading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [done, setDone] = useState<ContractImportResult | null>(null);
  const logLoad = useApiLoadError();
  const { show: showLogError, clear: clearLogError } = logLoad;
  const { show, showMessage } = useApiErrorDisplay();
  const { success } = useToast();
  // Only the answer to the newest look at the file counts.
  const request = useRef(0);

  const loadLog = useCallback(async () => {
    if (!canManage) return;
    try {
      setLog(await fetchContractImportLog());
      clearLogError();
    } catch (err) {
      logger.error("contract_import_log_load_failed", {
        error: errorText(err),
      });
      await showLogError(err, {
        object: "die Liste der Importe",
        // oxlint-disable-next-line ui-kit/menu-item-needs-description -- Aktion der Fehlermeldung, kein Menüeintrag.
        action: { label: "Erneut laden", onClick: () => void loadLog() },
      });
    }
  }, [canManage, clearLogError, showLogError]);

  useEffect(() => {
    void loadLog();
  }, [loadLog]);

  const read = useCallback(
    async (chosen: File, mapping: ContractImportMapping | null) => {
      const id = ++request.current;
      setVerified(false);
      setReading(true);
      try {
        const next = await previewContractImport({ file: chosen, mapping });
        if (id === request.current) {
          setResult(next);
          setVerified(true);
        }
      } catch (err) {
        logger.error("contract_import_preview_failed", {
          error: errorText(err),
        });
        if (id === request.current) void show(err, { object: "die Datei" });
      } finally {
        if (id === request.current) setReading(false);
      }
    },
    [show],
  );

  const choose = (chosen: File) => {
    if (!/\.(csv|txt)$/i.test(chosen.name)) {
      showMessage(
        "Bitte wählen Sie eine CSV-Datei. Eine Excel-Datei speichern Sie vorher als CSV.",
      );
      return;
    }
    if (chosen.size > MAX_CONTRACT_IMPORT_FILE_BYTES) {
      showMessage("Die Datei ist zu groß. Erlaubt sind 2 MB.");
      return;
    }
    setFile(chosen);
    setResult(null);
    setDone(null);
    void read(chosen, null);
  };

  const remap = (mapping: ContractImportMapping) => {
    if (!file || !result || applying) return;
    setResult({ ...result, mapping });
    void read(file, mapping);
  };

  const apply = async () => {
    if (!file || !result || !verified || reading || applying) return;
    setApplying(true);
    try {
      const applied = await applyContractImport({
        file,
        mapping: result.mapping,
        fingerprint: result.fingerprint,
      });
      setDone(applied);
      success(`${contractCountLabel(applied.counts.imported)} übernommen.`);
    } catch (err) {
      logger.error("contract_import_apply_failed", { error: errorText(err) });
      void show(err, { object: "die Verträge" });
      if (
        err instanceof ApiError &&
        err.code === wireErrorCode("care_contract.import_changed")
      ) {
        // The rows changed since the office looked: show the current ones,
        // the import waits for a new confirmation.
        void read(file, result.mapping);
      }
      return;
    } finally {
      setApplying(false);
    }
    await loadLog();
  };

  const restart = () => {
    request.current++;
    setFile(null);
    setResult(null);
    setVerified(false);
    setDone(null);
    setReading(false);
  };

  const step = !result ? 0 : result.unmapped.length > 0 ? 1 : 2;
  const withoutConditions = result ? rowsWithoutConditions(result) : 0;

  return (
    <TenantPage
      title="Verträge importieren"
      info={INFO}
      back
      backHref={traegerPath(VERTRAEGE)}
      backLabel="Zurück zu den Verträgen"
    >
      <div className="flex flex-col gap-6">
        {done ? (
          <SectionCard>
            <EmptyState
              tone="success"
              icon={<CheckCircle aria-hidden className="size-6" />}
              title={`${contractCountLabel(done.counts.imported)} übernommen`}
              description={doneDescription(done)}
              action={
                <div className="flex flex-wrap justify-center gap-3">
                  <ButtonLink
                    href={traegerPath(VERTRAEGE)}
                    variant="primary"
                    size="md"
                    tooltip="Öffnet die Liste aller Verträge."
                  >
                    Zu den Verträgen
                  </ButtonLink>
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={restart}
                    disabled={applying}
                    tooltip="Startet einen neuen Import mit einer anderen Datei."
                  >
                    Weitere Datei laden
                  </Button>
                </div>
              }
            />
          </SectionCard>
        ) : (
          <>
            <SectionCard>
              <WizardStepper steps={STEPS} current={step} />
            </SectionCard>
            {file ? (
              <SectionCard
                title="Datei"
                description={
                  // A long file name without spaces breaks anywhere instead
                  // of running past the card on a phone.
                  <span className="wrap-anywhere">
                    {result
                      ? `${file.name} · ${importFormatLabel(result.encoding, result.delimiter)} · ${result.headers.length} Spalten`
                      : file.name}
                  </span>
                }
                action={
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={restart}
                    disabled={applying}
                    tooltip="Verwirft die gelesene Datei, ohne etwas zu übernehmen."
                  >
                    Andere Datei
                  </Button>
                }
              />
            ) : (
              <SectionCard
                title="Datei wählen"
                description="Eine CSV-Datei mit einer Zeile je Kind und Überschriften in der ersten Zeile."
                action={
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    tooltip="Speichert eine Beispieldatei mit allen Spalten, die moto von selbst erkennt."
                    onClick={() =>
                      downloadBlob(
                        new Blob(["﻿", CONTRACT_IMPORT_SAMPLE], {
                          type: "text/csv;charset=utf-8",
                        }),
                        "vertraege-beispiel.csv",
                      )
                    }
                  >
                    <Download className="mr-2 h-4 w-4" aria-hidden="true" />
                    Beispieldatei
                  </Button>
                }
              >
                <Dropzone
                  accept=".csv,.txt,text/csv,text/plain"
                  legend="CSV-Datei mit Kindern und Verträgen"
                  label="CSV-Datei hierher ziehen"
                  dragLabel="Datei hier loslassen"
                  buttonLabel="Datei wählen"
                  hint="Bis 2 MB. Getrennt mit Semikolon oder Komma, aus Excel, OGS Connect oder einem anderen Programm."
                  loading={reading}
                  loadingLabel="Datei wird gelesen …"
                  onFile={choose}
                />
              </SectionCard>
            )}
            {result ? (
              <CareContractImportMappingCard
                headers={result.headers}
                mapping={result.mapping}
                unmapped={result.unmapped}
                schools={result.schools}
                debtorNumberScope={result.debtorNumberScope}
                disabled={applying}
                onChange={remap}
              />
            ) : null}
            {file && !verified ? (
              <Alert
                type="info"
                message={
                  reading
                    ? "Die Datei wird geprüft …"
                    : "Die Datei ist noch nicht geprüft. Bitte prüfen Sie erneut."
                }
                action={
                  !reading ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      tooltip="Liest die Datei noch einmal und prüft jede Zeile."
                      onClick={() => void read(file, result?.mapping ?? null)}
                    >
                      Erneut prüfen
                    </Button>
                  ) : undefined
                }
              />
            ) : null}
            {withoutConditions > 0 ? (
              <Alert
                type="warning"
                title="Zuerst die Vertragsbedingungen eintragen"
                message={`${withoutConditions === 1 ? "1 Zeile nennt" : `${withoutConditions} Zeilen nennen`} ein Modul ohne Vertragsbedingungen. Tragen Sie Laufzeit und Kündigungsfrist ein und prüfen Sie die Datei danach erneut.`}
                action={
                  <ButtonLink
                    href={traegerPath(BEDINGUNGEN)}
                    variant="ghost_tone"
                    size="compact"
                    tooltip="Öffnet die Vertragsbedingungen der Module."
                  >
                    Zu den Vertragsbedingungen
                  </ButtonLink>
                }
              />
            ) : null}
            {result && result.unmapped.length === 0 ? (
              <CareContractImportPreviewCard
                result={result}
                applying={applying}
                disabled={!verified}
                onApply={() => void apply()}
              />
            ) : null}
            {result && result.unmapped.length > 0 ? (
              <Alert
                type="info"
                message="Ordnen Sie alle Pflichtfelder zu. Dann zeigt moto die Zeilen."
              />
            ) : null}
          </>
        )}
        {logLoad.error ? (
          <LoadErrorAlert error={logLoad.error} />
        ) : log ? (
          <CareContractImportLogCard entries={log} />
        ) : null}
      </div>
    </TenantPage>
  );
}
