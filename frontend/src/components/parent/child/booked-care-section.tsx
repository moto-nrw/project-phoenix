"use client";

import { useCallback, useEffect, useState } from "react";
import { CalendarCheckIcon, CalendarXIcon } from "@phosphor-icons/react/ssr";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { OfferingChangeRequestModal } from "~/components/parent/offering-change-request-modal";
import { RequestSharingControl } from "~/components/parent/request-sharing-control";
import { WeeklyScheduleSection } from "~/components/parent/child/weekly-schedule-section";
import {
  ParentSection,
  ParentSubsection,
} from "~/components/parent/shell/parent-section";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  getChildCareOfferings,
  getChildCareSchedule,
  submitOfferingChangeRequest,
  type ChildCareOfferings,
  type ChildCareSchedule,
  type OfferingChangesDisabledReason,
} from "~/lib/parent-api";

const logger = createLogger({ component: "BookedCareSection" });

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
 * "Betreuung in der OGS": was gebucht ist, an welchen Tagen und wann das Kind
 * abgeholt wird. Der Block "AGs und Gruppen" entfaellt ersatzlos (#2303), die
 * Rubrik "Betreuungszeiten" ebenso (#2302).
 */
export function BookedCareSection({
  studentId,
  childFirstName,
  careEnded,
  enrolledUntil,
  reasonRequired = true,
}: Readonly<{
  studentId: string;
  childFirstName: string;
  careEnded: boolean;
  enrolledUntil?: string;
  /** Ob die OGS einen Grund verlangt (requiresGuardianReason). */
  reasonRequired?: boolean;
}>) {
  const t = useTranslations("parentMasterData");
  const tc = useTranslations("parentChild");
  const locale = useLocale();
  const [offerings, setOfferings] = useState<ChildCareOfferings | null>(null);
  const [schedule, setSchedule] = useState<ChildCareSchedule | null>(null);
  const [loading, setLoading] = useState(true);
  const [offeringsError, setOfferingsError] = useState(false);
  const [scheduleError, setScheduleError] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editRequest, setEditRequest] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setOfferingsError(false);
    setScheduleError(false);
    const [offeringsResult, scheduleResult] = await Promise.allSettled([
      getChildCareOfferings(studentId),
      getChildCareSchedule(studentId),
    ]);
    if (offeringsResult.status === "fulfilled") {
      setOfferings(offeringsResult.value);
    } else {
      setOfferings(null);
      setOfferingsError(true);
      logger.warn("booked_care_offerings_load_failed", {
        error: String(offeringsResult.reason),
        student_id: studentId,
      });
    }
    if (scheduleResult.status === "fulfilled") {
      setSchedule(scheduleResult.value);
    } else {
      setSchedule(null);
      setScheduleError(true);
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

  const periodLabel =
    offerings?.period_name && offerings.period_start && offerings.period_end
      ? t("careOfferings.period", {
          name: offerings.period_name,
          from: formatDate(offerings.period_start, false, locale),
          until: formatDate(offerings.period_end, false, locale),
        })
      : null;

  if (loading) {
    return (
      <div className="flex flex-col gap-5">
        <ParentSectionSkeleton rows={5} />
        <ParentSectionSkeleton rows={2} />
      </div>
    );
  }

  // Eine Kursanfrage gehört in den Abschnitt "Kurse" (#3075). Stünde sie
  // auch hier, sähe die Familie denselben Vorgang zweimal, in zwei
  // Formulierungen und mit zwei verschiedenen Knöpfen.
  const pendingRequest = offerings?.pending_request;
  const pending =
    pendingRequest &&
    pendingRequest.diff.length > 0 &&
    pendingRequest.diff.every((line) => line.is_course === true)
      ? undefined
      : pendingRequest;
  const decision = offerings?.last_decision;
  const hasScheduledCare =
    schedule?.weekdays.some((weekday) => weekday.status === "scheduled") ??
    false;
  const showDisabledNotice =
    offerings !== null &&
    offerings.offerings.length > 0 &&
    pending === undefined &&
    decision === undefined &&
    !offerings.can_request &&
    offerings.changes_disabled_reason !== undefined &&
    offerings.changes_disabled_reason !== "no_permission" &&
    offerings.changes_disabled_reason !== "no_enrollment";

  return (
    <div className="flex flex-col gap-5">
      {schedule ? (
        <WeeklyScheduleSection
          studentId={studentId}
          schedule={schedule}
          childFirstName={childFirstName}
          careEnded={careEnded}
          onScheduleChange={setSchedule}
        />
      ) : scheduleError ? (
        <ParentSection
          title={tc("care.weekTitle")}
          concept="calendar"
          prominent
        >
          <Alert type="error" message={t("careSchedule.loadError")} />
        </ParentSection>
      ) : null}

      {offerings ? (
        <ParentSection
          title={t("careOfferings.offeringsTitle")}
          description={
            periodLabel ??
            t("careOfferings.offeringsDescription", { name: childFirstName })
          }
          concept="carePlan"
          prominent
          actions={
            !careEnded && !pending && offerings.can_request ? (
              <Button
                type="button"
                variant="surface"
                size="md"
                onClick={() => setModalOpen(true)}
              >
                {t("careOfferings.requestButton")}
              </Button>
            ) : undefined
          }
        >
          {offerings.offerings.length === 0 ? (
            <EmptyState
              className="border-t border-gray-100"
              icon={
                hasScheduledCare ? (
                  <CalendarCheckIcon className="size-10" />
                ) : (
                  <CalendarXIcon className="size-10" />
                )
              }
              title={
                hasScheduledCare
                  ? t("careOfferings.scheduleOnlyTitle")
                  : offerings.changes_disabled_reason === "no_enrollment"
                    ? t("careOfferings.disabledNoEnrollment", {
                        name: childFirstName,
                      })
                    : t("careOfferings.noOfferings", { name: childFirstName })
              }
              description={
                hasScheduledCare
                  ? t("careOfferings.scheduleOnlyDescription", {
                      name: childFirstName,
                    })
                  : undefined
              }
            />
          ) : (
            <ul className="space-y-2">
              {offerings.offerings.map((offering) => (
                <li
                  key={`${offering.id}-${offering.valid_from ?? "current"}`}
                  className="rounded-xl border border-gray-200 p-3"
                >
                  <p className="text-sm font-semibold text-gray-900">
                    {offering.name}
                  </p>
                  <p className="mt-0.5 text-sm text-gray-700">
                    {weekdayList(offering.weekdays)}
                  </p>
                  <p className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-sm text-gray-500">
                    {offering.starts_later && offering.valid_from && (
                      <span>
                        {t("careOfferings.startsLater", {
                          date: formatDate(offering.valid_from, false, locale),
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
                          date: formatDate(offering.valid_until, false, locale),
                        })}
                      </span>
                    )}
                  </p>
                </li>
              ))}
            </ul>
          )}
          {pending && (
            <ParentSubsection
              title={t("careOfferings.requestsTitle")}
              actions={
                <StatusBadge
                  label={t("careOfferings.pendingBadge")}
                  tone="orange"
                />
              }
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm font-medium text-gray-900">
                  {t("careOfferings.pendingEffectiveFrom", {
                    date: formatDate(pending.effective_from, false, locale),
                  })}
                </p>
                <p className="text-sm text-gray-500">
                  {t("careOfferings.requestedAt", {
                    date: formatDate(pending.created_at, false, locale),
                  })}
                </p>
              </div>
              {pending.diff.length > 0 && (
                <dl className="space-y-1">
                  {pending.diff.map((line) => (
                    <div
                      key={line.label}
                      className="flex flex-wrap items-baseline gap-x-2 text-sm"
                    >
                      <dt className="font-medium text-gray-700">
                        {line.label}
                      </dt>
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
                <p className="text-sm text-gray-500">
                  {t("careOfferings.pendingNote", { note: pending.note })}
                </p>
              )}
              <p className="text-sm text-gray-500">
                {t("careOfferings.pendingNotice")}
              </p>
              <RequestSharingControl
                studentId={studentId}
                requestType="offering"
                requestId={pending.id}
                isSelf={pending.submitted_by_self}
              />
              {!careEnded && pending.submitted_by_self && (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  className="max-sm:min-h-11"
                  onClick={() => setEditRequest(pending.id)}
                >
                  {t("careOfferings.edit")}
                </Button>
              )}
            </ParentSubsection>
          )}

          {!pending && decision && (
            <ParentSubsection
              title={t("careOfferings.requestsTitle")}
              actions={
                decision.status === "approved" ? (
                  <StatusBadge
                    label={
                      decision.complete_withdrawal
                        ? careEnded && enrolledUntil
                          ? t("careOfferings.withdrawalCareEnded", {
                              date: formatDate(enrolledUntil, false, locale),
                            })
                          : t("careOfferings.withdrawalApproved")
                        : t("careOfferings.decisionApproved")
                    }
                    tone="green"
                  />
                ) : (
                  <StatusBadge
                    label={t("careOfferings.decisionRejected")}
                    tone="red"
                  />
                )
              }
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm text-gray-500">
                  {t("careOfferings.decisionDecidedAt", {
                    date: formatDate(decision.decided_at, false, locale),
                  })}
                </p>
              </div>
              {decision.status === "approved" &&
                decision.complete_withdrawal &&
                !careEnded && (
                  <p className="text-sm font-medium text-gray-900">
                    {t("careOfferings.withdrawalApprovedHint")}
                  </p>
                )}
              {decision.status === "approved" &&
                !decision.complete_withdrawal && (
                  <p className="text-sm font-medium text-gray-900">
                    {t("careOfferings.decisionApprovedFrom", {
                      date: formatDate(decision.effective_from, false, locale),
                    })}
                  </p>
                )}
              {decision.reason && (
                <p className="text-sm text-gray-600">
                  {t("careOfferings.decisionReason", {
                    reason: decision.reason,
                  })}
                </p>
              )}
              {decision.requested.length > 0 && (
                <div>
                  <p className="text-sm text-gray-500">
                    {t("careOfferings.decisionRequested")}
                  </p>
                  <ul className="mt-1 space-y-0.5">
                    {decision.requested.map((item) => (
                      <li key={item.id} className="text-sm text-gray-700">
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
              <RequestSharingControl
                studentId={studentId}
                requestType="offering"
                requestId={decision.id}
                isSelf={decision.submitted_by_self === true}
              />
            </ParentSubsection>
          )}

          {showDisabledNotice && offerings.changes_disabled_reason && (
            <p className="text-sm text-gray-500">
              {t(DISABLED_REASON_KEYS[offerings.changes_disabled_reason])}
            </p>
          )}
        </ParentSection>
      ) : offeringsError ? (
        <ParentSection
          title={t("careOfferings.offeringsTitle")}
          concept="carePlan"
          prominent
        >
          <Alert type="error" message={t("careOfferings.loadError")} />
        </ParentSection>
      ) : null}

      {modalOpen && (
        <OfferingChangeRequestModal
          studentId={studentId}
          childName={childFirstName}
          reasonRequired={reasonRequired}
          onClose={() => setModalOpen(false)}
          onSubmit={async (input, recipientIds) => {
            const next = await submitOfferingChangeRequest(
              studentId,
              input,
              recipientIds,
            );
            setOfferings(next);
            return next.pending_request?.id;
          }}
        />
      )}

      {editRequest && (
        <OfferingChangeRequestModal
          studentId={studentId}
          childName={childFirstName}
          reasonRequired={reasonRequired}
          editRequestId={editRequest}
          onClose={() => setEditRequest(null)}
          onEdited={refresh}
        />
      )}
    </div>
  );
}
