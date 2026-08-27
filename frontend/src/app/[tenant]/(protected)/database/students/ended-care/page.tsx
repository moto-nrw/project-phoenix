"use client";

// Beendete Betreuungen (#2487): jedes Kind, dessen Betreuung regulär zu Ende
// gegangen ist: von Hand beendet, durch ein Anmeldungsende oder später über
// den geführten Abschluss. Jahrgangs-Abgänge stehen weiterhin ausschließlich
// in der Abgänge-Ansicht des Jahrgangswechsels.
//
// Die Ansicht ist geschützt wie die endgültige Löschung: nur wer Kinder
// löschen darf, sieht sie, und nur dort stehen Austrittsgrund und Freitext.
//
// Design follows the Anmeldungen/Planung surface language: calm content
// section, uppercase kicker, gray-50 stats, no colored dashboards.

import { useEffect, useMemo, useState } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { formatCount } from "~/lib/format-utils";
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
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { useDebounce } from "~/lib/use-debounce";

const ENDED_CARE_DESCRIPTION =
  "Kinder, die nicht mehr in der OGS sind. Die Daten dieser Kinder bleiben erhalten. Sie stehen in keiner normalen Liste mehr und in keinem Export. Abgänge aus dem Jahrgangswechsel stehen weiterhin dort.";

// Der Schlüssel-Stamm. Suche und Seite hängen daran, damit jede Kombination
// ihren eigenen Cache-Eintrag bekommt; aktualisiert wird über den Stamm.
const SWR_KEY = "students-ended-care";
const PAGE_SIZE = 50;

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
  const [page, setPage] = useState(1);
  const [resumeTarget, setResumeTarget] = useState<EndedCareEntry | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<EndedCareEntry | null>(null);
  const { success: toastSuccess } = useToast();
  const refreshEndedCare = useTenantMutateMatching([SWR_KEY]);

  // Gesucht wird auf dem Server, nicht in der geladenen Seite: sonst fände die
  // Suche nur die Kinder, die gerade zufällig sichtbar sind (#2487).
  const debouncedSearch = useDebounce(search.trim(), 300);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  const {
    data,
    isLoading,
    error: loadError,
  } = useSWRAuth(
    canManage ? `${SWR_KEY}:${page}:${debouncedSearch}` : null,
    () =>
      fetchEndedCare({
        page,
        pageSize: PAGE_SIZE,
        search: debouncedSearch || undefined,
      }),
    { keepPreviousData: true },
  );

  const entries = useMemo(() => data?.items ?? [], [data]);
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const firstOnPage = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const lastOnPage = Math.min(page * PAGE_SIZE, total);

  // Eine Seite, die es nicht mehr gibt (letztes Kind der letzten Seite wieder
  // aufgenommen oder gelöscht), fällt auf die letzte vorhandene zurück.
  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

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
        <span className="text-gray-700">{entry.schoolClass || "–"}</span>
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
          {entry.reason && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => setResumeTarget(entry)}
            >
              Wieder aufnehmen
            </Button>
          )}
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

  // Statuszeile des Seitenkopfs aus der bereits geladenen Seite.
  const statusLine = [
    `${formatCount(total)} ${total === 1 ? "Kind" : "Kinder"}`,
    total > 0 ? `${firstOnPage} bis ${lastOnPage} auf dieser Seite` : null,
    debouncedSearch ? `gefiltert nach „${debouncedSearch}“` : null,
    totalPages > 1 ? `Seite ${page} von ${totalPages}` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  const contentEmpty = status !== "loading" && !canManage;

  return (
    <TenantPage
      title="Beendete Betreuungen"
      stats={statusLine}
      statsLoading={status === "loading" || (isLoading && data === undefined)}
      search={
        canManage
          ? {
              value: search,
              onChange: setSearch,
              placeholder: "Name oder Klasse suchen…",
            }
          : undefined
      }
      loading={status === "loading"}
      error={
        canManage && loadError
          ? "Die beendeten Betreuungen konnten nicht geladen werden."
          : null
      }
      back
    >
      {contentEmpty ? (
        <ForbiddenPage message="Diese Ansicht ist nur für Personen mit der Berechtigung „Benutzer löschen“." />
      ) : (
        <SectionCard
          title="Kinder ohne Betreuung"
          description={ENDED_CARE_DESCRIPTION}
        >
          <DataTable
            rows={entries}
            columns={columns}
            getRowKey={(entry) => entry.studentId}
            isLoading={isLoading && data === undefined}
            defaultSortKey="lastCareDay"
            defaultSortDirection="desc"
            emptyState={
              <EmptyState
                title={
                  debouncedSearch
                    ? "Kein Kind gefunden"
                    : "Noch keine beendeten Betreuungen"
                }
                description={
                  debouncedSearch
                    ? "Versuchen Sie einen anderen Namen oder eine andere Klasse."
                    : "Hier stehen Kinder, deren Betreuung beendet wurde."
                }
              />
            }
          />

          {totalPages > 1 ? (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 pt-3">
              <p className="text-sm text-gray-600">
                Seite {page} von {totalPages}
              </p>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  aria-label="Vorherige Seite"
                  disabled={page <= 1}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  Zurück
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  disabled={page >= totalPages}
                  aria-label="Nächste Seite"
                  onClick={() =>
                    setPage((current) => Math.min(totalPages, current + 1))
                  }
                >
                  Weiter
                </Button>
              </div>
            </div>
          ) : null}
        </SectionCard>
      )}

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
            await refreshEndedCare();
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
            await refreshEndedCare();
          }}
        />
      ) : null}
    </TenantPage>
  );
}
