"use client";

// Elternsprechtag in der Eltern-App (#4801): die kommenden Tage der Klasse
// eines Kindes. Eltern sehen nur freie Zeiten, buchen eine je Kind und
// verschieben oder sagen sie bis 24 Stunden vorher ab. Ob sie das für dieses
// Kind dürfen, entscheidet das Backend an der Eltern-Kind-Beziehung.

import {
  use,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";

import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { parseISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { parentPath } from "~/lib/parent-url";
import {
  bookChildParentConference,
  cancelChildParentConference,
  fetchChildParentConferences,
  listMyChildren,
  getChildFeatures,
  type Child,
} from "~/lib/parent-api";
import type {
  ParentConferenceOffer,
  ParentConferenceWindow,
} from "~/lib/parent-conferences";
import { calendarConcept } from "~/lib/concepts/calendar";

const logger = createLogger({ component: "ParentConferencePage" });

export default function ParentConferencePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id: studentId } = use(params);
  const t = useTranslations("parentConference");
  const locale = useLocale();
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const cancelErrors = useApiFormError();

  const [canBook, setCanBook] = useState(false);
  const [child, setChild] = useState<Child | null>(null);
  const [offers, setOffers] = useState<ParentConferenceOffer[] | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [busy, setBusy] = useState<string | null>(null);
  const [cancelTarget, setCancelTarget] =
    useState<ParentConferenceOffer | null>(null);
  const latestCancelRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    let active = true;
    Promise.all([
      listMyChildren(),
      fetchChildParentConferences(studentId),
      getChildFeatures(studentId),
    ])
      .then(([children, loaded, features]) => {
        if (!active) return;
        setCanBook(features.conference_book_allowed === true);
        setChild(
          children.find((entry) => entry.student_id === studentId) ?? null,
        );
        setOffers(loaded);
        clearLoadError();
      })
      .catch((err: unknown) => {
        if (!active) return;
        logger.warn("parent_conference_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: t("errorObject"),
          retry: () => setReloadKey((key) => key + 1),
        });
      });
    return () => {
      active = false;
    };
  }, [studentId, reloadKey, clearLoadError, showLoadError, t]);

  const dateLabel = useCallback(
    (date: string) =>
      parseISODate(date).toLocaleDateString(locale, {
        weekday: "long",
        day: "numeric",
        month: "long",
      }),
    [locale],
  );
  const timeLabel = (window: ParentConferenceWindow) =>
    t("timeRange", { from: window.start_time, to: window.end_time });

  const replaceOffer = (next: ParentConferenceOffer) =>
    setOffers((current) =>
      (current ?? []).map((offer) => (offer.id === next.id ? next : offer)),
    );

  const book = async (
    offer: ParentConferenceOffer,
    window: ParentConferenceWindow,
  ) => {
    if (busy || !canBook) return;
    setBusy(`${offer.id}:${window.start_time}`);
    try {
      const next = await bookChildParentConference(
        studentId,
        offer.id,
        window.start_time,
      );
      replaceOffer(next);
      toast.success(t("booked", { time: timeLabel(window) }));
    } catch (err) {
      logger.warn("parent_conference_book_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: t("errorObjectBooking") });
      setReloadKey((key) => key + 1);
    } finally {
      setBusy(null);
    }
  };

  const confirmCancel = async () => {
    if (!cancelTarget || busy || !canBook) return;
    cancelErrors.clear();
    setBusy(`${cancelTarget.id}:cancel`);
    try {
      await cancelChildParentConference(studentId, cancelTarget.id);
      toast.success(t("cancelled"));
      setCancelTarget(null);
      setReloadKey((key) => key + 1);
    } catch (err) {
      logger.warn("parent_conference_cancel_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await cancelErrors.show(err, {
        object: t("errorObjectBooking"),
        retry: () => latestCancelRef.current(),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestCancelRef.current = () => void confirmCancel();
  });

  const childName = child?.first_name ?? "";
  const header = (
    <ParentPageHeader
      title={t("title")}
      description={t("description", { child: childName })}
      backHref={parentPath(
        `/parents/children/${encodeURIComponent(studentId)}`,
      )}
      backLabel={t("back", { child: childName })}
    />
  );

  if (loadError) {
    return (
      <ParentPage>
        {header}
        <LoadErrorAlert error={loadError} />
      </ParentPage>
    );
  }
  if (offers === null) return <ParentPageSkeleton rows={2} />;

  return (
    <ParentPage>
      {header}
      {offers.length === 0 ? (
        <ParentSection title={t("title")} concept={calendarConcept}>
          <EmptyState title={t("empty", { child: childName })} />
        </ParentSection>
      ) : null}

      {offers.map((offer) => (
        <ParentSection
          key={offer.id}
          title={t("dayTitle", { date: dateLabel(offer.date) })}
          description={[
            offer.teacher_name
              ? t("teacher", { teacher: offer.teacher_name })
              : null,
            offer.location ? t("room", { room: offer.location }) : null,
          ]
            .filter(Boolean)
            .join(" · ")}
          concept={calendarConcept}
        >
          <div className="space-y-4">
            {offer.booking ? (
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium text-gray-700">
                    {t("bookingTitle")}
                  </p>
                  <p className="text-lg font-semibold text-gray-900">
                    {timeLabel(offer.booking)}
                  </p>
                </div>
                {canBook && offer.can_change ? (
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="md"
                    disabled={busy !== null}
                    onClick={() => {
                      cancelErrors.clear();
                      setCancelTarget(offer);
                    }}
                  >
                    {t("cancel")}
                  </Button>
                ) : null}
              </div>
            ) : null}

            {canBook && offer.booking && !offer.can_change ? (
              <Alert type="info" message={t("locked")} />
            ) : (
              <div>
                <p className="mb-2 text-sm font-medium text-gray-700">
                  {t("freeTitle")}
                </p>
                {offer.free_slots.length === 0 ? (
                  <p className="text-sm text-gray-600">{t("noFree")}</p>
                ) : (
                  <>
                    {canBook ? (
                      <p className="mb-3 text-sm text-gray-600">
                        {offer.booking ? t("moveHint") : t("chooseHint")}
                      </p>
                    ) : null}
                    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                      {offer.free_slots.map((window) =>
                        canBook ? (
                          <ChoiceTile
                            key={window.start_time}
                            as="button"
                            tone="green"
                            className="justify-center px-3 py-2.5 text-sm font-semibold"
                            disabled={busy !== null}
                            onClick={() => void book(offer, window)}
                          >
                            {busy === `${offer.id}:${window.start_time}`
                              ? "…"
                              : window.start_time}
                          </ChoiceTile>
                        ) : (
                          <span
                            key={window.start_time}
                            className="text-sm text-gray-600"
                          >
                            {window.start_time}
                          </span>
                        ),
                      )}
                    </div>
                  </>
                )}
              </div>
            )}
            {canBook ? (
              <p className="text-xs text-gray-500">{t("rule")}</p>
            ) : null}
          </div>
        </ParentSection>
      ))}

      <ConfirmationModal
        isOpen={cancelTarget !== null}
        onClose={() => {
          if (!busy) {
            setCancelTarget(null);
            cancelErrors.clear();
          }
        }}
        onConfirm={() => void confirmCancel()}
        title={t("cancelTitle")}
        confirmText={t("cancelConfirm")}
        cancelText={t("keep")}
        confirmVariant="danger"
        isConfirmLoading={busy !== null}
        loadingText="…"
        mobileSheet
      >
        <FormErrorAlert message={cancelErrors.error} className="mb-3" />
        <p className="text-sm text-gray-700">
          {cancelTarget?.booking
            ? t("cancelBody", { time: timeLabel(cancelTarget.booking) })
            : null}
        </p>
      </ConfirmationModal>
    </ParentPage>
  );
}
