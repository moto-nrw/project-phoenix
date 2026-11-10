"use client";

// The renewal banners of the status page (#2251, #6456). A renewal carries
// no acceptance of the earlier phase: the parents accept the required blocks
// of the new phase's form here. An opt-in renewal is confirmed with it; an
// opt-out renewal keeps running and only records the acceptance.

import { useState } from "react";
import { useLocale } from "next-intl";
import { FileText } from "lucide-react";

import { Button, ButtonLink } from "~/components/ui/button";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { Modal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { DocumentAttachmentList } from "~/components/enrollment/attachment-list";
import { LegalMarkdown } from "~/components/enrollment/legal-markdown";
import { useCareTermTranslations } from "~/lib/care-facility-term";
import { formatDate } from "~/lib/date-helpers";
import type { PublicLegalBlock } from "~/lib/enrollment-form-schema-api";
import type {
  RenewalConfirmation,
  StatusRenewalTerms,
} from "~/lib/enrollment-submission-api";
import { localizeLegalBlocks } from "~/lib/enrollment-translations";

export type RenewalMode = "opt_in" | "opt_out" | "submitted";

interface RenewalBannersProps {
  /** Link to the reduced offerings/weekdays flow; null while a change request is already open (#2251). */
  readonly adjustHref: string | null;
  readonly confirmingRenewal: boolean;
  /** Which banner the children's statuses call for; null shows none. */
  readonly mode: RenewalMode | null;
  readonly terms: StatusRenewalTerms | undefined;
  readonly withdrawingAll: boolean;
  readonly onConfirmRenewal: (
    confirmation: RenewalConfirmation,
  ) => Promise<void>;
  readonly onWithdraw: (childId?: string) => void;
}

const fieldName = (key: string) => `renewal-terms-${key}`;

export function RenewalBanners({
  adjustHref,
  confirmingRenewal,
  mode,
  terms,
  withdrawingAll,
  onConfirmRenewal,
  onWithdraw,
}: RenewalBannersProps) {
  const t = useCareTermTranslations("enrollmentStatus");
  const locale = useLocale();
  const [ticks, setTicks] = useState<Record<string, boolean>>({});
  const [showMissing, setShowMissing] = useState(false);
  const [openBlock, setOpenBlock] = useState<PublicLegalBlock | null>(null);
  if (mode === null) return null;

  const blocks = localizeLegalBlocks(terms?.blocks ?? [], locale);
  const confirmedAt = terms?.confirmed_at ?? null;
  // An opt-out renewal asks only while the parents have not accepted these
  // blocks; an opt-in renewal always asks with its confirmation.
  const asksForTerms =
    blocks.length > 0 && (mode === "opt_in" || confirmedAt === null);

  const handleConfirm = async () => {
    const missing = blocks.filter((block) => ticks[block.key] !== true);
    if (missing.length > 0) {
      setShowMissing(true);
      document.getElementById(fieldName(missing[0]!.key))?.focus();
      return;
    }
    await onConfirmRenewal({
      consentFlags: Object.fromEntries(
        blocks.map((block) => [block.key, true]),
      ),
      formSchemaId: terms?.form_schema_id ?? "0",
      termsFingerprint: terms?.terms_fingerprint ?? "",
    });
  };

  const termsFields = asksForTerms ? (
    <div className="mt-4 space-y-3">
      <p className="text-sm text-gray-700">{t("renewalTermsIntro")}</p>
      {blocks.map((block) => (
        <div key={block.key}>
          <ConsentCheckbox
            name={fieldName(block.key)}
            label={block.label}
            checked={ticks[block.key] === true}
            onChange={(checked) =>
              setTicks((previous) => ({ ...previous, [block.key]: checked }))
            }
            required
            error={
              showMissing && ticks[block.key] !== true
                ? t("renewalTermsMissing")
                : undefined
            }
            action={
              block.text ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  aria-label={t("renewalTermsReadAria", { title: block.title })}
                  onClick={() => setOpenBlock(block)}
                  className="shrink-0"
                >
                  <FileText className="h-3.5 w-3.5" aria-hidden="true" />
                  <span>{t("renewalTermsRead")}</span>
                </Button>
              ) : undefined
            }
          />
          <DocumentAttachmentList attachments={block.attachments} />
        </div>
      ))}
    </div>
  ) : null;

  const adjustLink = adjustHref ? (
    <ButtonLink href={adjustHref} variant="surface" size="md">
      {t("renewalAdjust")}
    </ButtonLink>
  ) : null;

  return (
    <>
      {mode === "opt_in" ? (
        <SectionCard title={t("renewalRequiredTitle")}>
          <p className="text-sm text-gray-700">{t("renewalRequiredText")}</p>
          {termsFields}
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <Button
              type="button"
              onClick={() => void handleConfirm()}
              disabled={confirmingRenewal}
              size="md"
            >
              {confirmingRenewal ? t("confirming") : t("confirmEnrollment")}
            </Button>
            {adjustLink}
            <Button
              type="button"
              onClick={() => onWithdraw()}
              disabled={withdrawingAll}
              variant="surface"
              size="md"
            >
              {withdrawingAll ? t("declining") : t("declineEnrollment")}
            </Button>
          </div>
          <p className="mt-3 text-xs text-gray-500">{t("renewalAdjustHint")}</p>
        </SectionCard>
      ) : (
        <SectionCard
          title={t(mode === "submitted" ? "confirmTerms" : "autoRenewedTitle")}
        >
          <p className="text-sm text-gray-700">
            {mode === "submitted"
              ? t(confirmedAt === null ? "childPendingText" : "termsConfirmed")
              : blocks.length === 0
                ? t("autoRenewedText")
                : confirmedAt === null
                  ? t("autoRenewedTermsText")
                  : t("autoRenewedTermsConfirmedText", {
                      date: formatDate(confirmedAt),
                    })}
          </p>
          {termsFields}
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            {asksForTerms ? (
              <Button
                type="button"
                onClick={() => void handleConfirm()}
                disabled={confirmingRenewal}
                size="md"
              >
                {confirmingRenewal ? t("confirming") : t("confirmTerms")}
              </Button>
            ) : null}
            {adjustLink}
            <Button
              type="button"
              onClick={() => onWithdraw()}
              disabled={withdrawingAll}
              variant="surface"
              size="md"
            >
              {withdrawingAll ? t("unsubscribing") : t("unsubscribe")}
            </Button>
          </div>
          {mode !== "submitted" ? (
            <p className="mt-3 text-xs text-gray-500">
              {t("renewalAdjustHint")}
            </p>
          ) : null}
        </SectionCard>
      )}
      <Modal
        isOpen={openBlock !== null}
        onClose={() => setOpenBlock(null)}
        title={openBlock?.title ?? ""}
        closeLabel={t("modalClose")}
        backdropLabel={t("modalBackdropClose")}
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      >
        <div className="max-h-[60vh] overflow-y-auto text-sm text-gray-700">
          <LegalMarkdown text={openBlock?.text ?? ""} />
        </div>
      </Modal>
    </>
  );
}
