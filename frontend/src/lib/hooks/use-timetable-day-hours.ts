"use client";

import { getSettingValue } from "~/lib/settings-api";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";

const DEFAULT_END_HOUR = 17;
const DEFAULT_START_TIME = "09:00";
const DEFAULT_END_TIME = "17:00";

function parseTime(value: unknown, fallback: string): string {
  if (typeof value !== "string") return fallback;
  const match = /^(\d{1,2}):(\d{2})$/.exec(value);
  if (!match) return fallback;
  const h = Number.parseInt(match[1]!, 10);
  const minutes = Number.parseInt(match[2]!, 10);
  if (!Number.isFinite(h) || h < 0 || h > 23 || minutes < 0 || minutes > 59) {
    return fallback;
  }
  return `${String(h).padStart(2, "0")}:${String(minutes).padStart(2, "0")}`;
}

function toHour(time: string): number {
  return Number.parseInt(time, 10);
}

function toMinutes(time: string): number {
  const [hours = "0", minutes = "0"] = time.split(":");
  return Number(hours) * 60 + Number(minutes);
}

export function useTimetableDayHours(): {
  dayStartHour: number;
  dayEndHour: number;
  dayStartTime: string;
  dayEndTime: string;
} {
  const { data } = useSettingsSchema(true, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
  });

  const dayStartTimeRaw = parseTime(
    getSettingValue(data, "timetable.day_start_time"),
    DEFAULT_START_TIME,
  );
  const dayEndTimeRaw = parseTime(
    getSettingValue(data, "timetable.day_end_time"),
    DEFAULT_END_TIME,
  );
  const dayStartMinutes = toMinutes(dayStartTimeRaw);
  const dayEndMinutes = toMinutes(dayEndTimeRaw);
  const hasValidDayRange = dayEndMinutes > dayStartMinutes;
  const dayStartTime = hasValidDayRange ? dayStartTimeRaw : DEFAULT_START_TIME;
  const dayEndTime = hasValidDayRange ? dayEndTimeRaw : DEFAULT_END_TIME;
  const dayStartHour = toHour(dayStartTime);
  const dayEndHour = hasValidDayRange
    ? Math.ceil(dayEndMinutes / 60)
    : DEFAULT_END_HOUR;

  return { dayStartHour, dayEndHour, dayStartTime, dayEndTime };
}
