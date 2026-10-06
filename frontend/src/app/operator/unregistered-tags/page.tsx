"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { getRelativeTime } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { UnregisteredTagScan } from "~/lib/operator/provisioning-helpers";
import { SimpleEmptyState } from "../provisioning/provisioning-shared";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { ListPageSkeleton } from "~/components/ui/page-skeletons";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { useApiFormError } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";

const logger = createLogger({ component: "OperatorUnregisteredTagsPage" });

type ResolvedFilter = "unresolved" | "all";

function OperatorUnregisteredTagsPageContent() {
  useSetBreadcrumb({ pageTitle: "Unbekannte RFID" });

  const {
    isAuthenticated,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
  } = useOrgSchoolFilter("/operator/unregistered-tags");

  const [resolvedFilter, setResolvedFilter] =
    useState<ResolvedFilter>("unresolved");
  const [resolveTarget, setResolveTarget] =
    useState<UnregisteredTagScan | null>(null);
  const [resolutionNote, setResolutionNote] = useState("");
  const resolveErrors = useApiFormError();
  const { show: showResolveError, clear: clearResolveError } = resolveErrors;
  const [resolving, setResolving] = useState(false);

  const swrKey = useMemo(() => {
    if (!isAuthenticated) return null;
    const orgPart = filterOrgId || "all-orgs";
    const schoolPart = selectedSchool?.id ?? "all-schools";
    return `operator-unregistered-tags-${resolvedFilter}-${orgPart}-${schoolPart}`;
  }, [filterOrgId, isAuthenticated, resolvedFilter, selectedSchool?.id]);

  const {
    data: scans,
    error: scansError,
    isLoading,
    mutate,
  } = useSWR(
    swrKey,
    () =>
      operatorProvisioningService.listUnregisteredTagScans({
        organizationId: filterOrgId || undefined,
        schoolId: selectedSchool?.id,
        resolved: resolvedFilter,
      }),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const scansLoadError = useSwrLoadError(
    scansError,
    "die Liste der RFID-Scans",
    () => void mutate(),
  );

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "unregistered-tags",
          label: "Unbekannte RFID",
          count: scans?.length,
        },
      ],
      activeTab: "unregistered-tags",
      onTabChange: () => undefined,
    }),
    [scans?.length],
  );

  const filters = useMemo<FilterConfig[]>(
    () => [
      {
        id: "status",
        label: "Status",
        type: "dropdown",
        value: resolvedFilter,
        onChange: (value) => {
          const nextValue = Array.isArray(value) ? value[0] : value;
          setResolvedFilter(nextValue === "all" ? "all" : "unresolved");
        },
        options: [
          { value: "unresolved", label: "Offen" },
          { value: "all", label: "Alle" },
        ],
      },
    ],
    [resolvedFilter],
  );

  const openResolveModal = useCallback(
    (scan: UnregisteredTagScan) => {
      setResolveTarget(scan);
      setResolutionNote("");
      clearResolveError();
    },
    [clearResolveError],
  );

  const closeResolveModal = useCallback(() => {
    setResolveTarget(null);
    setResolutionNote("");
    clearResolveError();
  }, [clearResolveError]);

  const handleResolve = useCallback(async () => {
    if (!resolveTarget) return;
    setResolving(true);
    clearResolveError();
    try {
      await operatorProvisioningService.resolveUnregisteredTagScan(
        resolveTarget.id,
        resolutionNote.trim() || undefined,
      );
      closeResolveModal();
      void mutate();
    } catch (err) {
      logger.warn("unregistered_tag_resolve_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showResolveError(err, { object: "das Erledigen des Scans" });
    } finally {
      setResolving(false);
    }
  }, [
    clearResolveError,
    closeResolveModal,
    mutate,
    resolutionNote,
    resolveTarget,
    showResolveError,
  ]);

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Unbekannte RFID"
        concept="rfid"
        tabs={tabs}
        filters={filters}
      />

      <OrgSchoolFilter
        idPrefix="unregistered-tags"
        organizations={activeOrganizations}
        filteredSchools={filteredSchools}
        filterOrgId={filterOrgId}
        selectedSchool={selectedSchool}
        onOrgChange={handleOrgFilterChange}
        onSchoolChange={handleSchoolFilterChange}
      />

      <LoadErrorAlert error={scansLoadError} className="mb-4" />

      {!isLoading && scans?.length === 0 ? (
        <SimpleEmptyState
          title="Keine unbekannten RFID-Scans"
          description="Es liegen keine passenden Scanversuche vor."
        />
      ) : (
        <UnregisteredTagsTable
          scans={scans ?? []}
          isLoading={isLoading}
          onResolve={openResolveModal}
        />
      )}

      {resolveTarget && (
        <ResolveScanModal
          scan={resolveTarget}
          note={resolutionNote}
          error={resolveErrors.error}
          loading={resolving}
          onNoteChange={setResolutionNote}
          onClose={closeResolveModal}
          onResolve={handleResolve}
        />
      )}
    </div>
  );
}

