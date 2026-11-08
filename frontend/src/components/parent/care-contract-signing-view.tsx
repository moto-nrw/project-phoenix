"use client";

// Einen Betreuungsvertrag lesen und unterschreiben (#4679). Dieselbe Ansicht
// dient dem einmaligen Link aus der Mail (ohne Konto) und der Eltern-App
// unter „Dokumente“: erst der Vertrag mit der gebuchten Betreuung und dem
// PDF, dann die Widerrufsbelehrung, dann die Unterschrift. Ein
// unterschriebener Vertrag zeigt nur noch die Bestätigung.

import { CheckCircle2, FileText, ShieldCheck } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";

import { useSignaturePadLabels } from "~/components/parent/news/signature-pad-labels";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { PdfPreview, useObjectUrl } from "~/components/ui/pdf-preview";
import { SectionCard } from "~/components/ui/section-card";
import { SignaturePad } from "~/components/ui/signature-pad";
import {
  CardGridSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError, transportFetch } from "~/lib/api-error";
import {
  fetchSigningView,
  signContract,
  signingDocumentUrl,
  type SigningService,
  type SigningSource,
  type SigningView,
} from "~/lib/care-contract-signing-api";
import { contractWeekdays } from "~/lib/care-contract-weekdays";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { SignatureValue } from "~/lib/signature";

const logger = createLogger({ component: "ContractSigningView" });

const NOTICE_FIELD = "withdrawal_notice_acknowledged";
const SIGNATURE_FIELD = "contract-signature";

function isLinkGone(err: unknown): boolean {
  return (
    err instanceof ApiError && err.code === "care_contract.signing_link_invalid"
  );
}

function serviceDays(services: readonly SigningService[]) {
  return contractWeekdays(services).map((weekday) => ({
    weekday,
    services: services.filter((service) => service.weekday === weekday),
  }));
}

export function ContractSigningView({
  source: initialSource,
}: Readonly<{ source: SigningSource }>) {
  const t = useTranslations("careContractSigning");
  // The page names one contract for its whole life.
  const [source] = useState(initialSource);
  const [view, setView] = useState<SigningView | null>(null);
  const [gone, setGone] = useState(false);
  const loadError = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadError;

  const load = useCallback(async () => {
    try {
      setView(await fetchSigningView(source));
      clearLoadError();
    } catch (err) {
      if (isLinkGone(err)) {
        setGone(true);
        return;
      }
      logger.error("care_contract_signing_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: t("errorObject"),
        retry: () => void load(),
      });
    }
  }, [source, clearLoadError, showLoadError, t]);

  useEffect(() => {
    void load();
  }, [load]);

  if (gone) {
    return (
      <SectionCard concept={careContractsConcept} title={t("invalidTitle")}>
        <p className="text-sm text-gray-700">{t("invalidText")}</p>
      </SectionCard>
    );
  }
  if (loadError.error) return <LoadErrorAlert error={loadError.error} />;
  if (!view)
    return (
      <SkeletonRegion label={t("loading")}>
        <CardGridSkeleton cards={3} className="grid grid-cols-1 gap-5" />
      </SkeletonRegion>
    );
  if (view.signature || view.status !== "versendet") {
    return <SignedConfirmation view={view} />;
  }
  return (
    <div className="space-y-5">
      <ContractFacts view={view} />
      <BookedCare view={view} />
      <ContractDocument source={source} view={view} />
      <WithdrawalNotice view={view} />
      <SignForm source={source} view={view} onSigned={setView} />
    </div>
  );
}

function ContractFacts({ view }: Readonly<{ view: SigningView }>) {
  const t = useTranslations("careContractSigning");
  const locale = useLocale();
  const fee =
    view.priceCents === null
      ? t("feeByRules")
      : new Intl.NumberFormat(locale, {
          style: "currency",
          currency: "EUR",
        }).format(view.priceCents / 100);
  return (
    <SectionCard
      concept={careContractsConcept}
      kicker={t("kicker", { traeger: view.traegerName })}
      title={t("title", { child: view.childName })}
      description={t("intro")}
    >
      <DataGrid>
        <DataField label={t("child")}>{view.childName}</DataField>
        <DataField label={t("place")}>
          {view.schoolName}
          {view.phaseName ? (
            <span className="block text-sm font-normal text-gray-500">
              {view.phaseName}
            </span>
          ) : null}
        </DataField>
        <DataField label={t("startsOn")}>
          {formatDate(view.effectiveFrom, false, locale)}
        </DataField>
        <DataField label={t("endsOn")}>
          {view.endsOn
            ? formatDate(view.endsOn, false, locale)
            : t("openEnded")}
        </DataField>
        <DataField label={t("fee")}>{fee}</DataField>
        <DataField label={t("party")}>{view.partyName || "–"}</DataField>
      </DataGrid>
    </SectionCard>
  );
}

