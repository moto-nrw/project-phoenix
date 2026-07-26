"use client";

import { getSettingValue } from "~/lib/settings-api";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";

const DEFAULT_START_HOUR = 9;
const DEFAULT_END_HOUR = 17;

function parseHour(value: unknown, fallback: number): number {
  if (typeof value !== "string") return fallback;
  const match = /^(\d{1,2}):/.exec(value);
  if (!match) return fallback;
  const h = Number.parseInt(match[1]!, 10);
  if (!Number.isFinite(h) || h < 0 || h > 23) return fallback;
  return h;
}

export function useTimetableDayHours(): {
  dayStartHour: number;
  dayEndHour: number;
} {
  const { data } = useSettingsSchema(true, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
  });

  const dayStartHour = parseHour(
    getSettingValue(data, "timetable.day_start_time"),
    DEFAULT_START_HOUR,
  );
  const dayEndHourRaw = parseHour(
    getSettingValue(data, "timetable.day_end_time"),
    DEFAULT_END_HOUR,
  );

  const dayEndHour =
    dayEndHourRaw <= dayStartHour ? DEFAULT_END_HOUR : dayEndHourRaw;

  return { dayStartHour, dayEndHour };
}
