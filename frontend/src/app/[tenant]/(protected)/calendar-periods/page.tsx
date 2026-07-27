"use client";

import { CalendarPeriodsEditor } from "~/components/planning/calendar-periods-editor";
import { ClosingDaysEditor } from "~/components/planning/closing-days-editor";
import { Loading } from "~/components/ui/loading";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

/**
 * Bewusst KEIN timetable.enabled-Route-Gate wie auf den übrigen
 * Planungsseiten: Kalenderzeiträume sind eine geteilte Ressource — die
 * Anmeldephasen (Anmeldungen-Bereich) verknüpfen sich mit ihnen, auch wenn
 * der Planungsbereich für die Schule abgeschaltet ist.
 *
 * Ebenso bewusst KEIN DesktopOnlyNotice wie im Anmeldungsbereich (#2033):
 * beide Editoren sind Tabelle plus Modal und damit mobil bedienbar. Bei
 * abgeschaltetem Planungsbereich ist dies zudem der einzige verbleibende
 * Eintrag der Planungsgruppe — eine Desktop-Sperre wäre dort eine Sackgasse.
 */
export default function CalendarPeriodsPage() {
  const { isReady } = useRequireAdmin();

  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full space-y-8">
      <CalendarPeriodsEditor />
      <section>
        <h2 className="mb-3 text-base font-semibold text-gray-900">
          Schließtage
        </h2>
        <ClosingDaysEditor />
      </section>
    </div>
  );
}
