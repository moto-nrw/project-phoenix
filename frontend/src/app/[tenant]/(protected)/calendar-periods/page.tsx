"use client";

import { CalendarPeriodsEditor } from "~/components/planning/calendar-periods-editor";
import { ClosingDaysEditor } from "~/components/planning/closing-days-editor";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SkeletonRegion, TableSkeleton } from "~/components/ui/page-skeletons";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

/**
 * Bewusst KEIN timetable.enabled-Route-Gate wie auf den übrigen
 * Planungsseiten: Kalenderzeiträume sind eine geteilte Ressource, die
 * Anmeldephasen (Anmeldungen-Bereich) verknüpfen sich mit ihnen, auch wenn
 * der Planungsbereich für die Schule abgeschaltet ist.
 *
 * Ebenso bewusst KEIN DesktopOnlyNotice wie im Anmeldungsbereich (#2033):
 * beide Editoren sind Tabelle plus Modal und damit mobil bedienbar. Bei
 * abgeschaltetem Planungsbereich ist dies zudem der einzige verbleibende
 * Eintrag der Planungsgruppe, eine Desktop-Sperre wäre dort eine Sackgasse.
 */
export default function CalendarPeriodsPage() {
  const { isReady } = useRequireAdmin();

  return (
    <div className="-mt-1.5 w-full space-y-6">
      {/* Der Kopf rendert immer sofort, nur die Datenregion skeletonisiert. */}
      <PageHeaderWithSearch title="Kalenderzeiträume" />
      {isReady ? (
        <>
          <CalendarPeriodsEditor />
          <section className="space-y-3">
            <h2 className="text-base font-semibold text-gray-900">
              Schließtage
            </h2>
            <ClosingDaysEditor />
          </section>
        </>
      ) : (
        <SkeletonRegion
          label="Kalenderzeiträume werden geladen"
          testId="loading"
          className="w-full space-y-6"
        >
          <TableSkeleton rows={5} columns={3} />
          <TableSkeleton rows={4} columns={3} />
        </SkeletonRegion>
      )}
    </div>
  );
}
