"use client";

import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";

/**
 * Ob die Person Elternmitteilungen schreiben kann. Dieselbe Bedingung wie der
 * Eintrag „Mitteilungen" in der Seitenleiste: jede Route unter
 * /api/parent-announcements verlangt admin:*, und die Schule muss
 * `operations.parent_news_enabled` eingeschaltet haben. So führt kein Knopf
 * auf eine Seite, die der Person fehlt.
 */
export function useParentAnnouncementsAvailable(): boolean {
  const { data: session } = useSession();
  const canAnnounce = hasPermission(session, "admin:*");
  const { data: settingsSchema } = useSettingsSchema(canAnnounce, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });
  return (
    canAnnounce &&
    getSettingValue(settingsSchema, "operations.parent_news_enabled") === true
  );
}