function UnregisteredTagsTable({
  scans,
  isLoading,
  onResolve,
}: Readonly<{
  scans: UnregisteredTagScan[];
  isLoading: boolean;
  onResolve: (scan: UnregisteredTagScan) => void;
}>) {
  const columns: DataTableColumn<UnregisteredTagScan>[] = useMemo(
    () => [
      {
        key: "tagUid",
        header: "RFID",
        render: (row) => row.tagUid,
        sortValue: (row) => row.tagUid.toLowerCase(),
        className: "font-mono text-xs font-medium text-gray-900",
      },
      {
        key: "schoolName",
        header: "Schule",
        render: (row) => (
          <div>
            <div className="font-medium text-gray-900">{row.schoolName}</div>
            <div className="text-xs text-gray-500">{row.organizationName}</div>
          </div>
        ),
        sortValue: (row) => row.schoolName.toLowerCase(),
      },
      {
        key: "device",
        header: "Gerät",
        render: (row) => row.deviceName || row.deviceIdentifier || "—",
        sortValue: (row) =>
          (row.deviceName || row.deviceIdentifier || "").toLowerCase(),
        className: "text-gray-600",
      },
      {
        key: "scannedAt",
        header: "Zeitpunkt",
        render: (row) => (
          <span
            title={new Date(row.scannedAt).toLocaleString("de-DE", {
              timeZone: "Europe/Berlin",
            })}
          >
            {getRelativeTime(row.scannedAt)}
          </span>
        ),
        sortValue: (row) => row.scannedAt,
        className: "text-gray-600",
      },
      {
        key: "status",
        header: "Status",
        render: (row) =>
          row.resolvedAt ? (
            <span className="bg-moto-green/15 text-moto-green-strong inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
              Erledigt
            </span>
          ) : (
            <span className="bg-moto-amber/15 text-moto-amber-strong inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
              Offen
            </span>
          ),
        sortValue: (row) => (row.resolvedAt ? 1 : 0),
      },
      {
        key: "action",
        header: "Aktion",
        align: "right",
        render: (row) =>
          !row.resolvedAt && (
            <button
              type="button"
              onClick={() => onResolve(row)}
              className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
            >
              Erledigen
            </button>
          ),
      },
    ],
    [onResolve],
  );

  return (
    <DataTable
      columns={columns}
      rows={scans}
      getRowKey={(row) => row.id}
      defaultSortKey="scannedAt"
      defaultSortDirection="desc"
      isLoading={isLoading}
    />
  );
}

export function ResolveScanModal({
  scan,
  note,
  error,
  loading,
  onNoteChange,
  onClose,
  onResolve,
}: Readonly<{
  scan: UnregisteredTagScan;
  note: string;
  error: FormErrorInput;
  loading: boolean;
  onNoteChange: (note: string) => void;
  onClose: () => void;
  onResolve: () => void;
}>) {
  const footer = (
    <>
      <Button
        type="button"
        variant="secondary"
        size="md"
        onClick={onClose}
        disabled={loading}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={onResolve}
        disabled={loading}
      >
        {loading ? "Speichern..." : "Erledigen"}
      </Button>
    </>
  );

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Scan erledigen"
      footer={footer}
      isDismissDisabled={loading}
      isBackdropDismissDisabled
    >
      <FormErrorAlert message={error} className="mb-3" />
      <p className="text-sm text-gray-600">
        <span className="font-mono font-medium">{scan.tagUid}</span> ·{" "}
        {scan.schoolName}
      </p>
      <label
        htmlFor="resolution-note"
        className="mt-4 block text-sm font-medium text-gray-700"
      >
        Notiz
      </label>
      <textarea
        id="resolution-note"
        name="resolution_note"
        value={note}
        onChange={(event) => onNoteChange(event.target.value)}
        className="mt-1 min-h-24 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-100 focus:outline-none"
      />
    </Modal>
  );
}

export default function OperatorUnregisteredTagsPage() {
  return (
    <Suspense
      fallback={<ListPageSkeleton label="Unbekannte RFID werden geladen" />}
    >
      <OperatorUnregisteredTagsPageContent />
    </Suspense>
  );
}
