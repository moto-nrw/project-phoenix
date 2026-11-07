"use client";

// Ein Betreuungsvertrag (#4678, ADR 0153): was der Vertrag festhält, an wen
// er geht, wann er versendet wurde, und das PDF, das die Eltern bekommen.
// Ein Entwurf lässt sich senden oder verwerfen; ein versendeter Vertrag
// behält sein PDF und lässt sich erneut senden. Ein abgeschlossener Vertrag
// (#4680) zeigt, wann und wie er zustande kam; erneut senden lässt er sich mit demselben PDF.

import { Download, RotateCw, Send, Trash2 } from "lucide-react";
import dynamic from "next/dynamic";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  CONTRACT_STATUS_LABELS,
  CONTRACT_STATUS_TONES,
  MAIL_STATE_LABELS,
  MAIL_STATE_TONES,
  SEND_SKIP_LABELS,
  childName,
  formatConclusion,
  formatModules,
  formatNotice,
  formatService,
  formatTerm,
  serviceDays,
} from "~/components/traeger/care-contract-format";
import { CareContractNoticeCard } from "~/components/traeger/care-contract-notice-card";
import {
  CareContractVersions,
  plannedVersion,
} from "~/components/traeger/care-contract-versions";
import { formatEuro } from "~/components/traeger/contribution-format";
import { CareContractSignatureCard } from "~/components/traeger/care-contract-signature-card";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { Input } from "~/components/ui/input";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { PdfPreview } from "~/components/ui/pdf-preview";
import { SectionCard } from "~/components/ui/section-card";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
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
import { mailStatusCopy } from "~/lib/email-delivery-status";
import { downloadBlob } from "~/lib/file-download";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import {
  type CareContractDetail,
  type ContractVersion,
  discardCareContract,
  downloadCareContract,
  fetchCareContract,
  sendCareContracts,
  resendCareContract,
} from "~/lib/traeger-care-contracts-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVertragPage" });

// The parents' change (#5979) reuses the school's review display; it loads
// with the card, so the contract page keeps its bundle budget.
const CareContractChangeCard = dynamic(
  () =>
    import("~/components/traeger/care-contract-change-card").then(
      (module) => module.CareContractChangeCard,
    ),
  { ssr: false },
);

const LIST_HREF = traegerPath("/traeger/vertraege");

