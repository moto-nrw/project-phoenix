"use client";

// Beitrag in der Zusammenfassung der Ferienanmeldung (#4664): die
// Elternabrechnung rechnet ihn aus den Buchungen mit den Regeln des Trägers.
// Bietet der Träger eine Befreiung bei Leistungsbezug an, geben Eltern hier an,
// dass sie Leistungen bekommen.

import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
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

export function HolidayRegistrationContribution({
  token,
  draft,
  offer,
  onBenefitsDeclaredChange,
}: Readonly<{
  token: string;
  draft: RegistrationDraft;
  offer: HolidayRegistrationOffer;
  onBenefitsDeclaredChange: (declared: boolean) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  const [contribution, setContribution] = useState<HolidayContribution | null>(
    null,
  );
  const [failed, setFailed] = useState(false);
  const children = useMemo(
    () => draft.children.map((child) => quoteBookings(child, offer)),
    [draft.children, offer],
  );
  const benefitsDeclared = draft.benefitsDeclared;

  const load = useCallback(async () => {
    setFailed(false);
    try {
      setContribution(
        await quoteHolidayContribution(token, children, benefitsDeclared),
      );
    } catch (err) {
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
          <>
            {contribution.children.map((child) => (
              <div key={child.position} className="space-y-2">
                <p className="font-semibold text-gray-900">
                  {childName(child.position)}
                </p>
                <DataGrid>
                  {child.lines.map((line, index) => (
                    <DataField
                      key={`${line.kind}-${index}`}
                      label={label(line)}
                    >
                      <span className="tabular-nums">
                        {euro.format(line.amountCents / 100)}
                      </span>
                    </DataField>
                  ))}
                  <DataField label={t("contributionChildTotal")}>
                    <span className="font-semibold tabular-nums">
                      {euro.format(child.totalCents / 100)}
                    </span>
                  </DataField>
                </DataGrid>
              </div>
            ))}
            {contribution.children.length > 1 ? (
              <p className="font-semibold text-gray-900">
                {t("contributionTotal")}:{" "}
                {euro.format(contribution.totalCents / 100)}
              </p>
            ) : null}
          </>
        ) : (
          <p>{t("contributionLoading")}</p>
        )}
        {contribution?.exemptionAvailable || (failed && benefitsDeclared) ? (
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
      </div>
    </SectionCard>
  );
}
