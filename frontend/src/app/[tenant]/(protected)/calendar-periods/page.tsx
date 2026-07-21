"use client";

import { CalendarOff } from "lucide-react";

import { CalendarPeriodsEditor } from "~/components/planning/calendar-periods-editor";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import {
  SETTINGS_SCHEMA_SWR_KEY,
  fetchSettingsSchema,
} from "~/lib/settings-api";
import { useSWRAuth } from "~/lib/swr";

/**
 * Rendered when the tenant setting `timetable.enabled` resolves to false.
 * Die Sidebar blendet den ganzen Planung-Bereich bereits aus; dieser Guard
 * deckt den Direktaufruf ab, ohne umzuleiten — gleiche Struktur wie die
 * DisabledStates von Betreuungsplan, Vertretung und Dienstplan.
 */
function CalendarPeriodsDisabledState() {
  return (
    <div
      className="flex flex-col gap-4"
      data-testid="calendar-periods-disabled-state"
    >
      <h1 className="text-lg font-semibold text-gray-900">Kalenderzeiträume</h1>
      <div className="moto-content-surface rounded-2xl border p-10 text-center shadow-sm">
        <CalendarOff className="mx-auto h-10 w-10 text-gray-300" aria-hidden />
        <h2 className="mt-4 text-base font-semibold text-gray-900">
          Kalenderzeiträume sind deaktiviert
        </h2>
        <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-600">
          Die Kalenderzeiträume gehören zum Planungsbereich, der für diese
          Schule ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“
          wieder aktiviert werden.
        </p>
      </div>
    </div>
  );
}

export default function CalendarPeriodsPage() {
  const { isReady } = useRequireAdmin();
  // Route-Gate wie die übrigen Planungsseiten: Settings-Schema ->
  // timetable.enabled; fetchSettingsSchema liefert null ohne Leserecht, die
  // Seite rendert dann normal (gleiche Graceful-Default-Logik wie die Sidebar).
  const { data: settingsSchema, isLoading: settingsSchemaLoading } = useSWRAuth(
    isReady ? SETTINGS_SCHEMA_SWR_KEY : null,
    fetchSettingsSchema,
    { revalidateOnFocus: false, revalidateOnReconnect: false },
  );
  const timetableDisabled =
    settingsSchema?.tabs
      .flatMap((tab) => tab.categories)
      .flatMap((category) => category.items)
      .find((item) => item.key === "timetable.enabled")?.value === false;

  if (!isReady || settingsSchemaLoading) return <Loading fullPage={false} />;
  if (timetableDisabled) return <CalendarPeriodsDisabledState />;

  return (
    <div className="-mt-1.5 w-full">
      <DesktopOnlyNotice />
      <div className="hidden lg:block">
        <CalendarPeriodsEditor />
      </div>
    </div>
  );
}
