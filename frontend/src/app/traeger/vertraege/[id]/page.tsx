"use client";

// Ein Betreuungsvertrag (#4678, ADR 0153): was der Vertrag festhält, an wen
// er geht, wann er versendet wurde, und das PDF, das die Eltern bekommen.
// Ein Entwurf lässt sich senden oder verwerfen; ein versendeter Vertrag
// behält sein PDF und lässt sich erneut senden.

import { Download, RotateCw, Send, Trash2 } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  CONTRACT_STATUS_LABELS,
  CONTRACT_STATUS_TONES,
  MAIL_STATE_LABELS,
  MAIL_STATE_TONES,
  SEND_SKIP_LABELS,
  childName,
  formatModules,
  formatNotice,
  formatService,
  formatTerm,
  serviceDays,
} from "~/components/traeger/care-contract-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { parentsConcept } from "~/lib/concepts/parents";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type CareContractDetail,
  discardCareContract,
  downloadCareContract,
  fetchCareContract,
  sendCareContracts,
} from "~/lib/traeger-care-contracts-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVertragPage" });

const LIST_HREF = traegerPath("/traeger/vertraege");

export default function TraegerVertragPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const router = useRouter();
  const [contract, setContract] = useState<CareContractDetail | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const actionError = useApiErrorDisplay();
  const discardErrors = useApiFormError();
  const { success, warning } = useToast();

  const load = useCallback(async () => {
    try {
      setContract(await fetchCareContract(id));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_contract_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Vertrag",
        retry: () => void load(),
      });
    }
  }, [id, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  // The preview follows the stored PDF once the contract is sent.
  const frozen = contract?.documentFrozen;
  const loaded = contract !== null;
  useEffect(() => {
    if (!loaded) return;
    let url: string | null = null;
    let cancelled = false;
    downloadCareContract(id)
      .then((file) => {
        if (cancelled) return;
        url = URL.createObjectURL(file.blob);
        setPdfUrl(url);
      })
      .catch((err: unknown) => {
        // The preview is optional; the button below still offers the PDF and
        // reports its own error.
        logger.warn("traeger_care_contract_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
      if (url) URL.revokeObjectURL(url);
    };
    // `frozen` is no input of the effect: a new PDF exists only after the
    // first send, so the preview reloads then.
  }, [id, frozen, loaded]);

  const send = async () => {
    if (!contract) return;
    setSending(true);
    try {
      const result = await sendCareContracts([contract.id]);
      const skip = result.skipped[0];
      if (skip) warning(`Nicht gesendet: ${SEND_SKIP_LABELS[skip.reason]}.`);
      else
        success(
          contract.status === "entwurf"
            ? "Der Vertrag ist unterwegs: per Mail und in die Eltern-App."
            : "Der Vertrag ist noch einmal unterwegs.",
        );
      await load();
    } catch (err) {
      logger.error("traeger_care_contract_send_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionError.show(err, { object: "der Vertrag" });
    } finally {
      setSending(false);
    }
  };

  const download = async () => {
    try {
      const file = await downloadCareContract(id);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_contract_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionError.show(err, { object: "das PDF" });
    }
  };

  const discard = async () => {
    setDiscarding(true);
    try {
      await discardCareContract(id);
      success("Der Entwurf ist verworfen.");
      router.push(LIST_HREF);
    } catch (err) {
      logger.error("traeger_care_contract_discard_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void discardErrors.show(err, { object: "der Entwurf" });
    } finally {
      setDiscarding(false);
    }
  };

  const isDraft = contract?.status === "entwurf";
  const canSend = isDraft || contract?.status === "versendet";
  const lastMail = contract?.mailings[0];
  const party = contract?.parties[0];

  return (
    <TenantPage
      title={contract ? childName(contract) : "Vertrag"}
      back
      backHref={LIST_HREF}
      backLabel="Zurück zu den Verträgen"
      stats={
        contract ? (
          <span className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-gray-600">
            <StatusBadge
              label={CONTRACT_STATUS_LABELS[contract.status]}
              tone={CONTRACT_STATUS_TONES[contract.status]}
            />
            <span>
              {formatModules(contract.moduleNames)} · {contract.schoolName} · ab{" "}
              {formatDate(contract.effectiveFrom)}
            </span>
          </span>
        ) : null
      }
      statsLoading={!contract && !pageLoad.error}
      actions={
        contract ? (
          <>
            {isDraft ? (
              <Button
                type="button"
                variant="ghost_danger"
                size="md"
                onClick={() => setConfirmDiscard(true)}
              >
                <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
                Entwurf verwerfen
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void download()}
            >
              <Download className="mr-2 h-4 w-4" aria-hidden="true" />
              PDF herunterladen
            </Button>
            {canSend ? (
              <Button
                type="button"
                variant={isDraft ? "success" : "outline"}
                size="md"
                disabled={sending || lastMail?.state === "queued"}
                onClick={() => void send()}
              >
                {isDraft ? (
                  <Send className="mr-2 h-4 w-4" aria-hidden="true" />
                ) : (
                  <RotateCw className="mr-2 h-4 w-4" aria-hidden="true" />
                )}
                {sending
                  ? "Wird gesendet …"
                  : isDraft
                    ? "Vertrag senden"
                    : "Erneut senden"}
              </Button>
            ) : null}
          </>
        ) : null
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !contract}
      overlays={
        <ConfirmDeleteModal
          isOpen={confirmDiscard}
          title="Entwurf verwerfen?"
          description={
            contract
              ? `Der Entwurf für ${childName(contract)} wird gelöscht. Die Anmeldung bleibt; Sie können danach einen neuen Entwurf erstellen.`
              : ""
          }
          gate={{ mode: "twoStep" }}
          confirmLabel="Entwurf verwerfen"
          loading={discarding}
          error={discardErrors.error}
          onConfirm={discard}
          onClose={() => setConfirmDiscard(false)}
        />
      }
    >
      {contract ? (
        <>
          <StatusNotice contract={contract} />
          <div className="grid gap-6 lg:grid-cols-3">
            <div className="flex flex-col gap-6 lg:col-span-2">
              <SectionCard
                concept={careContractsConcept}
                title="Vertragsangaben"
                description={
                  contract.documentFrozen
                    ? "So steht es im versendeten Vertrag."
                    : "So steht es im Vertrag, wenn Sie ihn senden."
                }
              >
                <DataGrid>
                  <DataField label="Kind">{childName(contract)}</DataField>
                  <DataField label="Geboren am">
                    {contract.version.childBirthDate
                      ? formatDate(contract.version.childBirthDate)
                      : "–"}
                  </DataField>
                  <DataField label="Betreuungsort">
                    {contract.version.schoolName}
                    {contract.version.phaseName ? (
                      <span className="block text-sm font-normal text-gray-500">
                        {contract.version.phaseName}
                      </span>
                    ) : null}
                  </DataField>
                  <DataField label="Beginn">
                    {formatDate(contract.version.effectiveFrom)}
                  </DataField>
                  <DataField label="Laufzeit">
                    {formatTerm(
                      contract.version.terms,
                      contract.version.endsOn,
                    )}
                  </DataField>
                  <DataField label="Beitrag im Monat">
                    {contract.version.priceCents === null
                      ? "Kein Beitrag hinterlegt"
                      : formatEuro(contract.version.priceCents)}
                  </DataField>
                </DataGrid>
              </SectionCard>
              <SectionCard
                title="Gebuchte Betreuung"
                description="Je Wochentag das Modul und bis wann das Kind betreut wird."
              >
                <DataGrid>
                  {serviceDays(contract.version.services).map((day) => (
                    <DataField key={day.weekday} label={day.label}>
                      {day.services.length === 0 ? (
                        <span className="font-normal text-gray-500">
                          Keine Betreuung
                        </span>
                      ) : (
                        day.services.map((service) => (
                          <span key={service.careOfferingId} className="block">
                            {formatService(service)}
                          </span>
                        ))
                      )}
                    </DataField>
                  ))}
                </DataGrid>
              </SectionCard>
              <SectionCard title="Kündigung und Satzung">
                <DataGrid>
                  <DataField label="Kündigung">
                    {formatNotice(contract.version.terms)}
                  </DataField>
                  <DataField label="Vertragspartner">
                    {contract.version.terms.contractPartner ||
                      "Der Träger selbst"}
                  </DataField>
                  <DataField label="Sonderkündigung" fullWidth>
                    {contract.version.terms.specialTermination
                      ? contract.version.terms.specialTerminationNote
                      : "Nicht vorgesehen"}
                  </DataField>
                  <DataField label="Satzung" fullWidth>
                    {contract.version.terms.statuteRequired ? (
                      <StatuteValue contract={contract} />
                    ) : (
                      "Keine Satzungsbestätigung"
                    )}
                  </DataField>
                </DataGrid>
              </SectionCard>
            </div>
            <div className="flex flex-col gap-6">
              <SectionCard concept={parentsConcept} title="Eltern">
                {party ? (
                  <DataGrid columns={1}>
                    <DataField label="Vertragspartei">
                      {`${party.firstName} ${party.lastName}`.trim()}
                    </DataField>
                    <DataField label="E-Mail">
                      <span className="wrap-anywhere">
                        {party.email || "Keine E-Mail-Adresse"}
                      </span>
                    </DataField>
                    <DataField label="Eltern-App">
                      {party.hasAccount
                        ? "Hat ein Konto: der Vertrag liegt auch unter „Dokumente“."
                        : "Kein Konto: der Vertrag kommt nur per Mail."}
                    </DataField>
                  </DataGrid>
                ) : (
                  <p className="text-sm text-gray-600">
                    Keine Vertragspartei hinterlegt.
                  </p>
                )}
              </SectionCard>
              <SectionCard
                title="Versand"
                description={
                  contract.mailings.length === 0
                    ? "Noch nicht gesendet."
                    : undefined
                }
              >
                {contract.mailings.length > 0 ? (
                  <ol className="space-y-3">
                    {contract.mailings.map((mailing) => (
                      <li
                        key={mailing.sentAt}
                        className="flex items-start justify-between gap-3"
                      >
                        <span className="min-w-0 text-sm">
                          <span className="block font-medium text-gray-900">
                            {formatBerlinDate(mailing.sentAt)},{" "}
                            {formatBerlinTime(mailing.sentAt)} Uhr
                          </span>
                          <span className="block wrap-anywhere text-gray-500">
                            an {mailing.recipient}
                          </span>
                        </span>
                        <StatusBadge
                          label={MAIL_STATE_LABELS[mailing.state]}
                          tone={MAIL_STATE_TONES[mailing.state]}
                        />
                      </li>
                    ))}
                  </ol>
                ) : null}
              </SectionCard>
            </div>
          </div>
          <SectionCard
            title={contract.documentFrozen ? "Versendetes PDF" : "Vorschau"}
            description={
              contract.documentFrozen
                ? "Dieses PDF haben die Eltern bekommen. Es ändert sich nicht mehr."
                : "So sieht der Vertrag mit Ihrem Briefkopf aus."
            }
          >
            {pdfUrl ? (
              <object
                data={pdfUrl}
                type="application/pdf"
                aria-label={`Vertrag von ${childName(contract)}`}
                className="h-[75vh] min-h-[480px] w-full rounded-xl border border-gray-200"
              >
                <p className="p-4 text-sm text-gray-600">
                  Ihr Browser zeigt das PDF hier nicht an. Laden Sie es oben mit
                  „PDF herunterladen“.
                </p>
              </object>
            ) : (
              <p className="text-sm text-gray-500">Die Vorschau lädt …</p>
            )}
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}

function StatusNotice({
  contract,
}: Readonly<{ contract: CareContractDetail }>) {
  const last = contract.mailings[0];
  if (contract.status === "entwurf") {
    return (
      <Alert
        type="info"
        message="Entwurf. Die Eltern sehen den Vertrag erst, wenn Sie ihn senden. Prüfen Sie Tage, Beginn und Beitrag."
      />
    );
  }
  if (last?.state === "failed") {
    return (
      <Alert
        type="warning"
        message={`Die Mail an ${last.recipient} kam nicht an. Prüfen Sie die Adresse mit der Schule und senden Sie den Vertrag erneut.`}
      />
    );
  }
  if (contract.status === "versendet" && contract.sentAt) {
    return (
      <Alert
        type="success"
        message={`Versendet am ${formatBerlinDate(contract.sentAt)}. Die Eltern haben den Vertrag per Mail${contract.parties[0]?.hasAccount ? " und in der Eltern-App" : ""}.`}
      />
    );
  }
  return null;
}

function StatuteValue({
  contract,
}: Readonly<{ contract: CareContractDetail }>) {
  const { statuteTitle, statuteUrl } = contract.version.terms;
  return (
    <span>
      Die Eltern erkennen die „{statuteTitle}“ an.
      {statuteUrl ? (
        <>
          {" "}
          <a
            href={statuteUrl}
            target="_blank"
            rel="noreferrer"
            className="text-moto-blue-strong underline"
          >
            Satzung öffnen
          </a>
        </>
      ) : null}
    </span>
  );
}
