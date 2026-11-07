"use client";

/**
 * Vertrag kündigen (#4682, § 312k BGB): the page behind „Verträge hier
 * kündigen“. The parents see their contract and what it allows today, pick a
 * termination or, within its period, a withdrawal, and confirm with „jetzt
 * kündigen“. The confirmation shows here at once and goes out by mail. A
 * contract they ended before shows its confirmation again.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Radio } from "~/components/ui/radio";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import type {
  ContractNotice,
  ContractNoticeOptions,
} from "~/lib/care-contract-notices";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import {
  declareMyContractNotice,
  fetchMyContractNoticeOptions,
} from "~/lib/parent-api";

type Choice = "ordentlich" | "sonderkuendigung" | "widerruf";

const DOCUMENTS_HREF = "/documents";

export function ParentContractNoticePage({
  organizationId,
  contractId,
}: Readonly<{ organizationId: string; contractId: string }>) {
  const t = useTranslations("parentContractNotice");
  const [options, setOptions] = useState<ContractNoticeOptions | null>(null);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;

  const read = useCallback(async () => {
    try {
      setOptions(
        await fetchMyContractNoticeOptions(organizationId, contractId),
      );
      clearLoadError();
    } catch (err) {
      await showLoadError(err, {
        object: t("errorObject"),
        retry: () => void read(),
      });
    }
  }, [organizationId, contractId, clearLoadError, showLoadError, t]);

  useEffect(() => {
    void read();
  }, [read]);

  const ended = options?.notice ?? null;
  return (
    <ParentPage>
      <ParentPageHeader
        title={ended ? t("doneHeading") : t("title")}
        description={ended ? undefined : t("description")}
        backHref={DOCUMENTS_HREF}
        backLabel={t("back")}
      />
      {load.error ? <LoadErrorAlert error={load.error} /> : null}
      {options === null && !load.error ? <ParentPageSkeleton rows={2} /> : null}
      {options && ended ? (
        <NoticeDone options={options} notice={ended} />
      ) : null}
      {options && !ended && options.status !== "aktiv" ? (
        <Alert type="info" message={t("notActive")} />
      ) : null}
      {options && !ended && options.status === "aktiv" ? (
        <NoticeForm
          options={options}
          organizationId={organizationId}
          contractId={contractId}
          onDeclared={setOptions}
        />
      ) : null}
    </ParentPage>
  );
}

/** The contract the parents are about to end. */
function ContractSummary({
  options,
}: Readonly<{ options: ContractNoticeOptions }>) {
  const t = useTranslations("parentContractNotice");
  return (
    <ParentSection
      level={2}
      concept={careContractsConcept}
      title={t("contractTitle", { child: options.childName })}
      description={t("contractFrom", { traeger: options.traegerName })}
    >
      <DataGrid>
        <DataField label={t("bookedCare")}>
          {options.modules}
          <span className="block text-sm font-normal text-gray-500">
            {options.schoolName}
          </span>
        </DataField>
        <DataField label={t("noticeTerms")}>{options.terms}</DataField>
      </DataGrid>
    </ParentSection>
  );
}

