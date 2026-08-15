"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { OfferingChangeRequestModal } from "~/components/parent/offering-change-request-modal";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  getChildCareOfferings,
  getChildCareSchedule,
  submitOfferingChangeRequest,
  withdrawOfferingChangeRequest,
  type ChildCareOfferings,
  type ChildCareSchedule,
  type OfferingChangesDisabledReason,
} from "~/lib/parent-api";

const logger = createLogger({ component: "BookedCareSection" });

/** Platzhalter fuer eine nicht gesetzte Zeit. Kein Gedankenstrich in Fliesstext. */
const EMPTY = "—";

const WEEKDAYS = [1, 2, 3, 4, 5] as const;
const WEEKDAY_DAY_KEYS: Record<number, string> = {
  1: "mon",
  2: "tue",
  3: "wed",
  4: "thu",
  5: "fri",
};

const DAY_KEY_TO_WEEKDAY: Record<string, number> = {
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
  sun: 7,
};

const DISABLED_REASON_KEYS: Record<OfferingChangesDisabledReason, string> = {
  no_enrollment: "careOfferings.disabledNoEnrollment",
  no_permission: "careOfferings.disabledNoPermission",
  school_disabled: "careOfferings.disabledSchoolOff",
  no_time_remaining: "careOfferings.disabledPeriodOver",
  period_over: "careOfferings.disabledPeriodOver",
};

/**
 * "Gebuchte Betreuung": was tatsaechlich gebucht ist, an welchen Tagen, und
 * wann das Kind kommt und geht.
 *
 * Der Wochenplan ist hier **reine Anzeige**. Woher die Bringzeit stammt,
 * entscheidet die Schule (Stundenplan, Anmeldung oder Handeintrag), deshalb
 * nennt der Hinweis keinen Mechanismus. Sie als Eingabefeld anzubieten war
 * die Ursache falscher Elternaenderungen (#2250). Der Block "AGs und Gruppen"
 * entfaellt ersatzlos (#2303), die Rubrik "Betreuungszeiten" ebenso (#2302).
 */
