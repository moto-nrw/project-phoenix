"use client";

// Startseite des Träger-Portals ("moto träger", #4649): alle Schulen des
// Trägers der Sitzung. Schulen ordnet das moto-Team zu. Wer das Recht „In OGS
// wechseln" hat, öffnet eine Schule ohne neue Anmeldung in einem neuen Tab
// (#4654).

import { useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { School } from "lucide-react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import {
  startTraegerSchoolSession,
  type TraegerSchool,
} from "~/lib/traeger-api";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TRAEGER_HANDOFF_OPEN, traegerHandoffUrl } from "~/lib/traeger-handoff";

const logger = createLogger({ component: "TraegerSchoolsPage" });

/** The Träger right „In OGS wechseln" (ADR 0084, #4654). */
const SCHOOL_SWITCH_RIGHT = "traeger_school:switch";

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

/**
 * Opens the school in a new tab: the tab opens on the click, so the browser
 * does not block it, and receives the school's address once the hand-off
 * token is there. Without a tab the school opens here.
 */
async function openSchool(school: TraegerSchool): Promise<void> {
  const tab = globalThis.open("", "_blank");
  try {
    const url = traegerHandoffUrl(
      school.subdomain,
      await startTraegerSchoolSession(school.id),
    );
    if (tab) {
      tab.opener = null;
      tab.location.href = url;
    } else {
      globalThis.location.assign(url);
    }
  } catch (err) {
    tab?.close();
    throw err;
  }
}

export default function TraegerSchoolsPage() {
  const { data: session } = useSession();
  const { schools: schoolData, error: schoolsError } = useTraegerSchools();
  const schools = schoolData ?? [];
  const loading = schoolData === null && !schoolsError;
  const [opening, setOpening] = useState<string | null>(null);
  const openErrors = useApiErrorDisplay();
  const canOpen = hasPermission(session, SCHOOL_SWITCH_RIGHT);

  const open = useCallback(
    async (school: TraegerSchool) => {
      setOpening(school.id);
      try {
        await openSchool(school);
      } catch (err) {
        logger.error("traeger_school_open_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void openErrors.show(err, { object: "die Schule" });
      } finally {
        setOpening(null);
      }
    },
    [openErrors],
  );

  const columns = useMemo<DataTableColumn<TraegerSchool>[]>(
    () =>
      canOpen
        ? [
            // On a phone the button takes the place beside the name; the
            // city moves under it.
            ...COLUMNS.map((column) =>
              column.key === "city"
                ? { ...column, stacked: "field" as const }
                : column,
            ),
            {
              key: "actions",
              header: "",
              align: "right",
              stacked: "meta",
              stackedLabel: false,
              render: (school) => (
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  aria-label={`${school.name} öffnen`}
                  disabled={opening !== null}
                  onClick={() => void open(school)}
                >
                  {opening === school.id
                    ? "Wird geöffnet …"
                    : TRAEGER_HANDOFF_OPEN}
                </Button>
              ),
            },
          ]
        : COLUMNS,
    [canOpen, open, opening],
  );

  return (
    <TenantPage
      title="Schulen"
      info={
        <>
          Alle Schulen Ihres Trägers. Neue Schulen ordnet das moto-Team zu.
          {canOpen &&
            " „Öffnen“ zeigt die Schule in einem neuen Tab. Dort arbeiten Sie mit der Rolle Verwaltung."}
        </>
      }
      stats={
        schools.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: schools.length,
                label: schools.length === 1 ? "Schule" : "Schulen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={loading}
      error={schoolsError}
    >
      <DataTable
        columns={columns}
        rows={schools}
        getRowKey={(school) => school.id}
        isLoading={loading}
        loadingRowCount={3}
        defaultSortKey="name"
        stackedOnMobile
        stackedLayout="row"
        stackedDetailKey={canOpen ? "city" : null}
        emptyState={
          <EmptyState
            icon={<School className="h-12 w-12" aria-hidden="true" />}
            title="Ihrem Träger sind noch keine Schulen zugeordnet."
            description="Das moto-Team ordnet die Schulen Ihrem Träger zu."
          />
        }
      />
    </TenantPage>
  );
}
