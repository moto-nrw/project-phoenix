"use client";

import type { SWRConfiguration } from "swr";

import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
  type SettingsSchema,
} from "~/lib/settings-api";
import { useSWRAuth } from "~/lib/swr";

/** Reads the settings schema from the active tenant's isolated SWR cache. */
export function useSettingsSchema(
  enabled = true,
  options?: SWRConfiguration<SettingsSchema | null, Error>,
) {
  return useSWRAuth<SettingsSchema | null>(
    enabled ? SETTINGS_SCHEMA_SWR_KEY : null,
    fetchSettingsSchema,
    options,
  );
}
