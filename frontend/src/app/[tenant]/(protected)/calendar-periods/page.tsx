"use client";

import {
  CalendarPeriodsActions,
  CalendarPeriodsEditor,
  useCalendarPeriods,
} from "~/components/planning/calendar-periods-editor";
import { ClosingDaysEditor } from "~/components/planning/closing-days-editor";
import { TenantPage } from "~/components/ui/tenant-page";
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

  // Solange die Rechteprüfung läuft, steht hier das Seitengerüst mit seinem
  // Ladezustand; die Zeiträume werden erst danach abgerufen.
  if (!isReady) {
    return <TenantPage title="Zeiträume" loading testId="loading" />;
  }

  return <CalendarPeriodsPageContent />;
}

/**
 * Der Kopf gehört der Seite: Titel, Statuszeile und die beiden
 * Anlegen-Aktionen. Leer- und Fehlerzustand bleiben in den beiden
 * Inhaltsblöcken, damit die Schließtage auch dann erreichbar sind, wenn die
 * Zeiträume leer sind oder nicht geladen werden konnten.
 */
function CalendarPeriodsPageContent() {
  const state = useCalendarPeriods();

  return (
    <TenantPage
      title="Zeiträume"
      stats={state.statusLine}
      statsLoading={state.loading}
      actions={<CalendarPeriodsActions state={state} />}
    >
      <CalendarPeriodsEditor state={state} />
      <ClosingDaysEditor />
    </TenantPage>
  );
}
