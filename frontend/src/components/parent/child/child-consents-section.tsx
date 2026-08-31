"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";
import {
  getChildConsents,
  withdrawChildPhotoConsent,
  type ChildConsent,
  type ChildConsentState,
} from "~/lib/parent-api";

export function ChildConsentsSection({
  studentId,
}: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentChild.consents");
  const locale = useLocale();
  const [consents, setConsents] = useState<ChildConsent[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawFailed, setWithdrawFailed] = useState(false);
  const [withdrawn, setWithdrawn] = useState(false);

  useEffect(() => {
    let active = true;
    async function loadConsents() {
      try {
        const next = await getChildConsents(studentId);
        if (!active) return;
        setConsents(next);
        setLoadFailed(false);
      } catch {
        if (active) setLoadFailed(true);
      }
    }
    void loadConsents();
    return () => {
      active = false;
    };
  }, [reloadKey, studentId]);

  async function confirmWithdrawal() {
    setWithdrawing(true);
    setWithdrawFailed(false);
    try {
      setConsents(await withdrawChildPhotoConsent(studentId));
      setWithdrawOpen(false);
      setWithdrawn(true);
    } catch {
      setWithdrawFailed(true);
    } finally {
      setWithdrawing(false);
    }
  }

  if (consents === null && !loadFailed) {
    return <ParentSectionSkeleton rows={4} />;
  }
  if (loadFailed) {
    return (
      <Alert
        type="error"
        message={t("loadError")}
        action={
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => {
              setLoadFailed(false);
              setReloadKey((key) => key + 1);
            }}
          >
            {t("retry")}
          </Button>
        }
      />
    );
  }

  const visibleConsents =
    consents?.filter((consent) => consent.state !== "not_recorded") ?? [];
  if (visibleConsents.length === 0) return null;

  return (
    <>
      <ParentSection
        title={t("title")}
        description={t("description")}
        concept="confirmations"
      >
        {withdrawn ? <Alert type="success" message={t("success")} /> : null}
        {withdrawFailed ? (
          <Alert type="error" message={t("withdrawError")} />
        ) : null}
        <ul className="divide-y divide-gray-100">
          {visibleConsents.map((consent) => (
            <li
              key={consent.key}
              className="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {t(`items.${consent.key}`)}
                </p>
                {consent.changed_at ? (
                  <p className="mt-1 text-sm text-gray-600">
                    {t(`dates.${consent.state}`, {
                      date: formatBerlinDate(consent.changed_at, locale),
                    })}
                  </p>
                ) : null}
                {consent.key === "photo" &&
                consent.state === "granted" &&
                consent.can_withdraw ? (
                  <div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      className="text-moto-red-strong hover:text-moto-red-strong mt-1 h-auto min-h-12 px-0 underline-offset-4 hover:bg-transparent hover:underline"
                      aria-label={t("withdrawButton")}
                      onClick={() => {
                        setWithdrawFailed(false);
                        setWithdrawOpen(true);
                      }}
                    >
                      {t("withdrawButtonShort")}
                    </Button>
                  </div>
                ) : null}
              </div>
              <div className="flex items-start sm:items-end">
                <StatusBadge
                  label={t(`states.${consent.state}`)}
                  tone={stateTone(consent.state)}
                />
              </div>
            </li>
          ))}
        </ul>
      </ParentSection>
      <ConfirmationModal
        isOpen={withdrawOpen}
        onClose={() => setWithdrawOpen(false)}
        onConfirm={() => void confirmWithdrawal()}
        title={t("confirmTitle")}
        confirmText={t("confirmButton")}
        cancelText={t("cancel")}
        loadingText={t("withdrawing")}
        isConfirmLoading={withdrawing}
        isDismissDisabled={withdrawing}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-strong"
        closeLabel={t("close")}
        backdropLabel={t("close")}
        mobileSheet
      >
        <p>{t("confirmBody")}</p>
      </ConfirmationModal>
    </>
  );
}

function stateTone(state: ChildConsentState): StatusBadgeTone {
  if (state === "granted") return "green";
  if (state === "withdrawn") return "orange";
  return "gray";
}
