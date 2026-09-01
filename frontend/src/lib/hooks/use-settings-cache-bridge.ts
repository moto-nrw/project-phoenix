"use client";

import { useEffect } from "react";
import { SETTINGS_SCHEMA_SWR_KEY } from "~/lib/settings-api";
import { subscribeSettingsChanged } from "~/lib/settings-broadcast";
import { useTenantMutate, useTenantMutateMatching } from "~/lib/swr";
import { CHANGE_REQUEST_ACCESS_SWR_KEY } from "./use-change-request-access";

/**
 * Bridges cross-tab/cross-origin settings-change broadcasts to the active
 * tenant's SWR cache entry for SETTINGS_SCHEMA_SWR_KEY. Mount once in the
 * protected layout —
 * every SWR consumer (sidebar, mobile bottom nav, settings page, timetable
 * day hours) then refreshes automatically when:
 *   1. another same-origin tab calls notifySettingsChanged()  → BroadcastChannel
 *   2. backend fires tenant_settings_changed                  → SSE → window event
 *
 * Without this bridge, only the tab that initiated the change saw the new
 * value; other tabs' SWR caches stayed stale until navigation or reload.
 */
export function useSettingsCacheBridge(): void {
  const tenantMutate = useTenantMutate();
  const refreshChangeRequestAccess = useTenantMutateMatching([
    CHANGE_REQUEST_ACCESS_SWR_KEY,
  ]);

  useEffect(() => {
    const invalidate = () => {
      void tenantMutate(SETTINGS_SCHEMA_SWR_KEY);
      void refreshChangeRequestAccess();
    };
    const unsubscribeBroadcast = subscribeSettingsChanged(invalidate);
    if (typeof window !== "undefined") {
      window.addEventListener("phoenix:tenant-settings-stale", invalidate);
    }
    return () => {
      unsubscribeBroadcast();
      if (typeof window !== "undefined") {
        window.removeEventListener("phoenix:tenant-settings-stale", invalidate);
      }
    };
  }, [tenantMutate, refreshChangeRequestAccess]);
}
