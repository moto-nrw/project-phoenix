"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { CalendarClock, Clock } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { ConfirmationModal } from "~/components/ui/modal";
import { RequestDiffPanel } from "~/components/messaging/request-diff-panel";
import { Section } from "~/components/parent/child-detail-section";
import {
  CareScheduleRequestModal,
  type CareScheduleInitialValues,
} from "~/components/parent/care-schedule-request-modal";
import {
  type ChildCareSchedule,
  getChildCareSchedule,
  submitCareScheduleRequest,
  withdrawCareScheduleRequest,
} from "~/lib/parent-api";
import {
  PARENT_DEPARTURE_MODE_I18N_KEYS,
  PARENT_DIFF_CARE_KIND_I18N_KEYS,
  type RequestDiffEntry,
} from "~/lib/messaging-status";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ChildCareSchedule" });

// Placeholder for an unset time / empty mode list (matches the backend diff's
// own "—" convention and the pickup modal). Not a Gedankenstrich in prose.
const EMPTY = "—";

// ISO weekday (Monday=1 .. Friday=5) → the parentMasterData.departureDays key.
const WEEKDAY_DAY_KEYS: Record<number, string> = {
  1: "mon",
  2: "tue",
  3: "wed",
  4: "thu",
  5: "fri",
};
const WEEKDAYS = [1, 2, 3, 4, 5] as const;

// The departure modes a parent may pick in the request modal. A day whose single
// current mode is outside this set (e.g. "accompanied") is prefilled as
// "unchanged" so the parent can't accidentally collapse it to one of these.
const REQUESTABLE_MODES = new Set(["alone", "bus", "pickup"]);

