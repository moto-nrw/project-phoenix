"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";

// Weekday numbers match the backend (ISO: Monday=1 .. Friday=5). Labels are
// localized per-locale via t(`request.weekday.${num}`).
const REQUEST_WEEKDAYS = [1, 2, 3, 4, 5] as const;

// Empty value = "leave this weekday's departure mode unchanged". Labels are
// localized per-locale via t(`request.careMode.${key}`). Only the modes a parent
// may pick here are listed — days whose current mode is outside this set (e.g.
// "accompanied") prefill as unchanged so the parent can't accidentally collapse
// them.
const REQUEST_CARE_MODES = [
  { value: "", key: "unchanged" },
  { value: "alone", key: "alone" },
  { value: "bus", key: "bus" },
  { value: "pickup", key: "pickup" },
] as const;

interface CareWeekdayDraft {
  mode: string;
  arrival: string;
  pickup: string;
}

/**
 * The current-plan values a weekday row is prefilled from. Submitting sends only
 * the aspects that DIFFER from these — otherwise every submit would re-request
 * the whole (unchanged) plan.
 */
export type CareScheduleInitialValues = Record<
  number,
  Readonly<CareWeekdayDraft>
>;

export interface CareScheduleRequestCapabilities {
  readonly arrival: boolean;
  readonly pickup: boolean;
  readonly departure_mode: boolean;
}

const EMPTY_DRAFT: CareWeekdayDraft = { mode: "", arrival: "", pickup: "" };

// RequestModalFooter renders the shared cancel/submit buttons using the kit
// Button (so disabled/loading states and brand styling come from the kit). It is
// passed to the Modal's `footer` slot — a sticky bar OUTSIDE the scrollable
// content — so "Anfrage senden" stays visible on short viewports.
function RequestModalFooter({
  submitting,
  onCancel,
  onSubmit,
}: Readonly<{
  submitting: boolean;
  onCancel: () => void;
  onSubmit: () => void;
}>) {
  const t = useTranslations("parentChildCare");
  return (
    <>
      <Button type="button" variant="outline" size="md" onClick={onCancel}>
        {t("cancel")}
      </Button>
      <Button
        type="button"
        size="md"
        className="gap-2"
        onClick={onSubmit}
        disabled={submitting}
      >
        {submitting && (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        )}
        {t("request.submit")}
      </Button>
    </>
  );
}

/**
 * The parent -> OGS "change the weekly care schedule permanently" request form.
 * With `initialValues` the per-day rows start from the current plan (fixes the
 * "Blindflug" of a blank form), and only rows whose values DIFFER from the
 * initial ones are submitted. Without `initialValues` it behaves as before (all
 * rows blank; every filled field is a change).
 *
 * Semantics for a field, mirroring the backend "empty string = unchanged"
 * contract: a field is sent only when it is non-empty AND differs from the
 * initial value. Clearing a prefilled field therefore leaves it unchanged (the
 * API cannot express "remove this time" here).
 */
export function CareScheduleRequestModal({
  onClose,
  onSubmit,
  initialValues,
  capabilities,
}: Readonly<{
  onClose: () => void;
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
  initialValues?: CareScheduleInitialValues;
  capabilities: CareScheduleRequestCapabilities;
}>) {
  const t = useTranslations("parentChildCare");
  const [rows, setRows] = useState<Record<number, CareWeekdayDraft>>(() =>
    Object.fromEntries(
      REQUEST_WEEKDAYS.map((num) => [
        num,
        { ...(initialValues?.[num] ?? EMPTY_DRAFT) },
      ]),
    ),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const careModeOptions = REQUEST_CARE_MODES.map((m) => ({
    value: m.value,
    label: t(`request.careMode.${m.key}`),
  }));

  const setField = (
    num: number,
    field: keyof CareWeekdayDraft,
    value: string,
  ) =>
    setRows((prev) => ({ ...prev, [num]: { ...prev[num]!, [field]: value } }));

  const handleSubmit = async () => {
    const weekdays = REQUEST_WEEKDAYS.flatMap((num) => {
      const row = rows[num]!;
      const base = initialValues?.[num] ?? EMPTY_DRAFT;
      const entry: {
        weekday: number;
        mode?: string;
        arrival?: string;
        pickup?: string;
      } = { weekday: num };
      // Only non-empty values that differ from the current plan count as a
      // change (empty string = unchanged per the backend contract).
      if (capabilities.departure_mode && row.mode && row.mode !== base.mode)
        entry.mode = row.mode;
      if (capabilities.arrival && row.arrival && row.arrival !== base.arrival)
        entry.arrival = row.arrival;
      if (capabilities.pickup && row.pickup && row.pickup !== base.pickup)
        entry.pickup = row.pickup;
      const changed =
        entry.mode !== undefined ||
        entry.arrival !== undefined ||
        entry.pickup !== undefined;
      return changed ? [entry] : [];
    });
    if (weekdays.length === 0) {
      setError(t("request.careSchedule.noChange"));
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({ weekdays });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("request.sendError"));
    } finally {
      setSubmitting(false);
    }
  };

  const timeClass =
    "w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none";

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("request.careSchedule.title")}
      footer={
        <RequestModalFooter
          submitting={submitting}
          onCancel={onClose}
          onSubmit={() => void handleSubmit()}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {t("request.careSchedule.intro")}
        </p>
        <div className="space-y-3">
          {REQUEST_WEEKDAYS.map((num) => {
            const row = rows[num]!;
            const weekdayLabel = t(`request.weekday.${num}`);
            return (
              <div key={num} className="rounded-xl border border-gray-200 p-3">
                <p className="mb-2 text-sm font-semibold text-gray-900">
                  {weekdayLabel}
                </p>
                <div className="space-y-2">
                  {capabilities.departure_mode && (
                    <CustomSelect
                      value={row.mode}
                      options={careModeOptions}
                      onChange={(v) => setField(num, "mode", v)}
                      ariaLabel={t("request.careSchedule.modeAria", {
                        day: weekdayLabel,
                      })}
                    />
                  )}
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    {capabilities.arrival && (
                      <label className="block">
                        <span className="mb-1 block text-xs font-medium text-gray-500">
                          {t("request.careSchedule.arrival")}
                        </span>
                        <input
                          type="time"
                          value={row.arrival}
                          onChange={(e) =>
                            setField(num, "arrival", e.target.value)
                          }
                          className={timeClass}
                        />
                      </label>
                    )}
                    {capabilities.pickup && (
                      <label className="block">
                        <span className="mb-1 block text-xs font-medium text-gray-500">
                          {t("request.careSchedule.pickup")}
                        </span>
                        <input
                          type="time"
                          value={row.pickup}
                          onChange={(e) =>
                            setField(num, "pickup", e.target.value)
                          }
                          className={timeClass}
                        />
                      </label>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}
