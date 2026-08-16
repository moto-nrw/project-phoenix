"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { CircleNotchIcon, InfoIcon } from "@phosphor-icons/react/ssr";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { Modal } from "~/components/ui/modal";
import { TimeField } from "~/components/ui/time-field";
import { ParentApiError, type ChildCareSchedule } from "~/lib/parent-api";

const WEEKDAYS = [1, 2, 3, 4, 5] as const;
const REQUESTABLE_MODES = ["alone", "bus", "pickup"] as const;

type Weekday = (typeof WEEKDAYS)[number];

interface Draft {
  readonly scheduled: boolean;
  readonly mode: string;
  readonly pickup: string;
}

export function PickupScheduleRequestModal({
  schedule,
  onClose,
  onSubmit,
}: Readonly<{
  schedule: ChildCareSchedule;
  onClose: () => void;
  onSubmit: (payload: Record<string, unknown>) => Promise<void>;
}>) {
  const t = useTranslations("parentMasterData");
  const canChangeCareDays =
    schedule.request_capabilities.pickup &&
    schedule.request_capabilities.departure_mode;
  const byWeekday = useMemo(
    () => new Map(schedule.weekdays.map((entry) => [entry.weekday, entry])),
    [schedule.weekdays],
  );
  const initial = useMemo<Record<Weekday, Draft>>(
    () =>
      Object.fromEntries(
        WEEKDAYS.map((weekday) => {
          const current = byWeekday.get(weekday);
          const mode =
            current?.modes.length === 1 &&
            REQUESTABLE_MODES.includes(
              current.modes[0] as (typeof REQUESTABLE_MODES)[number],
            )
              ? current.modes[0]!
              : "";
          return [
            weekday,
            {
              scheduled: current?.status === "scheduled",
              mode,
              pickup: current?.pickup ?? "",
            },
          ];
        }),
      ) as Record<Weekday, Draft>,
    [byWeekday],
  );
  const [rows, setRows] = useState(initial);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const update = (
    weekday: Weekday,
    field: keyof Draft,
    value: string | boolean,
  ) => {
    setRows((current) => ({
      ...current,
      [weekday]: { ...current[weekday], [field]: value },
    }));
    setError(null);
  };

  const handleSubmit = async () => {
    const missingPlan = WEEKDAYS.some(
      (weekday) =>
        rows[weekday].scheduled &&
        !initial[weekday].scheduled &&
        (!rows[weekday].mode || !rows[weekday].pickup),
    );
    if (missingPlan) {
      setError(t("careSchedule.requestMissingPlan"));
      return;
    }
    const weekdays = WEEKDAYS.flatMap((weekday) => {
      const current = rows[weekday];
      const base = initial[weekday];
      const change: {
        weekday: number;
        scheduled?: boolean;
        mode?: string;
        pickup?: string;
      } = {
        weekday,
      };
      if (current.scheduled !== base.scheduled) {
        change.scheduled = current.scheduled;
      }
      if (current.scheduled && !base.scheduled) {
        change.mode = current.mode;
        change.pickup = current.pickup;
      }
      if (!current.scheduled) {
        return change.scheduled === undefined ? [] : [change];
      }
      if (
        schedule.request_capabilities.departure_mode &&
        current.mode &&
        current.mode !== base.mode
      ) {
        change.mode = current.mode;
      }
      if (
        schedule.request_capabilities.pickup &&
        current.pickup &&
        current.pickup !== base.pickup
      ) {
        change.pickup = current.pickup;
      }
      return change.scheduled !== undefined ||
        change.mode !== undefined ||
        change.pickup !== undefined
        ? [change]
        : [];
    });
    if (weekdays.length === 0) {
      setError(t("careSchedule.requestNoChange"));
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit({ weekdays });
      onClose();
    } catch (err) {
      setError(resolveError(err));
    } finally {
      setSubmitting(false);
    }
  };
  const resolveError = (err: unknown) => {
    if (err instanceof ParentApiError) {
      switch (err.code) {
        case "care_request_already_pending":
          return t("careSchedule.requestAlreadyPending");
        case "care_request_field_disabled":
          return t("careSchedule.requestDisabled");
        case "invalid_request_payload":
          return t("careSchedule.requestInvalid");
      }
    }
    return t("careSchedule.requestError");
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("careSchedule.requestTitle")}
      closeLabel={t("careSchedule.close")}
      backdropLabel={t("careSchedule.close")}
      isDismissDisabled={submitting}
      mobileSheet
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="hidden sm:inline-flex"
            disabled={submitting}
            onClick={onClose}
          >
            {t("careSchedule.cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            className="w-full gap-2 sm:w-auto"
            disabled={submitting}
            onClick={() => void handleSubmit()}
          >
            {submitting && (
              <CircleNotchIcon
                weight="bold"
                className="size-4 animate-spin"
                aria-hidden="true"
              />
            )}
            {submitting
              ? t("careSchedule.requestSubmitting")
              : t("careSchedule.requestSubmit")}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {t("careSchedule.requestIntro")}
        </p>
        <div className="bg-moto-blue-soft flex items-start gap-2.5 rounded-xl p-3">
          <InfoIcon
            size={20}
            weight="bold"
            className="text-moto-blue-strong mt-0.5 shrink-0"
            aria-hidden="true"
          />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-gray-900">
              {t("careSchedule.requestNoticeTitle")}
            </p>
            <p className="mt-1 text-sm leading-6 text-gray-700">
              {t("careSchedule.requestNoticeBody")}
            </p>
          </div>
        </div>
        <div className="space-y-2.5">
          {WEEKDAYS.map((weekday) => {
            const label = t(`weekdayShort.${weekday}`);
            const currentModes = byWeekday.get(weekday)?.modes ?? [];
            const modeOptions = [
              {
                value: "",
                label:
                  currentModes.length > 0
                    ? t("careSchedule.modeCurrent", {
                        modes: currentModes
                          .map((mode) => t(`departureModes.${mode}`))
                          .join(", "),
                      })
                    : t("careSchedule.modeUnchanged"),
              },
              ...REQUESTABLE_MODES.map((mode) => ({
                value: mode,
                label: t(`departureModes.${mode}`),
              })),
            ];
            return (
              <fieldset
                key={weekday}
                className="rounded-xl border border-gray-200 p-3"
              >
                <legend className="px-1 text-sm font-semibold text-gray-900">
                  {label}
                </legend>
                <label className="mb-3 flex min-h-10 cursor-pointer items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm font-medium text-gray-900 has-[:disabled]:cursor-default has-[:disabled]:opacity-70">
                  <Checkbox
                    checked={rows[weekday].scheduled}
                    disabled={!canChangeCareDays}
                    onChange={(event) =>
                      update(weekday, "scheduled", event.target.checked)
                    }
                  />
                  {t("careSchedule.careDay")}
                </label>
                {rows[weekday].scheduled && (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    {schedule.request_capabilities.departure_mode && (
                      <div className="min-w-0">
                        <p
                          id={`pickup-mode-label-${weekday}`}
                          className="mb-1 text-sm font-medium text-gray-700"
                        >
                          {t("careSchedule.modes")}
                        </p>
                        <CustomSelect
                          value={rows[weekday].mode}
                          options={modeOptions}
                          onChange={(value) => update(weekday, "mode", value)}
                          ariaLabelledBy={`pickup-mode-label-${weekday}`}
                        />
                      </div>
                    )}
                    {schedule.request_capabilities.pickup && (
                      <TimeField
                        value={rows[weekday].pickup}
                        onChange={(value) => update(weekday, "pickup", value)}
                        label={t("careSchedule.pickup")}
                        hint={t("careSchedule.timeHint")}
                        placeholder={t("careSchedule.timeExample")}
                      />
                    )}
                  </div>
                )}
              </fieldset>
            );
          })}
        </div>
        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}