export default function TraegerVertragPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const router = useRouter();
  const [contract, setContract] = useState<CareContractDetail | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const actionError = useApiErrorDisplay();
  const discardErrors = useApiFormError();
  const recipientForm = useRef<HTMLFormElement>(null);
  const recipientErrors = useApiFormError(recipientForm);
  const [recipientEmail, setRecipientEmail] = useState("");
  const { success, warning } = useToast();
  const today = useBerlinToday();

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

  // A new Berlin day can put another version in force: read the contract
  // again, so booked care, contribution and PDF follow it (#4681).
  useEffect(() => {
    void load();
  }, [load, today]);

  // The preview follows the stored PDF once the contract is sent.
  const frozen = contract?.documentFrozen;
  const loaded = contract !== null;
  const versionInForce = contract?.version.number;
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
    // `frozen` and the version in force are no inputs of the effect: a new
    // PDF exists only after the first send or when another version holds.
  }, [id, frozen, loaded, versionInForce]);

  const refreshDelivery = async () => {
    setRefreshing(true);
    try {
      await load();
    } finally {
      setRefreshing(false);
    }
  };

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

  const resendToAddress = async () => {
    if (!contract) return;
    recipientErrors.clear();
    setSending(true);
    try {
      const result = await resendCareContract(contract.id, recipientEmail);
      const skip = result.skipped[0];
      if (skip) warning(`Nicht gesendet: ${SEND_SKIP_LABELS[skip.reason]}.`);
      else success("Der Vertrag ist an die neue Adresse unterwegs.");
      setRecipientEmail("");
      await load();
    } catch (err) {
      logger.error("traeger_care_contract_recipient_retry_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void recipientErrors.show(err, { object: "die Versandadresse" });
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
  const canSend =
    isDraft || contract?.status === "versendet" || contract?.status === "aktiv";
  const lastMail = contract?.mailings[0];
  const party = contract?.parties[0];
  const planned = contract
    ? plannedVersion(contract.versions, today)
    : undefined;
  // An active or ended contract shows its versions, so the office sees where
  // an approved change will appear; a draft has only its first version.
  const showVersions =
    contract !== null &&
    (contract.versions.length > 1 ||
      contract.status === "aktiv" ||
      contract.status === "gekuendigt" ||
      contract.status === "widerrufen" ||
      contract.status === "beendet");

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
              {contract.endsOn ? ` bis ${formatDate(contract.endsOn)}` : ""}
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
            {!isDraft && contract.mailings.length > 0 ? (
              <Button
                type="button"
                variant="ghost"
                size="md"
                disabled={refreshing || sending}
                onClick={() => void refreshDelivery()}
              >
                <RotateCw className="mr-2 h-4 w-4" aria-hidden="true" />
                {refreshing ? "Wird aktualisiert …" : "Versand aktualisieren"}
              </Button>
            ) : null}
            {canSend ? (
              <Button
                type="button"
                variant={isDraft ? "success" : "outline"}
                size="md"
                disabled={
                  sending ||
                  refreshing ||
                  lastMail?.state === "queued" ||
                  (!isDraft && !!lastMail?.sendBlock)
                }
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
          {planned ? (
            <Alert
              type="info"
              message={`Ab ${formatDate(planned.effectiveFrom)} gilt Version ${planned.number}. Eine Änderung der Betreuung ist freigegeben. Was sich ändert, steht unten unter „Versionen“.`}
            />
          ) : null}
          <CareContractSignatureCard contract={contract} onChanged={load} />
          <div className="grid gap-6 lg:grid-cols-3">
            <div className="flex flex-col gap-6 lg:col-span-2">
              <SectionCard
                concept={careContractsConcept}
                title="Vertragsangaben"
                description={
                  contract.concluded
                    ? "So steht es im abgeschlossenen Vertrag."
                    : contract.documentFrozen
                      ? "So steht es im versendeten Vertrag."
                      : "So steht es im Vertrag, wenn Sie ihn senden."
                }
              >
                <DataGrid>
                  {contract.concluded ? (
                    <DataField label="Abgeschlossen">
                      {formatConclusion(contract.concluded)}
                    </DataField>
                  ) : null}
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
                    {formatTerm(contract.version.terms, contractEnd(contract))}
                  </DataField>
                  <DataField label="Beitrag im Monat">
                    {contract.version.priceCents !== null
                      ? formatEuro(contract.version.priceCents)
                      : new Set(
                            contract.version.services.map(
                              (service) => service.careOfferingId,
                            ),
                          ).size > 1
                        ? "Laut Beitragsordnung für die gebuchten Module"
                        : "Laut Beitragsordnung (das Angebot hat keinen Preis)"}
                  </DataField>
                </DataGrid>
              </SectionCard>
              <SectionCard
                title="Gebuchte Betreuung"
                description={
                  contract.versions.length > 1
                    ? `Je Wochentag das Modul und bis wann das Kind betreut wird. So gilt es in Version ${contract.version.number}.`
                    : "Je Wochentag das Modul und bis wann das Kind betreut wird."
                }
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
                    <DataField label="E-Mail bei Vertragserstellung">
                      <span className="wrap-anywhere">
                        {party.email || "Keine E-Mail-Adresse"}
                      </span>
                    </DataField>
                    <DataField label="Elternkonto bei Vertragserstellung">
                      {party.hasAccount
                        ? "Konto verknüpft"
                        : "Kein Konto verknüpft"}
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
                  contract.mailings.length > 0
                    ? undefined
                    : contract.signature?.method === "paper"
                      ? "Auf Papier übergeben, nicht per Mail."
                      : "Noch nicht gesendet."
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
                          label={contractMailBadge(mailing).label}
                          tone={contractMailBadge(mailing).tone}
                        />
                      </li>
                    ))}
                  </ol>
                ) : null}
                {!isDraft &&
                canSend &&
                lastMail?.state === "failed" &&
                lastMail.sendBlock !== "stopped" ? (
                  <form
                    ref={recipientForm}
                    noValidate
                    className="mt-5 space-y-3"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void resendToAddress();
                    }}
                  >
                    <FormErrorAlert message={recipientErrors.error} />
                    <p className="text-sm text-gray-600">
                      Sie können eine andere Versandadresse verwenden. Das PDF
                      bleibt gleich. Die Vertragspartei und die bisherigen
                      Versanddaten bleiben erhalten.
                    </p>
                    <Input
                      name="recipient_email"
                      type="email"
                      label="Neue Versandadresse"
                      autoComplete="off"
                      maxLength={320}
                      value={recipientEmail}
                      error={recipientErrors.fieldError("recipient_email")}
                      onChange={(event) =>
                        setRecipientEmail(event.target.value)
                      }
                      disabled={sending || refreshing}
                    />
                    <Button
                      type="submit"
                      variant="primary"
                      disabled={sending || refreshing}
                    >
                      {sending ? "Wird gesendet …" : "An diese Adresse senden"}
                    </Button>
                  </form>
                ) : null}
              </SectionCard>
              {contract.status === "aktiv" ? (
                <CareContractChangeCard
                  contractId={contract.id}
                  onDecided={() => void load()}
                />
              ) : null}
              {contract.concluded ? (
                <CareContractNoticeCard
                  contractId={contract.id}
                  status={contract.status}
                  concludedAt={contract.concluded.at}
                  onChanged={() => void load()}
                />
              ) : null}
            </div>
          </div>
          {showVersions ? (
            <CareContractVersions
              contractId={contract.id}
              versions={contract.versions}
              end={{
                withdrawn: contract.status === "widerrufen",
                endsOn:
                  contract.status === "gekuendigt" ? contract.endsOn : null,
              }}
            />
          ) : null}
          <SectionCard
            title={contract.documentFrozen ? "PDF für die Eltern" : "Vorschau"}
            description={
              contract.documentFrozen
                ? "Dieses PDF gehört zum Versand. Es ändert sich nicht mehr."
                : "So sieht der Vertrag mit Ihrem Briefkopf aus."
            }
          >
            <PdfPreview
              url={pdfUrl}
              title={`Vertrag von ${childName(contract)}`}
            />
          </SectionCard>
        </>
      ) : null}
    </TenantPage>
  );
}