export function ChildCareScheduleSection({
  studentId,
}: Readonly<{ studentId: string }>) {
  const t = useTranslations("parentMasterData");
  const tMsg = useTranslations("parentOgsMessaging");
  const [data, setData] = useState<ChildCareSchedule | null>(null);
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
      setData(await getChildCareSchedule(studentId));
    } catch (err) {
      logger.warn("child_care_schedule_load_failed", {
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

  const byWeekday = useMemo(() => {
    const map = new Map<number, ChildCareSchedule["weekdays"][number]>();
    for (const w of data?.weekdays ?? []) map.set(w.weekday, w);
    return map;
  }, [data]);

  // Prefill the request modal from the current plan: arrival/pickup from the
  // current times; the mode only when the day has exactly ONE requestable mode
  // (a multi-mode day prefills "" so a submit doesn't collapse it).
  const initialValues = useMemo<CareScheduleInitialValues>(() => {
    const out: CareScheduleInitialValues = {};
    for (const num of WEEKDAYS) {
      const w = byWeekday.get(num);
      const singleMode =
        w && w.modes.length === 1 && REQUESTABLE_MODES.has(w.modes[0]!)
          ? w.modes[0]!
          : "";
      out[num] = {
        mode: singleMode,
        arrival: w?.arrival ?? "",
        pickup: w?.pickup ?? "",
      };
    }
    return out;
  }, [byWeekday]);

  // Re-derive each diff row's label / mode strings from its structured fields so
  // they render in the guardian's language (the backend `label`/`old`/`new` are
  // German). Mirrors ogs-conversation.RequestItem.
  const localizedDiff = useMemo<RequestDiffEntry[] | undefined>(() => {
    const diff = data?.pending_request?.diff;
    if (!diff) return undefined;
    const localizeMode = (key: string): string => {
      const modeKey = PARENT_DEPARTURE_MODE_I18N_KEYS[key];
      return modeKey ? tMsg(modeKey) : key;
    };
    return diff.map((entry) => {
      const careKey = entry.care_kind
        ? PARENT_DIFF_CARE_KIND_I18N_KEYS[entry.care_kind]
        : undefined;
      const label =
        entry.weekday && careKey
          ? `${tMsg(`diffWeekday${entry.weekday}`)} · ${tMsg(careKey)}`
          : entry.label;
      if (entry.care_kind !== "departure_mode") return { ...entry, label };
      return {
        ...entry,
        label,
        old: entry.old_modes?.length
          ? entry.old_modes.map(localizeMode).join(" / ")
          : entry.old,
        new: entry.new_mode ? localizeMode(entry.new_mode) : entry.new,
      };
    });
  }, [data, tMsg]);

  if (loading) {
    return <Skeleton className="h-48 w-full rounded-2xl" />;
  }

  if (error || !data) {
    return (
      <Section
        icon={CalendarClock}
        title={t("sections.careSchedule")}
        hint={t("careSchedule.hint")}
      >
        <Alert type="error" message={t("careSchedule.loadError")} />
      </Section>
    );
  }

  const pending = data.pending_request;

  const handleWithdraw = async () => {
    if (!pending) return;
    setWithdrawing(true);
    setWithdrawError(null);
    try {
      setData(await withdrawCareScheduleRequest(studentId, pending.id));
      setConfirmWithdraw(false);
    } catch (err) {
      logger.warn("child_care_schedule_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setWithdrawError(t("careSchedule.withdrawError"));
    } finally {
      setWithdrawing(false);
    }
  };

  return (
    <Section
      icon={CalendarClock}
      title={t("sections.careSchedule")}
      hint={t("careSchedule.hint")}
      actions={
        data.can_request && !pending ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setModalOpen(true)}
          >
            {t("requestButton")}
          </Button>
        ) : undefined
      }
    >
      <dl className="grid grid-cols-2 gap-2 sm:grid-cols-5">
        {WEEKDAYS.map((num) => {
          const w = byWeekday.get(num);
          const modes =
            w && w.modes.length > 0
              ? w.modes.map((m) => t(`departureModes.${m}`)).join(", ")
              : EMPTY;
          return (
            <div key={num} className="rounded-xl bg-gray-50 p-3">
              <dt className="text-[11px] font-medium tracking-wide text-gray-500 uppercase">
                {t(`departureDays.${WEEKDAY_DAY_KEYS[num]}`)}
              </dt>
              <dd className="mt-2 space-y-1 text-sm">
                <CareRow label={t("careSchedule.arrival")} value={w?.arrival} />
                <CareRow label={t("careSchedule.pickup")} value={w?.pickup} />
                <div>
                  <span className="text-xs text-gray-500">
                    {t("careSchedule.modes")}
                  </span>
                  <p className="font-medium text-gray-900">{modes}</p>
                </div>
              </dd>
            </div>
          );
        })}
      </dl>

      {pending && (
        <div className="rounded-xl bg-gray-50 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="bg-moto-amber/15 text-moto-amber-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold">
              <Clock className="h-3 w-3" aria-hidden="true" />
              {t("careSchedule.pendingBadge")}
            </p>
            <p className="text-xs text-gray-500">
              {t("careSchedule.requestedAt", {
                date: formatDate(pending.created_at),
              })}
            </p>
          </div>
          <RequestDiffPanel
            diff={localizedDiff}
            heading={tMsg("diffHeading")}
          />
          <p className="mt-3 text-xs text-gray-500">
            {t("careSchedule.pendingNotice")}
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
                {t("careSchedule.withdraw")}
              </Button>
              {withdrawError && (
                <p className="text-moto-red-strong mt-1 text-sm">
                  {withdrawError}
                </p>
              )}
            </div>
          )}
        </div>
      )}

      {modalOpen && (
        <CareScheduleRequestModal
          initialValues={initialValues}
          onClose={() => setModalOpen(false)}
          onSubmit={async (payload) => {
            setData(await submitCareScheduleRequest(studentId, payload));
          }}
        />
      )}

      <ConfirmationModal
        isOpen={confirmWithdraw}
        onClose={() => setConfirmWithdraw(false)}
        onConfirm={() => void handleWithdraw()}
        title={t("careSchedule.withdrawConfirmTitle")}
        confirmText={t("careSchedule.withdraw")}
        cancelText={t("back")}
        isConfirmLoading={withdrawing}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-strong"
      >
        <p className="text-sm text-gray-600">
          {t("careSchedule.withdrawConfirmBody")}
        </p>
      </ConfirmationModal>
    </Section>
  );
}

function CareRow({
  label,
  value,
}: Readonly<{ label: string; value?: string }>) {
  return (
    <div>
      <span className="text-xs text-gray-500">{label}</span>
      <p className="font-medium text-gray-900">{value || EMPTY}</p>
    </div>
  );
}