export function BookedCareSection({
  studentId,
}: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentMasterData");
  const tc = useTranslations("parentChild");
  const [offerings, setOfferings] = useState<ChildCareOfferings | null>(null);
  const [schedule, setSchedule] = useState<ChildCareSchedule | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [confirmWithdraw, setConfirmWithdraw] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    const [offeringsResult, scheduleResult] = await Promise.allSettled([
      getChildCareOfferings(studentId),
      getChildCareSchedule(studentId),
    ]);
    if (offeringsResult.status === "fulfilled") {
      setOfferings(offeringsResult.value);
    } else {
      setError(true);
      logger.warn("booked_care_offerings_load_failed", {
        error: String(offeringsResult.reason),
        student_id: studentId,
      });
    }
    // Der Wochenplan darf fehlen, ohne den Abschnitt zu kippen: die gebuchten
    // Angebote sind die eigentliche Aussage.
    if (scheduleResult.status === "fulfilled") {
      setSchedule(scheduleResult.value);
    } else {
      logger.warn("booked_care_schedule_load_failed", {
        error: String(scheduleResult.reason),
        student_id: studentId,
      });
    }
    setLoading(false);
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const refresh = useCallback(() => void load(), [load]);
  useMessagesActivity({
    eventName: "parent-conversation-refresh",
    studentId,
    onMatch: refresh,
    marksRead: false,
    refetchOnFocus: true,
  });

  const weekdayList = useCallback(
    (weekdays: number[]) =>
      weekdays.length === 0
        ? t("careOfferings.allDays")
        : weekdays.map((day) => t(`weekdayShort.${day}`)).join(", "),
    [t],
  );

  const dayKeyList = useCallback(
    (days: string[]) =>
      weekdayList(
        days.flatMap((day) => {
          const weekday = DAY_KEY_TO_WEEKDAY[day];
          return weekday === undefined ? [] : [weekday];
        }),
      ),
    [weekdayList],
  );

  const diffValue = useCallback(
    (state: "not_booked" | "removed" | "booked", days: string[]) => {
      if (state === "not_booked") return t("careOfferings.notBooked");
      if (state === "removed") return t("careOfferings.removed");
      return dayKeyList(days);
    },
    [dayKeyList, t],
  );

  const byWeekday = useMemo(() => {
    const map = new Map<number, ChildCareSchedule["weekdays"][number]>();
    for (const entry of schedule?.weekdays ?? []) map.set(entry.weekday, entry);
    return map;
  }, [schedule]);

  const periodLabel =
    offerings?.period_name && offerings.period_start && offerings.period_end
      ? t("careOfferings.period", {
          name: offerings.period_name,
          from: formatDate(offerings.period_start),
          until: formatDate(offerings.period_end),
        })
      : null;

  if (loading) {
    return <Skeleton className="h-64 w-full rounded-2xl" />;
  }

  if (error || !offerings) {
    return (
      <ParentSection title={tc("sections.care")}>
        <Alert type="error" message={t("careOfferings.loadError")} />
      </ParentSection>
    );
  }

  const pending = offerings.pending_request;
  const decision = offerings.last_decision;

  const handleWithdraw = async () => {
    if (!pending) return;
    setWithdrawing(true);
    setWithdrawError(null);
    try {
      setOfferings(await withdrawOfferingChangeRequest(studentId, pending.id));
      setConfirmWithdraw(false);
    } catch (err) {
      logger.warn("booked_care_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setWithdrawError(t("careOfferings.withdrawError"));
    } finally {
      setWithdrawing(false);
    }
  };

  return (
    <ParentSection
      title={tc("sections.care")}
      description={periodLabel ?? undefined}
    >
      <div className="space-y-3">
        <h3 className="text-[19px] font-bold text-gray-900">
          {t("careOfferings.offeringsTitle")}
        </h3>
        {offerings.offerings.length === 0 ? (
          <p className="text-[15px] text-gray-600">
            {t("careOfferings.noOfferings")}
          </p>
        ) : (
          <ul className="space-y-2">
            {offerings.offerings.map((offering) => (
              <li
                key={`${offering.id}-${offering.valid_from ?? "current"}`}
                className="rounded-xl border border-gray-200 p-3"
              >
                <p className="text-[17px] font-semibold text-gray-900">
                  {offering.name}
                </p>
                <p className="mt-0.5 text-[15px] text-gray-700">
                  {weekdayList(offering.weekdays)}
                </p>
                <p className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[15px] text-gray-500">
                  {offering.starts_later && offering.valid_from && (
                    <span>
                      {t("careOfferings.startsLater", {
                        date: formatDate(offering.valid_from),
                      })}
                    </span>
                  )}
                  {offering.includes_lunch && (
                    <span>{t("careOfferings.lunch")}</span>
                  )}
                  {offering.includes_holiday_care && (
                    <span>{t("careOfferings.holidayCare")}</span>
                  )}
                  {offering.valid_until && (
                    <span>
                      {t("careOfferings.until", {
                        date: formatDate(offering.valid_until),
                      })}
                    </span>
                  )}
                </p>
              </li>
            ))}
          </ul>
        )}
      </div>

      {schedule && (
        <div className="space-y-3 border-t border-gray-100 pt-4">
          <h3 className="text-[19px] font-bold text-gray-900">
            {tc("care.weekTitle")}
          </h3>
          {/* Mobil eine Karte je Wochentag statt einer Tabelle. Alle Werte sind
              Anzeige, kein Feld: die Zeiten pflegt die OGS. */}
          <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5">
            {WEEKDAYS.map((num) => {
              const day = byWeekday.get(num);
              const modes =
                day && day.modes.length > 0
                  ? day.modes
                      .map((mode) => t(`departureModes.${mode}`))
                      .join(", ")
                  : EMPTY;
              return (
                <div
                  key={num}
                  className="rounded-xl border border-gray-200 p-3"
                >
                  <dt className="text-[17px] font-semibold text-gray-900">
                    {t(`departureDays.${WEEKDAY_DAY_KEYS[num]}`)}
                  </dt>
                  <dd className="mt-1.5 space-y-1.5">
                    <CareFact
                      label={t("careSchedule.arrival")}
                      value={day?.arrival}
                    />
                    <CareFact
                      label={t("careSchedule.pickup")}
                      value={day?.pickup}
                    />
                    <CareFact label={t("careSchedule.modes")} value={modes} />
                  </dd>
                </div>
              );
            })}
          </dl>
          <p className="text-[15px] text-gray-500">
            {tc("care.arrivalReadOnly")}
          </p>
        </div>
      )}

      <div className="space-y-3 border-t border-gray-100 pt-4">
        <h3 className="text-[19px] font-bold text-gray-900">
          {t("careOfferings.requestsTitle")}
        </h3>

        {pending && (
          <div className="rounded-xl border border-gray-200 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <StatusBadge
                label={t("careOfferings.pendingBadge")}
                tone="orange"
              />
              <p className="text-[15px] text-gray-500">
                {t("careOfferings.requestedAt", {
                  date: formatDate(pending.created_at),
                })}
              </p>
            </div>
            <p className="mt-2 text-[17px] font-medium text-gray-900">
              {t("careOfferings.pendingEffectiveFrom", {
                date: formatDate(pending.effective_from),
              })}
            </p>
            {pending.diff.length > 0 && (
              <dl className="mt-2 space-y-1">
                {pending.diff.map((line) => (
                  <div
                    key={line.label}
                    className="flex flex-wrap items-baseline gap-x-2 text-[15px]"
                  >
                    <dt className="font-medium text-gray-700">{line.label}</dt>
                    <dd className="text-gray-500 line-through">
                      {diffValue(line.old_state, line.old_days)}
                    </dd>
                    <dd className="text-gray-900">
                      {diffValue(line.new_state, line.new_days)}
                    </dd>
                  </div>
                ))}
              </dl>
            )}
            {pending.note && (
              <p className="mt-2 text-[15px] text-gray-500">
                {t("careOfferings.pendingNote", { note: pending.note })}
              </p>
            )}
            <p className="mt-3 text-[15px] text-gray-500">
              {t("careOfferings.pendingNotice")}
            </p>
            {pending.submitted_by_self && (
              <div className="mt-3">
                <Button
                  type="button"
                  variant="outline_danger"
                  size="touch"
                  onClick={() => {
                    setWithdrawError(null);
                    setConfirmWithdraw(true);
                  }}
                >
                  {t("careOfferings.withdraw")}
                </Button>
                {withdrawError && (
                  <p className="text-parent-red-strong mt-1 text-[15px]">
                    {withdrawError}
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {!pending && decision && (
          <div className="rounded-xl border border-gray-200 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              {decision.status === "approved" ? (
                <StatusBadge
                  label={t("careOfferings.decisionApproved")}
                  tone="green"
                />
              ) : (
                <StatusBadge
                  label={t("careOfferings.decisionRejected")}
                  tone="red"
                />
              )}
              <p className="text-[15px] text-gray-500">
                {t("careOfferings.decisionDecidedAt", {
                  date: formatDate(decision.decided_at),
                })}
              </p>
            </div>
            {decision.status === "approved" && (
              <p className="mt-2 text-[17px] font-medium text-gray-900">
                {t("careOfferings.decisionApprovedFrom", {
                  date: formatDate(decision.effective_from),
                })}
              </p>
            )}
            {decision.reason && (
              <p className="mt-2 text-[15px] text-gray-600">
                {t("careOfferings.decisionReason", { reason: decision.reason })}
              </p>
            )}
            {decision.requested.length > 0 && (
              <div className="mt-3">
                <p className="text-[15px] text-gray-500">
                  {t("careOfferings.decisionRequested")}
                </p>
                <ul className="mt-1 space-y-0.5">
                  {decision.requested.map((item) => (
                    <li key={item.id} className="text-[15px] text-gray-700">
                      <span className="font-medium text-gray-900">
                        {item.name}
                      </span>
                      {" · "}
                      {weekdayList(item.weekdays)}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {!pending && !decision && offerings.can_request && (
          <p className="text-[15px] text-gray-600">
            {t("careOfferings.noRequests")}
          </p>
        )}

        {!pending && offerings.can_request && (
          <Button
            type="button"
            variant="outline"
            size="touch"
            onClick={() => setModalOpen(true)}
          >
            {t("careOfferings.requestButton")}
          </Button>
        )}

        {!pending &&
          !offerings.can_request &&
          offerings.changes_disabled_reason &&
          offerings.changes_disabled_reason !== "no_permission" && (
            <p className="text-[15px] text-gray-500">
              {t(DISABLED_REASON_KEYS[offerings.changes_disabled_reason])}
            </p>
          )}
      </div>

      {modalOpen && (
        <OfferingChangeRequestModal
          studentId={studentId}
          onClose={() => setModalOpen(false)}
          onSubmit={async (input) => {
            setOfferings(await submitOfferingChangeRequest(studentId, input));
          }}
        />
      )}

      <ConfirmationModal
        isOpen={confirmWithdraw}
        onClose={() => setConfirmWithdraw(false)}
        onConfirm={() => void handleWithdraw()}
        title={t("careOfferings.withdrawConfirmTitle")}
        confirmText={t("careOfferings.withdraw")}
        cancelText={t("back")}
        isConfirmLoading={withdrawing}
        confirmButtonClass="bg-parent-red hover:bg-parent-red-strong"
      >
        <p className="text-[15px] text-gray-600">
          {t("careOfferings.withdrawConfirmBody")}
        </p>
      </ConfirmationModal>
    </ParentSection>
  );
}

/** Eine Angabe des Wochenplans. Anzeige, nie ein Feld. */
function CareFact({
  label,
  value,
}: Readonly<{ label: string; value?: string }>) {
  return (
    <div>
      <span className="block text-[15px] text-gray-500">{label}</span>
      <span className="block text-[17px] font-medium text-gray-900">
        {value || EMPTY}
      </span>
    </div>
  );
}
