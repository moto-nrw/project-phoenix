"use client";

import useSWR from "swr";

import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
  type ResolvedSetting,
} from "~/lib/settings-api";

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

function findSetting(
  schema: Awaited<ReturnType<typeof fetchSettingsSchema>>,
  key: string,
): ResolvedSetting | undefined {
  if (!schema) return undefined;
  for (const tab of schema.tabs) {
    for (const cat of tab.categories) {
      for (const item of cat.items) {
        if (item.key === key) return item;
      }
    }
  }
  return undefined;
}

export function useTimetableDayHours(): {
  dayStartHour: number;
  dayEndHour: number;
} {
  const { data } = useSWR(SETTINGS_SCHEMA_SWR_KEY, fetchSettingsSchema, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
  });

  const startSetting = findSetting(data ?? null, "timetable.day_start_time");
  const endSetting = findSetting(data ?? null, "timetable.day_end_time");

  const dayStartHour = parseHour(startSetting?.value, DEFAULT_START_HOUR);
  const dayEndHourRaw = parseHour(endSetting?.value, DEFAULT_END_HOUR);

  const dayEndHour =
    dayEndHourRaw <= dayStartHour ? DEFAULT_END_HOUR : dayEndHourRaw;

  return { dayStartHour, dayEndHour };
}
