"use client";

import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";
import { CareScheduleRequestModal } from "~/components/parent/care-schedule-request-modal";
import { RequestSharingControl } from "~/components/parent/request-sharing-control";
import {
  ParentSection,
  ParentSubsection,
} from "~/components/parent/shell/parent-section";
import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { RequestDiffEntry } from "~/lib/messaging-status";
import {
  submitCareScheduleRequest,
  type ChildCareSchedule,
} from "~/lib/parent-api";

const WEEKDAYS = [1, 2, 3, 4, 5] as const;
const DAY_KEYS: Record<number, "mon" | "tue" | "wed" | "thu" | "fri"> = {
  1: "mon",
  2: "tue",
  3: "wed",
  4: "thu",
  5: "fri",
};
type PendingRequest = NonNullable<ChildCareSchedule["pending_request"]>;

interface ScheduleProps {
  studentId: string;
  schedule: ChildCareSchedule;
  childFirstName: string;
  careEnded: boolean;
  onScheduleChange: (schedule: ChildCareSchedule) => void;
}

export function WeeklyScheduleSection(props: Readonly<ScheduleProps>) {
  const t = useTranslations("parentMasterData.careSchedule");
  const tc = useTranslations("parentChild");
  const [requestOpen, setRequestOpen] = useState(false);
  const { schedule, studentId, childFirstName, careEnded } = props;
  const showAction =
    !careEnded && schedule.can_request && !schedule.pending_request;
  // No action and no pending request: say why. Booking-led schools point to
  // the offerings section; otherwise this guardian lacks the request
  // permission (the school enabled at least one field).
  const schoolAllowsRequests =
    schedule.request_capabilities.pickup ||
    schedule.request_capabilities.departure_mode;
  const noticeKey =
    !careEnded && !schedule.can_request && !schedule.pending_request
      ? schoolAllowsRequests
        ? "noPermissionNotice"
        : "readOnlyNotice"
      : null;
  return (
    <>
      <ParentSection
        title={tc("care.weekTitle")}
        concept="calendar"
        prominent
        actions={
          showAction ? (
            <Button
              type="button"
              variant="surface"
              size="md"
              onClick={() => setRequestOpen(true)}
            >
              {t("requestButton")}
            </Button>
          ) : undefined
        }
      >
        <ScheduleDays schedule={schedule} childFirstName={childFirstName} />
        {noticeKey && <p className="text-sm text-gray-600">{t(noticeKey)}</p>}
        {schedule.pending_request && (
          <PendingRequestPanel
            studentId={studentId}
            pending={schedule.pending_request}
          />
        )}
      </ParentSection>
      {requestOpen && (
        <CareScheduleRequestModal
          weekdays={schedule.weekdays}
          capabilities={schedule.request_capabilities}
          onClose={() => setRequestOpen(false)}
          onSubmit={async (payload) =>
            props.onScheduleChange(
              await submitCareScheduleRequest(studentId, payload),
            )
          }
        />
      )}
    </>
  );
}

function ScheduleDays({
  schedule,
  childFirstName,
}: Readonly<{
  schedule: ChildCareSchedule;
  childFirstName: string;
}>) {
  const days = new Map(schedule.weekdays.map((day) => [day.weekday, day]));
  return (
    <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-5">
      {WEEKDAYS.map((weekday) => (
        <ScheduleDay
          key={weekday}
          weekday={weekday}
          day={days.get(weekday)}
          childFirstName={childFirstName}
        />
      ))}
    </dl>
  );
}

function ScheduleDay({
  weekday,
  day,
  childFirstName,
}: Readonly<{
  weekday: number;
  day?: ChildCareSchedule["weekdays"][number];
  childFirstName: string;
}>) {
  const t = useTranslations("parentMasterData");
  const scheduled = day?.status === "scheduled";
  const status = scheduled
    ? t("careSchedule.inCare", { firstName: childFirstName })
    : day?.status === "not_scheduled"
      ? t("careSchedule.notInCare")
      : t("careSchedule.unknownCareDay");
  const modes =
    scheduled && day.modes.length > 0
      ? day.modes.map((mode) => t(`departureModes.${mode}`)).join(", ")
      : t("careSchedule.notSet");
  return (
    <div className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <dt className="flex flex-col items-start gap-3 text-base font-semibold text-gray-900">
        <span>{t(`careSchedule.weekdays.${DAY_KEYS[weekday]}`)}</span>
        <StatusBadge
          label={status}
          tone={scheduled ? "green" : "gray"}
          showDot={false}
        />
      </dt>
      <dd className="mt-4 space-y-4">
        {scheduled && (
          <>
            <CareFact
              label={t("careSchedule.pickup")}
              value={
                day.pickup
                  ? t("careSchedule.timeValue", { time: day.pickup })
                  : undefined
              }
              emptyLabel={t("careSchedule.notSet")}
            />
            <CareFact
              label={t("careSchedule.modes")}
              value={modes}
              emptyLabel={t("careSchedule.notSet")}
            />
          </>
        )}
      </dd>
    </div>
  );
}

function PendingRequestPanel({
  studentId,
  pending,
}: Readonly<{
  studentId: string;
  pending: PendingRequest;
}>) {
  const t = useTranslations("parentMasterData.careSchedule");
  const locale = useLocale();
  return (
    <ParentSubsection
      title={t("requestTitle")}
      actions={<StatusBadge label={t("pendingBadge")} tone="orange" />}
    >
      <p className="text-sm text-gray-500">
        {t("requestedAt", {
          date: formatDate(pending.created_at, false, locale),
        })}
      </p>
      {pending.diff.length > 0 && (
        <dl className="space-y-2">
          {pending.diff.map((entry, index) => (
            <CareScheduleDiff key={`${entry.label}-${index}`} entry={entry} />
          ))}
        </dl>
      )}
      <p className="text-sm text-gray-500">{t("pendingNotice")}</p>
      <RequestSharingControl
        studentId={studentId}
        requestType="care_schedule"
        requestId={pending.id}
        isSelf={pending.submitted_by_self}
      />
    </ParentSubsection>
  );
}

function CareScheduleDiff({ entry }: Readonly<{ entry: RequestDiffEntry }>) {
  const t = useTranslations("parentMasterData");
  const day = entry.weekday ? DAY_KEYS[entry.weekday] : undefined;
  const kind =
    entry.care_kind === "pickup"
      ? "pickup"
      : entry.care_kind === "departure_mode"
        ? "modes"
        : entry.care_kind === "arrival"
          ? "arrival"
          : undefined;
  const label =
    day && kind
      ? `${t(`careSchedule.weekdays.${day}`)} · ${t(`careSchedule.${kind}`)}`
      : entry.label;
  const oldValue = entry.old_modes
    ? entry.old_modes.map((mode) => t(`departureModes.${mode}`)).join(", ")
    : entry.old;
  const newValue = entry.new_mode
    ? t(`departureModes.${entry.new_mode}`)
    : entry.new;
  return (
    <div className="text-sm">
      <dt className="font-medium text-gray-900">{label}</dt>
      <dd className="text-gray-600">
        {oldValue} → {newValue}
      </dd>
    </div>
  );
}

function CareFact({
  label,
  value,
  emptyLabel,
}: Readonly<{
  label: string;
  value?: string;
  emptyLabel: string;
}>) {
  return (
    <div>
      <span className="block text-sm text-gray-500">{label}</span>
      <span className="block text-sm font-medium text-gray-900">
        {value || emptyLabel}
      </span>
    </div>
  );
}
