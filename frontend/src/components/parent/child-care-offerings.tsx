"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { CalendarClock, CheckCircle2, Clock, XCircle } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { Section } from "~/components/parent/child-detail-section";
import { OfferingChangeRequestModal } from "~/components/parent/offering-change-request-modal";
import {
  type ChildCareOfferings,
  type OfferingChangesDisabledReason,
  getChildCareOfferings,
  submitOfferingChangeRequest,
  withdrawOfferingChangeRequest,
} from "~/lib/parent-api";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildCareOfferings" });

// Reason identifier from the backend → the message key explaining it.
const DISABLED_REASON_KEYS: Record<OfferingChangesDisabledReason, string> = {
  no_enrollment: "careOfferings.disabledNoEnrollment",
  no_permission: "careOfferings.disabledNoPermission",
  school_disabled: "careOfferings.disabledSchoolOff",
  period_over: "careOfferings.disabledPeriodOver",
};

/**
 * The child's booked care offerings and activity groups, plus the
 * post-enrollment change-request flow (#1665). Sits on the Stammdaten page next
 * to the weekly care times, which is where a guardian already goes to change
 * something permanent.
 */
export function ChildCareOfferingsSection({
  studentId,
}: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentMasterData");
  const [data, setData] = useState<ChildCareOfferings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [confirmWithdraw, setConfirmWithdraw] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const [withdrawError, setWithdrawError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      setData(await getChildCareOfferings(studentId));
    } catch (err) {
      logger.warn("child_care_offerings_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const weekdayList = useCallback(
    (weekdays: number[]) =>
      weekdays.length === 0
        ? t("careOfferings.allDays")
        : weekdays.map((day) => t(`weekdayShort.${day}`)).join(", "),
    [t],
  );

  const periodLabel = useMemo(() => {
    if (!data?.period_name || !data.period_start || !data.period_end) {
      return null;
    }
    return t("careOfferings.period", {
      name: data.period_name,
      from: formatDate(data.period_start),
      until: formatDate(data.period_end),
    });
  }, [data, t]);

  if (loading) {
    return (
      <div className="h-48 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
    );
  }

  if (error || !data) {
    return (
      <Section
        title={t("sections.careOfferings")}
        hint={t("careOfferings.hint")}
      >
        <p className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
          {t("careOfferings.loadError")}
        </p>
      </Section>
    );
  }

  const pending = data.pending_request;
  const decision = data.last_decision;

  const handleWithdraw = async () => {
    if (!pending) return;
    setWithdrawing(true);
    setWithdrawError(null);
    try {
      setData(await withdrawOfferingChangeRequest(studentId, pending.id));
      setConfirmWithdraw(false);
    } catch (err) {
      logger.warn("child_care_offerings_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setWithdrawError(t("careOfferings.withdrawError"));
    } finally {
      setWithdrawing(false);
    }
  };

  return (
    <Section title={t("sections.careOfferings")} hint={t("careOfferings.hint")}>
      {periodLabel && <p className="text-xs text-gray-500">{periodLabel}</p>}

      <div className="space-y-2">
        <h3 className="text-xs font-semibold tracking-wide text-gray-500">
          {t("careOfferings.offeringsTitle")}
        </h3>
        {data.offerings.length === 0 ? (
          <p className="text-sm text-gray-600">
            {t("careOfferings.noOfferings")}
          </p>
        ) : (
          <ul className="space-y-2">
            {data.offerings.map((offering) => (
              <li
                key={offering.id}
                className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
              >
                <p className="text-sm font-semibold text-gray-900">
                  {offering.name}
                </p>
                <p className="mt-0.5 text-sm text-gray-700">
                  {weekdayList(offering.weekdays)}
                </p>
                <p className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500">
                  {offering.includes_lunch && (
                    <span>{t("careOfferings.lunch")}</span>
                  )}
                  {offering.includes_holiday_care && (
                    <span>{t("careOfferings.holidayCare")}</span>
                  )}
                  {offering.price_cents !== undefined && (
                    <span>
                      {t("careOfferings.price", {
                        amount: formatEuro(offering.price_cents),
                      })}
                    </span>
                  )}
                </p>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="space-y-2">
        <h3 className="text-xs font-semibold tracking-wide text-gray-500">
          {t("careOfferings.groupsTitle")}
        </h3>
        {data.groups.length === 0 ? (
          <p className="text-sm text-gray-600">{t("careOfferings.noGroups")}</p>
        ) : (
          <ul className="space-y-2">
            {data.groups.map((group) => (
              <li
                key={`${group.id}-${group.valid_from}`}
                className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <p className="text-sm font-semibold text-gray-900">
                    {group.name}
                  </p>
                  {group.starts_later && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#3a63ad]">
                      <CalendarClock className="h-3 w-3" aria-hidden="true" />
                      {t("careOfferings.startsLater", {
                        date: formatDate(group.valid_from),
                      })}
                    </span>
                  )}
                </div>
                <p className="mt-0.5 text-sm text-gray-700">
                  {weekdayList(group.weekdays)}
                </p>
                {group.valid_until && (
                  <p className="mt-1 text-xs text-gray-500">
                    {t("careOfferings.until", {
                      date: formatDate(group.valid_until),
                    })}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Requests get their own labelled block, separated from the read-only
          lists above: what is booked and what was asked for are two different
          questions, and a decided request must not read like a booking. */}
      <div className="space-y-2 border-t border-gray-100 pt-4">
        <h3 className="text-xs font-semibold tracking-wide text-gray-500">
          {t("careOfferings.requestsTitle")}
        </h3>

        {pending && (
          <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="inline-flex items-center gap-1 rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-semibold text-[#92710b]">
                <Clock className="h-3 w-3" aria-hidden="true" />
                {t("careOfferings.pendingBadge")}
              </p>
              <p className="text-xs text-gray-500">
                {t("careOfferings.requestedAt", {
                  date: formatDate(pending.created_at),
                })}
              </p>
            </div>
            <p className="mt-2 text-sm font-medium text-gray-900">
              {t("careOfferings.pendingEffectiveFrom", {
                date: formatDate(pending.effective_from),
              })}
            </p>
            {pending.diff.length > 0 && (
              <dl className="mt-2 space-y-1">
                {pending.diff.map((line) => (
                  <div
                    key={line.label}
                    className="flex flex-wrap items-baseline gap-x-2 text-sm"
                  >
                    <dt className="font-medium text-gray-700">{line.label}</dt>
                    <dd className="text-gray-500 line-through">{line.old}</dd>
                    <dd className="text-gray-900">{line.new}</dd>
                  </div>
                ))}
              </dl>
            )}
            {pending.note && (
              <p className="mt-2 text-xs text-gray-500">
                {t("careOfferings.pendingNote", { note: pending.note })}
              </p>
            )}
            <p className="mt-3 text-xs text-gray-500">
              {t("careOfferings.pendingNotice")}
            </p>
            {pending.submitted_by_self && (
              <div className="mt-3">
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  onClick={() => {
                    setWithdrawError(null);
                    setConfirmWithdraw(true);
                  }}
                >
                  {t("careOfferings.withdraw")}
                </Button>
                {withdrawError && (
                  <p className="mt-1 text-sm text-[#CC2626]">{withdrawError}</p>
                )}
              </div>
            )}
          </div>
        )}

        {/* The outcome of a decided request, so a guardian learns it here and
            not only in the chat. No diff: after an approval was applied the
            comparison is empty and would read as "nothing changed". */}
        {!pending && decision && (
          <div className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
            <div className="flex flex-wrap items-center justify-between gap-2">
              {decision.status === "approved" ? (
                <p className="inline-flex items-center gap-1 rounded-full bg-[#83CD2D]/15 px-2 py-0.5 text-xs font-semibold text-[#4f7d18]">
                  <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
                  {t("careOfferings.decisionApproved")}
                </p>
              ) : (
                <p className="inline-flex items-center gap-1 rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-xs font-semibold text-[#CC2626]">
                  <XCircle className="h-3 w-3" aria-hidden="true" />
                  {t("careOfferings.decisionRejected")}
                </p>
              )}
              <p className="text-xs text-gray-500">
                {t("careOfferings.decisionDecidedAt", {
                  date: formatDate(decision.decided_at),
                })}
              </p>
            </div>
            {decision.status === "approved" && (
              <p className="mt-2 text-sm font-medium text-gray-900">
                {t("careOfferings.decisionApprovedFrom", {
                  date: formatDate(decision.effective_from),
                })}
              </p>
            )}
            {decision.reason && (
              <p className="mt-2 text-sm text-gray-600">
                {t("careOfferings.decisionReason", { reason: decision.reason })}
              </p>
            )}
          </div>
        )}

        {!pending && !decision && data.can_request && (
          <p className="text-sm text-gray-600">
            {t("careOfferings.noRequests")}
          </p>
        )}

        {!pending && data.can_request && (
          <div>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => setModalOpen(true)}
            >
              {t("careOfferings.requestButton")}
            </Button>
          </div>
        )}

        {/* A missing enrollment or a school that keeps changes offline is worth
            saying out loud: silently hiding the button reads as a broken app.
            The no-permission case stays silent — a guardian who may not request
            changes does not need to be reminded of it on every visit. */}
        {!pending &&
          !data.can_request &&
          data.changes_disabled_reason &&
          data.changes_disabled_reason !== "no_permission" && (
            <p className="text-xs text-gray-500">
              {t(DISABLED_REASON_KEYS[data.changes_disabled_reason])}
            </p>
          )}
      </div>

      {modalOpen && (
        <OfferingChangeRequestModal
          studentId={studentId}
          onClose={() => setModalOpen(false)}
          onSubmit={async (input) => {
            setData(await submitOfferingChangeRequest(studentId, input));
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
        confirmButtonClass="bg-[#FF3130] hover:bg-[#CC2626]"
      >
        <p className="text-sm text-gray-600">
          {t("careOfferings.withdrawConfirmBody")}
        </p>
      </ConfirmationModal>
    </Section>
  );
}

/** Cents → "12,50 €" without pulling in a formatter for one label. */
function formatEuro(cents: number): string {
  const euros = (cents / 100).toFixed(2).replace(".", ",");
  return `${euros} €`;
}
