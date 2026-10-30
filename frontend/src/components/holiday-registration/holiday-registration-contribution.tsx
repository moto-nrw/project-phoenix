"use client";

// Beitrag in der Zusammenfassung der Ferienanmeldung (#4664): die
// Elternabrechnung rechnet ihn aus den Buchungen mit den Regeln des Trägers.
// Bietet der Träger eine Befreiung bei Leistungsbezug an, geben Eltern hier an,
// dass sie Leistungen bekommen, und können den Nachweis gleich hochladen
// (#5238). Der Upload ist freiwillig; ohne ihn sammelt das Träger-Büro den
// Nachweis wie bisher ein.

import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ContributionBreakdown } from "~/components/holiday-contribution/contribution-breakdown";
import { FileUploadInput } from "~/components/enrollment/enrollment-file-upload";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import type { FormField } from "~/lib/enrollment-form-schema-api";
import {
  ENROLLMENT_FILE_TYPES,
  MAX_UPLOAD_MB,
  type NewUploadValue,
  type UploadValue,
} from "~/lib/enrollment-uploads";
import type {
  ContributionLine,
  HolidayContribution,
} from "~/lib/holiday-contribution";
import {
  type HolidayRegistrationOffer,
  quoteHolidayContribution,
} from "~/lib/holiday-registration-api";
import { createLogger } from "~/lib/logger";
import {
  quoteBookings,
  type RegistrationDraft,
} from "./holiday-registration-model";

const logger = createLogger({ component: "HolidayRegistrationContribution" });

/** The uploads of this form session; a proof is never stored before. */
function newUploads(values: readonly UploadValue[]): NewUploadValue[] {
  return values.filter((value): value is NewUploadValue => "token" in value);
}

export function HolidayRegistrationContribution({
  token,
  draft,
  offer,
  onBenefitsDeclaredChange,
  onBenefitsProofsChange,
}: Readonly<{
  token: string;
  draft: RegistrationDraft;
  offer: HolidayRegistrationOffer;
  onBenefitsDeclaredChange: (declared: boolean) => void;
  /** The upload sits in an `EnrollmentUploadScope` of the summary. */
  onBenefitsProofsChange: (proofs: NewUploadValue[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  // The upload control speaks with the texts of the online registration's
  // uploads (#4783).
  const uploadText = useTranslations("enrollmentForm");
  const locale = useLocale();
  const proofField = useMemo<FormField>(
    () => ({
      key: "benefits_proofs",
      label: t("benefitsProofLabel"),
      help_text: t("benefitsProofHint"),
      type: "file",
      required: false,
      sort_order: 0,
      file_options: {
        allowed_types: [...ENROLLMENT_FILE_TYPES],
        max_size_mb: MAX_UPLOAD_MB,
        multiple: true,
      },
    }),
    [t],
  );
  const [contribution, setContribution] = useState<HolidayContribution | null>(
    null,
  );
  const [failed, setFailed] = useState(false);
  // Whether the Träger offers the exemption; it stays while a new quote is
  // computed, so the checkbox does not vanish under the pointer.
  const [exemptionOffered, setExemptionOffered] = useState(false);
  const children = useMemo(
    () => draft.children.map((child) => quoteBookings(child, offer)),
    [draft.children, offer],
  );
  const benefitsDeclared = draft.benefitsDeclared;

  // Only the newest request may show its answer: a quick change of the
  // statement on benefits must not be overwritten by an older answer.
  const latest = useRef(0);
  const load = useCallback(async () => {
    const request = ++latest.current;
    // The shown amount belongs to the old choice; it goes until the new one
    // arrives.
    setContribution(null);
    setFailed(false);
    try {
      const quoted = await quoteHolidayContribution(
        token,
        children,
        benefitsDeclared,
      );
      if (request === latest.current) {
        setContribution(quoted);
        setExemptionOffered(quoted.exemptionAvailable);
      }
    } catch (err) {
      if (request !== latest.current) return;
      // The registration can be sent without it; the card says so.
      logger.warn("holiday_registration_contribution_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setContribution(null);
      setFailed(true);
    }
  }, [token, children, benefitsDeclared]);

  useEffect(() => {
    void load();
  }, [load]);

  const euro = useMemo(
    () => new Intl.NumberFormat(locale, { style: "currency", currency: "EUR" }),
    [locale],
  );
  const share = useMemo(
    () => new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }),
    [locale],
  );
  const label = (line: ContributionLine): string => {
    switch (line.kind) {
      case "base":
        return line.weekStartsOn && line.weekEndsOn
          ? `${t("weekRange", {
              starts: formatDate(line.weekStartsOn, false, locale),
              ends: formatDate(line.weekEndsOn, false, locale),
            })} · ${line.timeModelName}`
          : line.timeModelName;
      case "sibling":
        return t("contributionSibling", {
          rank: line.childRank ?? 2,
          discount:
            line.discountBasisPoints !== null
              ? `${share.format(line.discountBasisPoints / 100)} %`
              : euro.format((line.discountCents ?? 0) / 100),
        });
      case "benefits_exemption":
        return t("contributionExemption", { note: line.note });
    }
  };
  const childName = (position: number) => {
    const child = draft.children[position - 1];
    return child ? `${child.firstName.trim()} ${child.lastName.trim()}` : "";
  };

  return (
    <SectionCard title={t("contributionTitle")} headingLevel={3}>
      <div className="space-y-4 text-sm text-gray-700">
        <p>{t("contributionText")}</p>
        {failed ? (
          <div className="space-y-2">
            <p>{t("contributionError")}</p>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void load()}
            >
              {t("contributionRetry")}
            </Button>
          </div>
        ) : contribution ? (
          <ContributionBreakdown
            items={contribution.children.map((child) => ({
              key: String(child.position),
              name: childName(child.position),
              lines: child.lines.map((line) => ({
                label: label(line),
                amount: euro.format(line.amountCents / 100),
              })),
              total: euro.format(child.totalCents / 100),
            }))}
            childTotalLabel={t("contributionChildTotal")}
            totalLabel={t("contributionTotal")}
            total={euro.format(contribution.totalCents / 100)}
          />
        ) : (
          <p>{t("contributionLoading")}</p>
        )}
        {exemptionOffered || (failed && benefitsDeclared) ? (
          <label
            htmlFor="holiday-registration-benefits"
            className="flex items-start gap-3"
          >
            <Checkbox
              id="holiday-registration-benefits"
              name="benefits_declared"
              checked={benefitsDeclared}
              onChange={(event) =>
                onBenefitsDeclaredChange(event.target.checked)
              }
            />
            <span className="space-y-1">
              <span className="block">{t("benefitsLabel")}</span>
              <span className="block text-gray-600">{t("benefitsHint")}</span>
            </span>
          </label>
        ) : null}
        {benefitsDeclared ? (
          <div className="pl-8">
            <FileUploadInput
              field={proofField}
              inputName="benefits_proofs"
              value={draft.benefitsProofs}
              onChange={(next) => onBenefitsProofsChange(newUploads(next))}
              tr={(key, values) => uploadText(key, values)}
            />
          </div>
        ) : null}
      </div>
    </SectionCard>
  );
}