function BookedCare({ view }: Readonly<{ view: SigningView }>) {
  const t = useTranslations("careContractSigning");
  const locale = useLocale();
  return (
    <SectionCard title={t("bookedTitle")} description={t("bookedDescription")}>
      <ul className="divide-y divide-gray-100">
        {serviceDays(view.services).map((day) => (
          <li
            key={day.weekday}
            className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-baseline sm:gap-4"
          >
            <span className="w-28 shrink-0 text-sm font-semibold text-gray-900">
              {t(`weekdays.${day.weekday}`)}
            </span>
            {day.services.length === 0 ? (
              <span className="text-sm text-gray-500">{t("noCare")}</span>
            ) : (
              <span className="flex min-w-0 flex-col gap-0.5 text-sm text-gray-700">
                {day.services.map((service) => (
                  <span key={`${service.moduleName}-${service.endsAt}`}>
                    <span className="font-medium text-gray-900">
                      {service.moduleName}
                    </span>
                    {service.endsAt
                      ? ` · ${t("until", { time: service.endsAt })}`
                      : null}
                    {service.lastDay
                      ? ` · ${t("bookedUntil", { date: formatDate(service.lastDay, false, locale) })}`
                      : null}
                  </span>
                ))}
              </span>
            )}
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}

function ContractDocument({
  source,
  view,
}: Readonly<{ source: SigningSource; view: SigningView }>) {
  const t = useTranslations("careContractSigning");
  const [blob, setBlob] = useState<Blob | null>(null);
  const url = useObjectUrl(blob);
  const documentUrl = signingDocumentUrl(source);

  useEffect(() => {
    let cancelled = false;
    transportFetch(documentUrl, { method: "GET", cache: "no-store" })
      .then(async (response) => {
        if (!response.ok) throw new Error(`document ${response.status}`);
        const loaded = await response.blob();
        if (!cancelled) setBlob(loaded);
      })
      .catch((err: unknown) => {
        // The preview is optional; „PDF öffnen“ still offers the file.
        logger.warn("care_contract_signing_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [documentUrl]);

  return (
    <SectionCard
      icon={FileText}
      title={t("pdfTitle")}
      description={t("pdfDescription")}
      action={
        <a
          href={url ?? documentUrl}
          target="_blank"
          rel="noreferrer"
          className="text-moto-blue-strong inline-flex items-center gap-2 text-sm font-semibold underline-offset-4 hover:underline"
        >
          {t("openPdf")}
        </a>
      }
    >
      <PdfPreview url={url} title={t("pdfTitle")} className="max-h-[40rem]" />
      <p className="mt-3 text-xs break-all text-gray-500">
        {t("checksum")}:{" "}
        <span className="font-mono">{view.documentSha256}</span>
      </p>
    </SectionCard>
  );
}

function WithdrawalNotice({ view }: Readonly<{ view: SigningView }>) {
  const t = useTranslations("careContractSigning");
  return (
    <SectionCard
      icon={ShieldCheck}
      title={t("withdrawalTitle")}
      description={t("withdrawalDescription")}
    >
      <div className="space-y-3 text-sm leading-relaxed text-gray-700">
        {view.withdrawalNotice.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </div>
    </SectionCard>
  );
}

function SignForm({
  source,
  view,
  onSigned,
}: Readonly<{
  source: SigningSource;
  view: SigningView;
  onSigned: (view: SigningView) => void;
}>) {
  const t = useTranslations("careContractSigning");
  const labels = useSignaturePadLabels();
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const [notice, setNotice] = useState(false);
  const [earlyStart, setEarlyStart] = useState(false);
  const [signature, setSignature] = useState<SignatureValue | null>(null);
  const [submitted, setSubmitted] = useState(false);
  const [saving, setSaving] = useState(false);

  const submit = async () => {
    setSubmitted(true);
    formError.clear();
    if (!notice || !signature) return;
    setSaving(true);
    try {
      onSigned(
        await signContract(source, {
          signature,
          noticeAcknowledged: notice,
          earlyStart,
        }),
      );
    } catch (err) {
      logger.error("care_contract_signing_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formError.show(err, { object: t("submitObject") });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      concept={careContractsConcept}
      title={t("signTitle")}
      description={t("signDescription")}
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formError.error} />
        <ConsentCheckbox
          name={NOTICE_FIELD}
          required
          checked={notice}
          onChange={setNotice}
          label={t("noticeConfirm")}
          error={submitted && !notice ? t("noticeRequired") : undefined}
        />
        <ConsentCheckbox
          name="early_start_requested"
          checked={earlyStart}
          onChange={setEarlyStart}
          label={t("earlyStart")}
          description={t("earlyStartHint")}
        />
        <SignaturePad
          id={SIGNATURE_FIELD}
          value={signature}
          onChange={setSignature}
          disabled={saving}
          signerNameHint={view.partyName}
          labels={labels}
          error={submitted && !signature ? t("signatureRequired") : null}
        />
        <div className="flex justify-end">
          <Button
            type="submit"
            variant="success"
            size="lg"
            disabled={saving}
            className="w-full sm:w-auto"
          >
            {saving ? t("submitting") : t("submit")}
          </Button>
        </div>
      </form>
    </SectionCard>
  );
}

function SignedConfirmation({ view }: Readonly<{ view: SigningView }>) {
  const t = useTranslations("careContractSigning");
  const locale = useLocale();
  const signature = view.signature;
  return (
    <SectionCard
      icon={CheckCircle2}
      kicker={t("kicker", { traeger: view.traegerName })}
      title={signature ? t("signedTitle") : t("alreadyInForce")}
    >
      <div className="space-y-4">
        {signature ? (
          <Alert
            type="success"
            message={`${t("signedBy", {
              name: signature.signerName,
              date: formatBerlinDate(signature.signedAt, locale),
              time: formatBerlinTime(signature.signedAt, locale),
            })} ${t("signedMail")}`}
          />
        ) : null}
        <DataGrid>
          <DataField label={t("child")}>{view.childName}</DataField>
          <DataField label={t("startsOn")}>
            {formatDate(view.effectiveFrom, false, locale)}
          </DataField>
          {signature?.withdrawalEndsOn ? (
            <DataField label={t("withdrawalTitle")} fullWidth>
              {t("withdrawalUntil", {
                date: formatDate(signature.withdrawalEndsOn, false, locale),
              })}
              {signature.earlyStartRequested ? (
                <span className="block text-sm font-normal text-gray-500">
                  {t("earlyStartChosen")}
                </span>
              ) : null}
            </DataField>
          ) : null}
        </DataGrid>
      </div>
    </SectionCard>
  );
}
