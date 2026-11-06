"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth } from "~/lib/swr";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { CustomSelect } from "~/components/ui/custom-select";
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
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";

const logger = createLogger({ component: "OperatorUnregisteredTagsPage" });

type ResolvedFilter = "unresolved" | "all";

function OperatorUnregisteredTagsPageContent() {
  useSetBreadcrumb({ pageTitle: "Unbekannte RFID" });

  const {
    isAuthenticated,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    urlSchoolId,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
    organizationsLoadError,
    schoolsLoadError,
  } = useOrgSchoolFilter("/operator/unregistered-tags");

  const [resolvedFilter, setResolvedFilter] =
    useState<ResolvedFilter>("unresolved");
  const [resolveTarget, setResolveTarget] =
    useState<UnregisteredTagScan | null>(null);
  const [resolutionNote, setResolutionNote] = useState("");
  const resolveErrors = useApiFormError();
  const { show: showResolveError, clear: clearResolveError } = resolveErrors;
  const [resolving, setResolving] = useState(false);

  const {
    data: scans,
    error: scansError,
    isLoading,
    mutate,
  } = useSWRAuth(
    isAuthenticated
      ? operatorProvisioningKeys.unregisteredTags(
          resolvedFilter,
          filterOrgId,
          urlSchoolId,
        )
      : null,
    () =>
      operatorProvisioningService.listUnregisteredTagScans({
        organizationId: filterOrgId || undefined,
        schoolId: urlSchoolId || undefined,
        resolved: resolvedFilter,
      }),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const scansLoadError = useSwrLoadError(
    scansError,
    "die Liste der RFID-Scans",
    () => void mutate(),
  );

  const count = scans?.length;

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
    <TenantPage
      title="Unbekannte RFID"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats
            items={[{ value: count, label: count === 1 ? "Scan" : "Scans" }]}
          />
        )
      }
      searchSlot={
        <OrgSchoolFilter
          idPrefix="unregistered-tags"
          organizations={activeOrganizations}
          filteredSchools={filteredSchools}
          filterOrgId={filterOrgId}
          selectedSchool={selectedSchool}
          onOrgChange={handleOrgFilterChange}
          onSchoolChange={handleSchoolFilterChange}
        >
          <StatusFilter value={resolvedFilter} onChange={setResolvedFilter} />
        </OrgSchoolFilter>
      }
      overlays={
        <>
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
        </>
      }
    >
      <LoadErrorAlert error={organizationsLoadError} />
      <LoadErrorAlert error={schoolsLoadError} />
      <LoadErrorAlert error={scansLoadError} />

      {!isLoading && !scansError && scans?.length === 0 ? (
        <SimpleEmptyState
          title="Keine unbekannten RFID-Scans"
          description="Es liegen keine passenden Scanversuche vor."
        />
      ) : (scans === undefined && !isLoading) || scansError ? null : (
        <UnregisteredTagsTable
          scans={scans ?? []}
          isLoading={isLoading}
          onResolve={openResolveModal}
        />
      )}
    </TenantPage>
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
            <StatusBadge tone="green" compact label="Erledigt" />
          ) : (
            <StatusColorBadge
              label="Offen"
              color={LOCATION_COLORS.WARNING}
              compact
            />
          ),
        sortValue: (row) => (row.resolvedAt ? 1 : 0),
      },
      {
        key: "actions",
        header: "Aktion",
        align: "right",
        render: (row) =>
          !row.resolvedAt && (
            <Button
              type="button"
              variant="surface"
              size="compact"
              onClick={() => onResolve(row)}
            >
              Erledigen
            </Button>
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
        variant="outline"
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
      <div className="mt-4">
        <Textarea
          id="resolution-note"
          name="resolution_note"
          label="Notiz"
          value={note}
          onChange={(event) => onNoteChange(event.target.value)}
          className="min-h-24 resize-y"
        />
      </div>
    </Modal>
  );
}

function StatusFilter({
  value,
  onChange,
}: {
  readonly value: "all" | "unresolved";
  readonly onChange: (value: "all" | "unresolved") => void;
}) {
  return (
    <div className="flex-1">
      <label
        id="filter-unregistered-tags-status-label"
        htmlFor="filter-unregistered-tags-status"
        className="mb-1 block text-xs font-medium text-gray-500"
      >
        Status
      </label>
      <CustomSelect
        id="filter-unregistered-tags-status"
        ariaLabelledBy="filter-unregistered-tags-status-label"
        value={value}
        options={[
          { value: "unresolved", label: "Offen" },
          { value: "all", label: "Alle" },
        ]}
        onChange={(next) => onChange(next === "all" ? "all" : "unresolved")}
      />
    </div>
  );
}

export default function OperatorUnregisteredTagsPage() {
  return (
    <Suspense fallback={<TenantPage title="Unbekannte RFID" loading />}>
      <OperatorUnregisteredTagsPageContent />
    </Suspense>
  );
}
