"use client";

import { useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { Modal } from "~/components/ui/modal";
import { TimeField } from "~/components/ui/time-field";
import {
  ParentApiError,
  type CareScheduleRequestInput,
  type ChildCareSchedule,
} from "~/lib/parent-api";

const DAY_KEYS: Record<number, "mon" | "tue" | "wed" | "thu" | "fri"> = {
  1: "mon",
  2: "tue",
  3: "wed",
  4: "thu",
  5: "fri",
};

interface DayDraft {
  scheduled: boolean;
  pickup: string;
  mode: string;
}

interface RequestedDay {
  weekday: number;
  scheduled?: boolean;
  pickup?: string;
  mode?: string;
}

interface RequestedDayResult {
  change?: RequestedDay;
  missingPlan: boolean;
}

type Capabilities = ChildCareSchedule["request_capabilities"];
type Weekdays = ChildCareSchedule["weekdays"];
type SubmitRequest = (payload: CareScheduleRequestInput) => Promise<void>;

function requestErrorKey(error: unknown): string {
  if (!(error instanceof ParentApiError)) return "requestError";
  if (error.code === "care_request_already_pending") {
    return "requestAlreadyPending";
  }
  if (error.code === "care_request_field_disabled") return "requestDisabled";
  if (error.status === 400) return "requestInvalid";
  return "requestError";
}

function requestedDay(
  day: Weekdays[number],
  draft: DayDraft,
  capabilities: Capabilities,
): RequestedDayResult {
  const wasScheduled = day.status === "scheduled";
  const canChangeDay = capabilities.pickup && capabilities.departure_mode;
  if (canChangeDay && draft.scheduled !== wasScheduled) {
    if (!draft.scheduled) {
      return {
        change: { weekday: day.weekday, scheduled: false },
        missingPlan: false,
      };
    }
    if (!draft.pickup || !draft.mode) return { missingPlan: true };
    return {
      change: {
        weekday: day.weekday,
        scheduled: true,
        pickup: draft.pickup,
        mode: draft.mode,
      },
      missingPlan: false,
    };
  }
  if (!draft.scheduled) return { missingPlan: false };
  const change: RequestedDay = { weekday: day.weekday };
  if (
    capabilities.pickup &&
    draft.pickup &&
    draft.pickup !== (day.pickup ?? "")
  ) {
    change.pickup = draft.pickup;
  }
  const currentMode = day.modes.length === 1 ? day.modes[0] : "";
  if (capabilities.departure_mode && draft.mode && draft.mode !== currentMode) {
    change.mode = draft.mode;
  }
  return {
    change: Object.keys(change).length > 1 ? change : undefined,
    missingPlan: false,
  };
}

function requestPayload(
  weekdays: Weekdays,
  drafts: Readonly<Record<number, DayDraft>>,
  capabilities: Capabilities,
) {
  const results = weekdays.map((day) =>
    requestedDay(day, drafts[day.weekday]!, capabilities),
  );
  return {
    payload: {
      weekdays: results.flatMap((result) =>
        result.change ? [result.change] : [],
      ),
    },
    missingPlan: results.some((result) => result.missingPlan),
  };
}

function draftsFromWeekdays(weekdays: Weekdays): Record<number, DayDraft> {
  return Object.fromEntries(
    weekdays.map((day) => [
      day.weekday,
      {
        pickup: day.pickup ?? "",
        mode: day.modes.length === 1 ? day.modes[0]! : "",
        scheduled: day.status === "scheduled",
      },
    ]),
  ) as Record<number, DayDraft>;
}

function ModalFooter({
  submitting,
  onClose,
  onSubmit,
}: Readonly<{
  submitting: boolean;
  onClose: () => void;
  onSubmit: () => void;
}>) {
  const t = useTranslations("parentMasterData.careSchedule");
  return (
    <>
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        {t("cancel")}
      </Button>
      <Button type="button" size="md" disabled={submitting} onClick={onSubmit}>
        {submitting && <Loader2 className="size-4 animate-spin" aria-hidden />}
        {t(submitting ? "requestSubmitting" : "requestSubmit")}
      </Button>
    </>
  );
}

function ScheduleDayFields({
  weekday,
  draft,
  capabilities,
  onChange,
}: Readonly<{
  weekday: number;
  draft: DayDraft;
  capabilities: Capabilities;
  onChange: (draft: DayDraft) => void;
}>) {
  const t = useTranslations("parentMasterData");
  const day = t(`careSchedule.weekdays.${DAY_KEYS[weekday]}`);
  const canChangeDay = capabilities.pickup && capabilities.departure_mode;
  return (
    <fieldset className="space-y-3 rounded-xl border border-gray-200 p-4">
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {day}
      </legend>
      {canChangeDay && <CareDayToggle draft={draft} onChange={onChange} />}
      {draft.scheduled && (
        <ScheduleValues
          day={day}
          draft={draft}
          capabilities={capabilities}
          onChange={onChange}
        />
      )}
    </fieldset>
  );
}

function CareDayToggle({
  draft,
  onChange,
}: Readonly<{ draft: DayDraft; onChange: (draft: DayDraft) => void }>) {
  const t = useTranslations("parentMasterData.careSchedule");
  return (
    <label className="flex items-center gap-3 text-sm font-medium text-gray-700">
      <Checkbox
        checked={draft.scheduled}
        onChange={(event) =>
          onChange({ ...draft, scheduled: event.target.checked })
        }
      />
      {t("careDay")}
    </label>
  );
}

function ScheduleValues({
  day,
  draft,
  capabilities,
  onChange,
}: Readonly<{
  day: string;
  draft: DayDraft;
  capabilities: Capabilities;
  onChange: (draft: DayDraft) => void;
}>) {
  const t = useTranslations("parentMasterData");
  const options = ["alone", "bus", "pickup"].map((value) => ({
    value,
    label: t(`departureModes.${value}`),
  }));
  return (
    <>
      {capabilities.departure_mode && (
        <CustomSelect
          value={draft.mode}
          options={options}
          onChange={(mode) => onChange({ ...draft, mode })}
          ariaLabel={t("careSchedule.modeAria", { day })}
          placeholder={t("careSchedule.modeUnchanged")}
        />
      )}
      {capabilities.pickup && (
        <TimeField
          value={draft.pickup}
          onChange={(pickup) => onChange({ ...draft, pickup })}
          label={t("careSchedule.pickup")}
          hint={t("careSchedule.timeHint")}
          placeholder={t("careSchedule.timeExample")}
        />
      )}
    </>
  );
}

function useRequestForm(
  weekdays: Weekdays,
  capabilities: Capabilities,
  onSubmit: SubmitRequest,
  onClose: () => void,
) {
  const t = useTranslations("parentMasterData.careSchedule");
  const [drafts, setDrafts] = useState(() => draftsFromWeekdays(weekdays));
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const submit = async () => {
    const { payload, missingPlan } = requestPayload(
      weekdays,
      drafts,
      capabilities,
    );
    if (missingPlan) return setError(t("requestMissingPlan"));
    if (payload.weekdays.length === 0) return setError(t("requestNoChange"));
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit(payload);
      onClose();
    } catch (submitError) {
      setError(t(requestErrorKey(submitError)));
    } finally {
      setSubmitting(false);
    }
  };
  return { drafts, setDrafts, submitting, error, submit };
}

