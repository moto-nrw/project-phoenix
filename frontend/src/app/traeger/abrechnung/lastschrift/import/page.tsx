"use client";

// Import bestehender SEPA-Mandate (#6198), zum Beispiel aus OGS Connect: Datei
// wählen, Spalten zuordnen, jede Zeile prüfen und die bereiten übernehmen.
// moto liest die Datei nur und bewahrt sie nicht auf; die IBAN erscheint nur
// verdeckt. Eine Datei lässt sich nach Korrekturen wieder laden: was schon
// übernommen ist, zeigt moto als „Schon vorhanden“.

import { CheckCircle } from "lucide-react";
import { useSession } from "next-auth/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { SepaImportLogCard } from "~/components/traeger/sepa-import-log-card";
import { SepaImportMappingCard } from "~/components/traeger/sepa-import-mapping-card";
import { importFormatLabel } from "~/components/traeger/sepa-import-format";
import { SepaImportPreviewCard } from "~/components/traeger/sepa-import-preview-card";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { Dropzone } from "~/components/ui/dropzone";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import {
  fetchSepaCreditor,
  type MandateSequence,
} from "~/lib/traeger-sepa-api";
import {
  MAX_IMPORT_FILE_BYTES,
  applySepaImport,
  fetchSepaImportLog,
  previewSepaImport,
  type ImportLogEntry,
  type ImportMapping,
  type ImportResult,
} from "~/lib/traeger-sepa-import-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerSepaImportPage" });

const STEPS = ["Datei wählen", "Spalten zuordnen", "Prüfen und übernehmen"];

const INFO =
  "Hier übernehmen Sie Mandate, die Sie schon in einem anderen Programm haben. Referenz und Datum der Unterschrift bleiben gleich. Die Datei bewahrt moto nicht auf.";