/**
 * The last day of the contract: the end of its newest effective version.
 * A version ends the day before a module switch, which is no end of the
 * contract (#4681).
 */
function contractEnd(contract: CareContractDetail): string | null {
  // The latest effective date, not the highest number: a change may go in
  // front of a planned version.
  const newest = contract.versions
    .filter(
      (version) =>
        version.status === "wirksam" || version.status === "abgeloest",
    )
    .reduce<ContractVersion | null>(
      (latest, version) =>
        latest === null || version.effectiveFrom > latest.effectiveFrom
          ? version
          : latest,
      null,
    );
  return newest ? newest.endsOn : contract.version.endsOn;
}

type ContractMailing = CareContractDetail["mailings"][number];

// The same words as the Abrechnung's letters: only a delivery report reads as
// delivered, and an outcome outranks the plain state of the mail.
function contractMailBadge(mailing: ContractMailing): {
  readonly label: string;
  readonly tone: StatusBadgeTone;
} {
  const copy =
    mailing.sendBlock === "provider"
      ? mailStatusCopy("provider_blocked", "", false, "active")
      : mailing.sendBlock === "provider_unknown"
        ? mailStatusCopy(mailing.outcome, "", false, "unreadable")
        : mailing.state === "queued" || mailing.outcome === "accepted"
          ? null
          : mailStatusCopy(mailing.outcome);
  return (
    copy ?? {
      label: MAIL_STATE_LABELS[mailing.state],
      tone: MAIL_STATE_TONES[mailing.state],
    }
  );
}

