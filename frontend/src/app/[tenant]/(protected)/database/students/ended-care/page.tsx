"use client";

// Beendete Betreuungen (#2487): jedes Kind, dessen Betreuung regulär zu Ende
// gegangen ist — von Hand beendet, durch ein Anmeldungsende oder später über
// den geführten Abschluss. Jahrgangs-Abgänge stehen weiterhin ausschließlich
// in der Abgänge-Ansicht des Jahrgangswechsels.
//
// Die Ansicht ist geschützt wie die endgültige Löschung: nur wer Kinder
// löschen darf, sieht sie — und nur dort stehen Austrittsgrund und Freitext.
//
// Design follows the Anmeldungen/Planung surface language: calm content
// section, uppercase kicker, gray-50 stats, no colored dashboards.

import { useMemo, useState } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";

import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { CareResumeModal } from "~/components/students/care-resume-modal";
import { StudentDeletionModal } from "~/components/students/student-deletion-modal";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import {
  CARE_EXIT_REASON_LABELS,
  fetchEndedCare,
  type EndedCareEntry,
} from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const SWR_KEY = "students-ended-care";

function reasonLabel(entry: EndedCareEntry): string {
  if (!entry.reason) return "Kein Grund hinterlegt";
  const label = CARE_EXIT_REASON_LABELS[entry.reason];
  if (entry.reason === "other" && entry.reasonNote) {
    return `${label}: ${entry.reasonNote}`;
  }
  return label;
}

export default function EndedCarePage() {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const canManage = hasPermission(session, "users:delete");

  const [search, setSearch] = useState("");
  const [resumeTarget, setResumeTarget] = useState<EndedCareEntry | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<EndedCareEntry | null>(null);
  const { success: toastSuccess } = useToast();
  const tenantMutate = useTenantMutate();

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth(canManage ? SWR_KEY : null, () =>
    fetchEndedCare({ pageSize: 200 }),
  );

  const entries = useMemo(() => data?.items ?? [], [data]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return entries;
    return entries.filter((entry) =>
      `${entry.firstName} ${entry.lastName} ${entry.schoolClass}`
        .toLowerCase()
        .includes(needle),
    );
  }, [entries, search]);

  const withReason = useMemo(
    () => entries.filter((entry) => entry.reason !== null).length,
    [entries],
  );

  const columns: DataTableColumn<EndedCareEntry>[] = [
    {
      key: "name",
      header: "Name",
      render: (entry) => (
        <span className="font-medium text-gray-900">
          {entry.lastName}, {entry.firstName}
        </span>
      ),
      sortValue: (entry) =>
        `${entry.lastName.toLowerCase()} ${entry.firstName.toLowerCase()}`,
    },
    {
      key: "schoolClass",
      header: "Klasse",
      render: (entry) => (
        <span className="text-gray-700">{entry.schoolClass || "—"}</span>
      ),
      sortValue: (entry) => entry.schoolClass.toLowerCase(),
    },
    {
      key: "lastCareDay",
      header: "Letzter Betreuungstag",
      render: (entry) => (
        <span className="text-gray-700">{formatDate(entry.lastCareDay)}</span>
      ),
      sortValue: (entry) => entry.lastCareDay,
    },
    {
      key: "reason",
      header: "Grund",
      render: (entry) => (
        <span className="text-gray-600">{reasonLabel(entry)}</span>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      render: (entry) => (
        <div className="flex items-center justify-end gap-1">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => setResumeTarget(entry)}
          >
            Wieder aufnehmen
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="text-moto-red-strong"
            onClick={() => setDeleteTarget(entry)}
          >
            Endgültig löschen
          </Button>
        </div>
      ),
    },
  ];

  if (status === "loading") {
    return <Loading />;
  }

  if (!canManage) {
    return (
      <div className="w-full space-y-4">
        <BackButton referrer="/database/students" />
        <Alert
          type="info"
          message="Diese Ansicht ist nur für Personen mit der Berechtigung „Benutzer löschen“."
        />
      </div>
    );
  }

  return (
    <div className="w-full space-y-4">
      <BackButton referrer="/database/students" />

      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p
              className="text-xs font-semibold tracking-wide uppercase"
              style={{ color: LOCATION_COLORS.OTHER_ROOM }}
            >
              Beendete Betreuungen
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Kinder, die nicht mehr in der OGS sind
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Die Daten dieser Kinder bleiben erhalten. Sie stehen in keiner
              normalen Liste mehr und in keinem Export. Abgänge aus dem
              Jahrgangswechsel stehen weiterhin dort.
            </p>
          </div>
          <div className="w-full lg:w-64">
            <Input
              value={search}
              placeholder="Name oder Klasse suchen…"
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-2 sm:max-w-md">
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {entries.length}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Kinder gesamt
            </span>
          </div>
          <div className="rounded-xl bg-gray-50 px-3 py-2">
            <span className="block text-sm font-semibold text-gray-900">
              {withReason}
            </span>
            <span className="block text-[11px] font-medium text-gray-500">
              Mit Grund
            </span>
          </div>
        </div>

        {loadError ? (
          <div className="mt-4">
            <Alert
              type="error"
              message="Die beendeten Betreuungen konnten nicht geladen werden."
            />
          </div>
        ) : null}

        <div className="mt-4">
          {isLoading ? (
            <Loading />
          ) : filtered.length === 0 ? (
            <EmptyState
              title={
                search
                  ? "Kein Kind gefunden"
                  : "Noch keine beendeten Betreuungen"
              }
              description={
                search
                  ? "Versuchen Sie einen anderen Namen oder eine andere Klasse."
                  : "Hier stehen Kinder, deren Betreuung beendet wurde."
              }
            />
          ) : (
            <DataTable
              rows={filtered}
              columns={columns}
              getRowKey={(entry) => entry.studentId}
              defaultSortKey="lastCareDay"
              defaultSortDirection="desc"
            />
          )}
        </div>
      </section>

      {resumeTarget ? (
        <CareResumeModal
          isOpen
          studentId={resumeTarget.studentId}
          displayName={`${resumeTarget.firstName} ${resumeTarget.lastName}`}
          onClose={() => setResumeTarget(null)}
          onResumed={async () => {
            toastSuccess(
              `Betreuung von ${resumeTarget.firstName} ${resumeTarget.lastName} wieder aufgenommen`,
            );
            setResumeTarget(null);
            await tenantMutate(SWR_KEY);
          }}
        />
      ) : null}

      {deleteTarget ? (
        <StudentDeletionModal
          isOpen
          studentId={deleteTarget.studentId}
          displayName={`${deleteTarget.firstName} ${deleteTarget.lastName}`}
          careEnded
          onClose={() => setDeleteTarget(null)}
          onDeleted={async () => {
            toastSuccess(
              `${deleteTarget.firstName} ${deleteTarget.lastName} wurde gelöscht`,
            );
            setDeleteTarget(null);
            await tenantMutate(SWR_KEY);
          }}
        />
      ) : null}
    </div>
  );
}
