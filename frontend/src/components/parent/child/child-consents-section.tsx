"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import {
  getChildConsents,
  grantChildPhotoConsent,
  withdrawChildPhotoConsent,
  type ChildConsent,
  type ChildConsentState,
} from "~/lib/parent-api";

type PhotoConsentAction = "withdraw" | "grant";

export function ChildConsentsSection({
  studentId,
}: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentChild.consents");
  const locale = useLocale();
  const [consents, setConsents] = useState<ChildConsent[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [pendingAction, setPendingAction] = useState<PhotoConsentAction | null>(
    null,
  );
  const [submittingAction, setSubmittingAction] = useState(false);
  const [successfulAction, setSuccessfulAction] =
    useState<PhotoConsentAction | null>(null);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // Der Fehler einer Änderung steht im offenen Dialog: ein Toast läge
  // unsichtbar unter ihm.
  const actionErrors = useApiFormError();
  const latestConfirmRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    let active = true;
    async function loadConsents() {
      try {
        const next = await getChildConsents(studentId);
        if (!active) return;
        setConsents(next);
        setLoadFailed(false);
      } catch (err) {
        if (!active) return;
        setLoadFailed(true);
        void showLoadError(err, {
          object: t("errorObjectList"),
          retry: () => {
            clearLoadError();
            setLoadFailed(false);
            setReloadKey((key) => key + 1);
          },
        });
      }
    }
    void loadConsents();
    return () => {
      active = false;
    };
  }, [clearLoadError, reloadKey, showLoadError, studentId, t]);

  async function confirmAction() {
    const action = pendingAction;
    if (!action) return;

    setSubmittingAction(true);
    actionErrors.clear();
    try {
      const updatedConsents =
        action === "grant"
          ? await grantChildPhotoConsent(studentId)
          : await withdrawChildPhotoConsent(studentId);
      setConsents(updatedConsents);
      setPendingAction(null);
      setSuccessfulAction(action);
    } catch (err) {
      void actionErrors.show(err, {
        object: t("errorObjectPhoto"),
        retry: () => latestConfirmRef.current(),
      });
    } finally {
      setSubmittingAction(false);
    }
  }

  useLayoutEffect(() => {
    latestConfirmRef.current = () => void confirmAction();
  });

  if (consents === null && !loadFailed) {
    return <ParentSectionSkeleton rows={4} />;
  }
  if (loadFailed) return <LoadErrorAlert error={loadError} />;

  const visibleConsents =
    consents?.filter((consent) => consent.state !== "not_recorded") ?? [];
  if (visibleConsents.length === 0) return null;

  const canChangePhotoConsent = visibleConsents.some(
    (consent) =>
      consent.key === "photo" && (consent.can_withdraw || consent.can_grant),
  );

  return (
    <>
      <ParentSection
        title={t("title")}
        description={
          canChangePhotoConsent
            ? `${t("description")} ${t("photoDescription")}`
            : t("description")
        }
        concept="confirmations"
      >
        {successfulAction === "withdraw" ? (
          <Alert type="success" message={t("success")} />
        ) : null}
        {successfulAction === "grant" ? (
          <Alert type="success" message={t("grantSuccess")} />
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
                        actionErrors.clear();
                        setSuccessfulAction(null);
                        setPendingAction("withdraw");
                      }}
                    >
                      {t("withdrawButtonShort")}
                    </Button>
                  </div>
                ) : null}
                {consent.key === "photo" &&
                consent.state === "withdrawn" &&
                consent.can_grant ? (
                  <div>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      className="text-moto-green-strong hover:text-moto-green-strong mt-1 h-auto min-h-12 px-0 underline-offset-4 hover:bg-transparent hover:underline"
                      aria-label={t("grantButton")}
                      onClick={() => {
                        actionErrors.clear();
                        setSuccessfulAction(null);
                        setPendingAction("grant");
                      }}
                    >
                      {t("grantButtonShort")}
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
        isOpen={pendingAction !== null}
        onClose={() => {
          actionErrors.clear();
          setPendingAction(null);
        }}
        onConfirm={() => void confirmAction()}
        title={
          pendingAction === "grant" ? t("grantConfirmTitle") : t("confirmTitle")
        }
        confirmText={
          pendingAction === "grant"
            ? t("grantConfirmButton")
            : t("confirmButton")
        }
        cancelText={t("cancel")}
        loadingText={
          pendingAction === "grant" ? t("granting") : t("withdrawing")
        }
        isConfirmLoading={submittingAction}
        isDismissDisabled={submittingAction}
        confirmVariant={pendingAction === "withdraw" ? "danger" : "primary"}
        closeLabel={t("close")}
        backdropLabel={t("close")}
        mobileSheet
      >
        <FormErrorAlert message={actionErrors.error} className="mb-3" />
        <p>
          {pendingAction === "grant" ? t("grantConfirmBody") : t("confirmBody")}
        </p>
      </ConfirmationModal>
    </>
  );
}

function stateTone(state: ChildConsentState): StatusBadgeTone {
  if (state === "granted") return "green";
  if (state === "withdrawn") return "orange";
  return "gray";
}
