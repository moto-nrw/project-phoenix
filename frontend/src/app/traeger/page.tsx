"use client";

// Startseite des Träger-Portals ("moto träger", #4649): alle Schulen des
// Trägers der Sitzung. Nur zum Lesen; Schulen ordnet das moto-Team zu.

import { useCallback, useEffect, useState } from "react";
import { School } from "lucide-react";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchTraegerSchools, type TraegerSchool } from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerSchoolsPage" });

const COLUMNS: DataTableColumn<TraegerSchool>[] = [
  {
    key: "name",
    header: "Schule",
    render: (school) => (
      <span className="font-medium text-gray-900">{school.name}</span>
    ),
    sortValue: (school) => school.name,
    stacked: "title",
  },
  {
    key: "city",
    header: "Ort",
    render: (school) => school.city || "–",
    sortValue: (school) => school.city || null,
    stacked: "meta",
  },
];

export default function TraegerSchoolsPage() {
  const [schools, setSchools] = useState<TraegerSchool[]>([]);
  const [loading, setLoading] = useState(true);
  const schoolsLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = schoolsLoad;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSchools(await fetchTraegerSchools());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_schools_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setSchools([]);
      await showLoadError(err, {
        object: "die Liste der Schulen",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="w-full space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Schulen
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Alle Schulen Ihres Trägers. Neue Schulen ordnet das moto-Team zu.
        </p>
      </div>

      {schoolsLoad.error ? (
        <LoadErrorAlert error={schoolsLoad.error} />
      ) : (
        <DataTable
          columns={COLUMNS}
          rows={schools}
          getRowKey={(school) => school.id}
          isLoading={loading}
          loadingRowCount={3}
          defaultSortKey="name"
          stackedOnMobile
          stackedLayout="row"
          emptyState={
            <EmptyState
              icon={<School className="h-12 w-12" aria-hidden="true" />}
              title="Ihrem Träger sind noch keine Schulen zugeordnet."
              description="Das moto-Team ordnet die Schulen Ihrem Träger zu."
            />
          }
        />
      )}
    </div>
  );
}