// The reason and the next step as they are now. "Prüfen Sie die Adresse" only
// fits a mail that came back; a stop or a block has another way out.
function contractHoldNotice(mailing: ContractMailing): string | null {
  const other = "Unter „Versand“ können Sie eine andere Adresse verwenden.";
  switch (mailing.sendBlock) {
    case "stopped":
      return "Die Schule hat die Mails an diese Person gestoppt. Klären Sie das mit der Schule, oder geben Sie den Vertrag anders weiter.";
    case "address":
      return `Die Adresse ${mailing.recipient} ist bei einer Ihrer Schulen gesperrt. Die Schule kann sie prüfen. ${other}`;
    case "provider":
      return `Die Adresse ${mailing.recipient} ist gesperrt. Der moto-Support muss das klären. ${other}`;
    case "provider_unknown":
      return "Ob diese Adresse gesperrt ist, lässt sich gerade nicht prüfen. Bitte versuchen Sie es später noch einmal.";
    case "delayed":
      return "Die Mail kommt später an, es wird weiter versucht. Bitte senden Sie sie nicht noch einmal, sonst kommt sie doppelt an.";
  }
  switch (mailing.outcome) {
    case "unsubscribed":
      return `${mailing.recipient} hat sich von den Mails abgemeldet. Sprechen Sie die Person direkt an. ${other}`;
    case "complained":
      return `${mailing.recipient} hat eine Mail als unerwünscht gemeldet. Sprechen Sie die Person direkt an. ${other}`;
    case "held_by_stop":
    case "blocked":
    case "provider_blocked":
      return "Die Mail ging damals nicht raus. Die Sperre besteht nicht mehr. Sie können neu senden.";
    case "overdue":
      return "Die Mail ist seit Tagen verzögert. Sie können sie noch einmal senden.";
    default:
      return null;
  }
}

function StatusNotice({
  contract,
}: Readonly<{ contract: CareContractDetail }>) {
  const last = contract.mailings[0];
  if (contract.status === "entwurf") {
    return (
      <Alert
        type="info"
        message="Entwurf. Die Eltern sehen den Vertrag erst, wenn Sie ihn senden. Prüfen Sie die gebuchte Betreuung, den Beginn und den Beitrag."
      />
    );
  }
  // A signed contract shows its signature card; the mail no longer needs a
  // notice unless it failed (#4679).
  if (contract.signature && last?.state !== "failed") return null;
  const held = last ? contractHoldNotice(last) : null;
  if (held) {
    return <Alert type="warning" message={held} />;
  }
  if (last?.state === "failed") {
    return (
      <Alert
        type="warning"
        message={`Die Mail an ${last.recipient} kam nicht an. Prüfen Sie die Adresse. Unter „Versand“ können Sie eine andere Adresse verwenden.`}
      />
    );
  }
  if (contract.status === "gekuendigt" && contract.endsOn) {
    return (
      <Alert
        type="info"
        message={`Gekündigt. Der Vertrag endet am ${formatDate(contract.endsOn)}. Bis dahin gilt die gebuchte Betreuung. Die Bestätigung steht unter „Vertrag beenden“.`}
      />
    );
  }
  if (contract.status === "gekuendigt") {
    return (
      <Alert
        type="warning"
        message="Außerordentlich gekündigt. Prüfen Sie den Grund der Eltern und legen Sie unter „Vertrag beenden“ fest, wann der Vertrag endet. Bis dahin gilt die gebuchte Betreuung."
      />
    );
  }
  if (contract.status === "widerrufen") {
    return (
      <Alert
        type="info"
        message="Widerrufen. Der Vertrag ist vom ersten Tag an aufgehoben und gilt an keinem Tag."
      />
    );
  }
  if (contract.concluded?.method === "mit_anmeldung") {
    return (
      <Alert
        type="success"
        message={`Der Vertrag gilt seit der Annahme der Anmeldung am ${formatBerlinDate(contract.concluded.at)}. Die Eltern bekommen ihn per E-Mail und in der Eltern-App. Sie müssen nichts unterschreiben.`}
      />
    );
  }
  if (last?.state === "queued") {
    return (
      <Alert
        type="info"
        message="Die Mail wartet auf den Versand. Mit „Versand aktualisieren“ prüfen Sie den Stand."
      />
    );
  }
  if (last?.state === "unknown") {
    return (
      <Alert
        type="warning"
        message="Das Ergebnis des Mail-Versands ist unbekannt. Mit „Versand aktualisieren“ prüfen Sie den Stand."
      />
    );
  }
  if (last?.state === "sent") {
    return (
      <Alert
        type="success"
        message={`Gesendet am ${formatBerlinDate(last.sentAt)}. Die Mail ging an ${last.recipient}.`}
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