function NoticeForm({
  options,
  organizationId,
  contractId,
  onDeclared,
}: Readonly<{
  options: ContractNoticeOptions;
  organizationId: string;
  contractId: string;
  onDeclared: (next: ContractNoticeOptions) => void;
}>) {
  const t = useTranslations("parentContractNotice");
  const locale = useLocale();
  const [choice, setChoice] = useState<Choice>("ordentlich");
  const [sending, setSending] = useState(false);
  const form = useRef<HTMLFormElement>(null);
  const errors = useApiFormError(form);
  const date = (iso: string | null) =>
    iso ? formatDate(iso, false, locale) : "–";

  const submit = async () => {
    errors.clear();
    setSending(true);
    try {
      onDeclared(
        await declareMyContractNotice(
          organizationId,
          contractId,
          choice === "widerruf"
            ? { kind: "widerruf" }
            : { kind: "kuendigung", terminationKind: choice },
        ),
      );
      window.scrollTo({ top: 0 });
    } catch (err) {
      void errors.show(err, { object: t("declareErrorObject") });
    } finally {
      setSending(false);
    }
  };

  const endsOn =
    choice === "sonderkuendigung"
      ? options.specialEndsOn
      : options.ordinaryEndsOn;
  const choices: {
    value: Choice;
    title: string;
    hint: string;
    shown: boolean;
  }[] = [
    {
      value: "ordentlich",
      title: t("ordinary", { date: date(options.ordinaryEndsOn) }),
      hint: t("ordinaryHint"),
      shown: true,
    },
    {
      value: "sonderkuendigung",
      title: t("special", { date: date(options.specialEndsOn) }),
      hint: t("specialHint"),
      shown: options.specialEndsOn !== null,
    },
    {
      value: "widerruf",
      title: t("withdrawal"),
      hint: t("withdrawalHint", { date: date(options.withdrawalUntil) }),
      shown: options.withdrawalUntil !== null,
    },
  ];

  return (
    <>
      <ContractSummary options={options} />
      <form
        ref={form}
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <ParentSection level={2} title={t("chooseTitle")}>
          <div className="space-y-4">
            <FormErrorAlert message={errors.error} />
            <fieldset className="space-y-2">
              <legend className="sr-only">{t("chooseTitle")}</legend>
              {choices
                .filter((item) => item.shown)
                .map((item) => (
                  <ChoiceTile
                    key={item.value}
                    selected={choice === item.value}
                    className="p-4"
                  >
                    <Radio
                      name="notice"
                      value={item.value}
                      checked={choice === item.value}
                      onChange={() => setChoice(item.value)}
                    />
                    <span className="min-w-0">
                      <span className="block text-base text-gray-900">
                        {item.title}
                      </span>
                      <span className="mt-0.5 block font-normal text-gray-600">
                        {item.hint}
                      </span>
                    </span>
                  </ChoiceTile>
                ))}
            </fieldset>
            <Alert
              type="info"
              message={
                choice === "widerruf"
                  ? options.earlyStart
                    ? t("summaryWithdrawalEarly")
                    : t("summaryWithdrawal")
                  : t("summaryTermination", { date: date(endsOn) })
              }
            />
            <div className="flex flex-col gap-3 sm:flex-row-reverse sm:items-center sm:justify-start">
              <Button
                type="submit"
                variant="danger"
                size="base"
                className="w-full sm:w-auto"
                isLoading={sending}
                loadingText={t("sending")}
              >
                {choice === "widerruf"
                  ? t("submitWithdrawal")
                  : t("submitTermination")}
              </Button>
              <p className="text-sm text-gray-600">{t("confirmationHint")}</p>
            </div>
          </div>
        </ParentSection>
      </form>
    </>
  );
}

/** The declaration made, with its confirmation as sent. */
function NoticeDone({
  options,
  notice,
}: Readonly<{ options: ContractNoticeOptions; notice: ContractNotice }>) {
  const t = useTranslations("parentContractNotice");
  const locale = useLocale();
  const withdrawn = notice.kind === "widerruf";
  return (
    <>
      <Alert
        type="success"
        message={
          withdrawn
            ? t("doneWithdrawal")
            : t("doneTermination", {
                date: formatDate(notice.endsOn ?? "", false, locale),
              })
        }
      />
      <ParentSection
        level={2}
        concept={careContractsConcept}
        title={t("contractTitle", { child: options.childName })}
        description={t("contractFrom", { traeger: options.traegerName })}
      >
        <div className="space-y-4">
          <StatusBadge
            label={withdrawn ? t("statusWithdrawn") : t("statusTerminated")}
            tone={withdrawn ? "red" : "orange"}
          />
          <DataGrid>
            <DataField label={withdrawn ? t("withdrawnLabel") : t("endsOn")}>
              {withdrawn
                ? t("withdrawnValue")
                : formatDate(notice.endsOn ?? "", false, locale)}
            </DataField>
            <DataField label={t("declaredAt")}>
              {t("declaredAtValue", {
                date: formatBerlinDate(notice.declaredAt, locale),
                time: formatBerlinTime(notice.declaredAt, locale),
              })}
            </DataField>
            <DataField label={t("bookedCare")}>{options.modules}</DataField>
            <DataField label={t("sentTo")}>
              <span className="wrap-anywhere">{notice.recipient}</span>
            </DataField>
          </DataGrid>
          <figure
            aria-label={t("confirmationTitle")}
            className="moto-content-surface rounded-xl border p-4"
          >
            <figcaption className="mb-2 text-sm font-medium text-gray-900">
              {t("confirmationTitle")}
            </figcaption>
            <p className="text-sm wrap-anywhere whitespace-pre-line text-gray-700">
              {notice.confirmationText}
            </p>
          </figure>
          <ButtonLink href={DOCUMENTS_HREF} variant="outline" size="md">
            {t("back")}
          </ButtonLink>
        </div>
      </ParentSection>
    </>
  );
}
