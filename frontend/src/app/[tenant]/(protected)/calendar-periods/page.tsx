"use client";

import { CalendarPeriodsEditor } from "~/components/planning/calendar-periods-editor";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { Loading } from "~/components/ui/loading";
import { DesktopOnlyNotice } from "~/components/ui/desktop-only-notice";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";

/**
 * Rendered when the tenant setting `timetable.enabled` resolves to false.
 * Die Sidebar blendet den ganzen Planung-Bereich bereits aus; dieser Guard
 * deckt den Direktaufruf ab, ohne umzuleiten — gleiche Struktur wie die
 * DisabledStates von Betreuungsplan, Vertretung und Dienstplan.
 */
function CalendarPeriodsDisabledState() {
  return (
    <PlanningDisabledState
      pageTitle="Kalenderzeiträume"
      heading="Kalenderzeiträume sind deaktiviert"
      description="Die Kalenderzeiträume gehören zum Planungsbereich, der für diese Schule ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“ wieder aktiviert werden."
      testId="calendar-periods-disabled-state"
    />
  );
}

export default function CalendarPeriodsPage() {
  const { isReady } = useRequireAdmin();
  // Route-Gate wie die übrigen Planungsseiten: Settings-Schema ->
  // timetable.enabled; fetchSettingsSchema liefert null ohne Leserecht, die
  // Seite rendert dann normal (gleiche Graceful-Default-Logik wie die Sidebar).
  const { data: settingsSchema, isLoading: settingsSchemaLoading } =
    useSettingsSchema(isReady, {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    });
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;

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