const LASTSCHRIFT = "/traeger/abrechnung/lastschrift";

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export default function TraegerSepaImportPage() {
  const { data: session } = useSession();
  const canBill = hasPermission(session ?? null, "traeger_billing:manage");
  const [creditorId, setCreditorId] = useState<string | null>(null);
  const [log, setLog] = useState<ImportLogEntry[] | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [verified, setVerified] = useState(false);
  const [answers, setAnswers] = useState<Record<number, MandateSequence>>({});
  const [reading, setReading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [done, setDone] = useState<ImportResult | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const { show, showMessage } = useApiErrorDisplay();
  const { success } = useToast();
  // Only the answer to the newest look at the file counts.
  const request = useRef(0);

  const load = useCallback(async () => {
    if (!canBill) return;
    try {
      const [creditor, entries] = await Promise.all([
        fetchSepaCreditor(),
        fetchSepaImportLog(),
      ]);
      setCreditorId(creditor.creditorId);
      setLog(entries);
      clearLoadError();
    } catch (err) {
      logger.error("sepa_import_load_failed", { error: errorText(err) });
      await showLoadError(err, {
        object: "den Import",
        retry: () => void load(),
      });
    }
  }, [canBill, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const read = useCallback(
    async (
      chosen: File,
      mapping: ImportMapping | null,
      sequences: Readonly<Record<number, MandateSequence>> = {},
    ) => {
      const id = ++request.current;
      setVerified(false);
      setReading(true);
      try {
        const next = await previewSepaImport({
          file: chosen,
          mapping,
          sequences,
        });
        if (id === request.current) {
          setResult(next);
          setVerified(true);
        }
      } catch (err) {
        logger.error("sepa_import_preview_failed", { error: errorText(err) });
        if (id === request.current) void show(err, { object: "die Datei" });
      } finally {
        if (id === request.current) setReading(false);
      }
    },
    [show],
  );

  const choose = (chosen: File) => {
    if (!/\.(csv|txt)$/i.test(chosen.name)) {
      showMessage("Bitte wählen Sie eine CSV-Datei.");
      return;
    }
    if (chosen.size > MAX_IMPORT_FILE_BYTES) {
      showMessage("Die Datei ist zu groß. Erlaubt sind 2 MB.");
      return;
    }
    setFile(chosen);
    setResult(null);
    setAnswers({});
    setDone(null);
    void read(chosen, null);
  };

  const remap = (mapping: ImportMapping) => {
    if (!file || !result || applying) return;
    setResult({ ...result, mapping });
    setAnswers({});
    void read(file, mapping);
  };

  const answer = (line: number, sequence: MandateSequence) => {
    if (!file || !result || applying) return;
    const next = { ...answers, [line]: sequence };
    setAnswers(next);
    void read(file, result.mapping, next);
  };

  const apply = async () => {
    if (!file || !result || !verified || reading || applying) return;
    setApplying(true);
    try {
      const applied = await applySepaImport({
        file,
        mapping: result.mapping,
        sequences: answers,
      });
      setDone(applied);
      success(
        applied.counts.imported === 1
          ? "1 Mandat ist übernommen."
          : `${applied.counts.imported} Mandate sind übernommen.`,
      );
      setLog(await fetchSepaImportLog());
    } catch (err) {
      logger.error("sepa_import_apply_failed", { error: errorText(err) });
      void show(err, { object: "die Mandate" });
    } finally {
      setApplying(false);
    }
  };

  const restart = () => {
    request.current++;
    setFile(null);
    setResult(null);
    setVerified(false);
    setAnswers({});
    setDone(null);
    setReading(false);
  };

  const step = !result ? 0 : result.unmapped.length > 0 ? 1 : 2;
  const loading =
    canBill && !pageLoad.error && (creditorId === null || log === null);
  const missingCreditor = creditorId === "";

  return (
    <TenantPage
      title="Mandate importieren"
      info={INFO}
      back
      backHref={traegerPath(LASTSCHRIFT)}
      backLabel="Zurück zur Lastschrift"
      error={pageLoad.error}
      loading={loading}
    >
      <div className="flex flex-col gap-6">
        {done ? (
          <SectionCard>
            <EmptyState
              tone="success"
              icon={<CheckCircle aria-hidden className="size-6" />}
              title={
                done.counts.imported === 1
                  ? "1 Mandat übernommen"
                  : `${done.counts.imported} Mandate übernommen`
              }
              description={`${done.counts.duplicate} schon vorhanden, ${done.counts.clarify + done.counts.conflict + done.counts.invalid} nicht übernommen. Sie finden die Mandate unter Lastschrift.`}
              action={
                <div className="flex flex-wrap justify-center gap-3">
                  <ButtonLink
                    href={traegerPath(LASTSCHRIFT)}
                    variant="primary"
                    size="md"
                  >
                    Zur Lastschrift
                  </ButtonLink>
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={restart}
                    disabled={applying}
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
            {missingCreditor ? (
              <Alert
                type="warning"
                title="Zuerst die Gläubiger-ID eintragen"
                message="Ein Mandat gilt immer für eine Gläubiger-ID. Tragen Sie Ihre unter Einstellungen → Abrechnung ein."
                action={
                  <ButtonLink
                    href={traegerPath("/traeger/einstellungen/abrechnung")}
                    variant="ghost_tone"
                    size="compact"
                  >
                    Zu den Einstellungen
                  </ButtonLink>
                }
              />
            ) : null}
            {file ? (
              <SectionCard
                title="Datei"
                description={
                  result
                    ? `${file.name} · ${importFormatLabel(result.encoding, result.delimiter)} · ${result.headers.length} Spalten`
                    : file.name
                }
                action={
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={restart}
                    disabled={applying}
                  >
                    Andere Datei
                  </Button>
                }
              />
            ) : (
              <SectionCard
                title="Datei wählen"
                description="Eine CSV-Datei mit einer Zeile je Mandat und Überschriften in der ersten Zeile."
              >
                <Dropzone
                  accept=".csv,.txt,text/csv,text/plain"
                  legend="CSV-Datei mit Mandaten"
                  label="CSV-Datei hierher ziehen"
                  dragLabel="Datei hier loslassen"
                  buttonLabel="Datei wählen"
                  hint="Bis 2 MB. Getrennt mit Semikolon oder Komma, aus Excel oder einem anderen Programm."
                  loading={reading}
                  loadingLabel="Datei wird gelesen …"
                  disabled={missingCreditor || loading}
                  onFile={choose}
                />
              </SectionCard>
            )}
            {result ? (
              <SepaImportMappingCard
                headers={result.headers}
                mapping={result.mapping}
                unmapped={result.unmapped}
                creditorId={result.creditorId}
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
                      onClick={() =>
                        void read(file, result?.mapping ?? null, answers)
                      }
                    >
                      Erneut prüfen
                    </Button>
                  ) : undefined
                }
              />
            ) : null}
            {result && verified && result.unmapped.length === 0 ? (
              <SepaImportPreviewCard
                result={result}
                answers={answers}
                applying={applying}
                onAnswer={answer}
                onApply={() => void apply()}
              />
            ) : null}
            {result && result.unmapped.length > 0 ? (
              <Alert
                type="info"
                message={
                  result.unmapped.includes("creditor_id") &&
                  result.unmapped.length === 1
                    ? "Bestätigen Sie die Gläubiger-ID. Dann zeigt moto die Zeilen."
                    : "Ordnen Sie alle Pflichtfelder zu. Dann zeigt moto die Zeilen."
                }
              />
            ) : null}
          </>
        )}
        {log ? <SepaImportLogCard entries={log} /> : null}
      </div>
    </TenantPage>
  );
}