function ScheduleFields({
  weekdays,
  capabilities,
  drafts,
  setDrafts,
}: Readonly<{
  weekdays: Weekdays;
  capabilities: Capabilities;
  drafts: Record<number, DayDraft>;
  setDrafts: React.Dispatch<React.SetStateAction<Record<number, DayDraft>>>;
}>) {
  const canChangeDay = capabilities.pickup && capabilities.departure_mode;
  return weekdays
    .filter((day) => canChangeDay || day.status === "scheduled")
    .map((day) => (
      <ScheduleDayFields
        key={day.weekday}
        weekday={day.weekday}
        draft={drafts[day.weekday]!}
        capabilities={capabilities}
        onChange={(draft) =>
          setDrafts((current) => ({ ...current, [day.weekday]: draft }))
        }
      />
    ));
}

export function CareScheduleRequestModal({
  weekdays,
  capabilities,
  onClose,
  onSubmit,
}: Readonly<{
  weekdays: Weekdays;
  capabilities: Capabilities;
  onClose: () => void;
  onSubmit: SubmitRequest;
}>) {
  const t = useTranslations("parentMasterData.careSchedule");
  const form = useRequestForm(weekdays, capabilities, onSubmit, onClose);

  return (
    <Modal
      mobileSheet
      isOpen
      onClose={onClose}
      title={t("requestTitle")}
      closeLabel={t("close")}
      isDismissDisabled={form.submitting}
      footer={
        <ModalFooter
          submitting={form.submitting}
          onClose={onClose}
          onSubmit={() => void form.submit()}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">{t("requestIntro")}</p>
        <Alert
          type="info"
          title={t("requestNoticeTitle")}
          message={t("requestNoticeBody")}
        />
        <ScheduleFields
          weekdays={weekdays}
          capabilities={capabilities}
          drafts={form.drafts}
          setDrafts={form.setDrafts}
        />
        {form.error && <Alert type="error" message={form.error} />}
      </div>
    </Modal>
  );
}
