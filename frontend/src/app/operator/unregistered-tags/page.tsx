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
import {
  CardSkeletons,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";

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
  const [resolveError, setResolveError] = useState("");
  const [resolving, setResolving] = useState(false);

  const swrKey = useMemo(() => {
    if (!isAuthenticated) return null;
    const orgPart = filterOrgId || "all-orgs";
    const schoolPart = selectedSchool?.id ?? "all-schools";
    return `operator-unregistered-tags-${resolvedFilter}-${orgPart}-${schoolPart}`;
  }, [filterOrgId, isAuthenticated, resolvedFilter, selectedSchool?.id]);

  const {
    data: scans,
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

  const openResolveModal = useCallback((scan: UnregisteredTagScan) => {
    setResolveTarget(scan);
    setResolutionNote("");
    setResolveError("");
  }, []);

  const closeResolveModal = useCallback(() => {
    setResolveTarget(null);
    setResolutionNote("");
    setResolveError("");
  }, []);

  const handleResolve = useCallback(async () => {
    if (!resolveTarget) return;
    setResolving(true);
    setResolveError("");
    try {
      await operatorProvisioningService.resolveUnregisteredTagScan(
        resolveTarget.id,
        resolutionNote.trim() || undefined,
      );
      closeResolveModal();
      void mutate();
    } catch (err) {
      logger.error("unregistered_tag_resolve_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setResolveError(
        err instanceof Error
          ? err.message
          : "RFID-Scan konnte nicht erledigt werden",
      );
    } finally {
      setResolving(false);
    }
  }, [closeResolveModal, mutate, resolutionNote, resolveTarget]);

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

      {isLoading && <CardSkeletons />}

      {!isLoading && (!scans || scans.length === 0) && (
        <SimpleEmptyState
          title="Keine unbekannten RFID-Scans"
          description="Es liegen keine passenden Scanversuche vor."
        />
      )}

      {!isLoading && scans && scans.length > 0 && (
        <UnregisteredTagsTable scans={scans} onResolve={openResolveModal} />
      )}

      {resolveTarget && (
        <ResolveScanModal
          scan={resolveTarget}
          note={resolutionNote}
          error={resolveError}
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
  onResolve,
}: Readonly<{
  scans: UnregisteredTagScan[];
  onResolve: (scan: UnregisteredTagScan) => void;
}>) {
  return (
    <div className="overflow-hidden rounded-lg border border-gray-100 bg-white shadow-sm">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-100 text-sm">
          <thead className="bg-gray-50">
            <tr className="text-left text-xs font-semibold tracking-wide text-gray-500 uppercase">
              <th className="px-4 py-3">RFID</th>
              <th className="px-4 py-3">Schule</th>
              <th className="px-4 py-3">Gerät</th>
              <th className="px-4 py-3">Zeitpunkt</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3 text-right">Aktion</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {scans.map((scan) => (
              <tr key={scan.id}>
                <td className="px-4 py-3 font-mono text-xs font-medium text-gray-900">
                  {scan.tagUid}
                </td>
                <td className="px-4 py-3">
                  <div className="font-medium text-gray-900">
                    {scan.schoolName}
                  </div>
                  <div className="text-xs text-gray-500">
                    {scan.organizationName}
                  </div>
                </td>
                <td className="px-4 py-3 text-gray-600">
                  {scan.deviceName || scan.deviceIdentifier || "—"}
                </td>
                <td className="px-4 py-3 text-gray-600">
                  <span
                    title={new Date(scan.scannedAt).toLocaleString("de-DE", {
                      timeZone: "Europe/Berlin",
                    })}
                  >
                    {getRelativeTime(scan.scannedAt)}
                  </span>
                </td>
                <td className="px-4 py-3">
                  {scan.resolvedAt ? (
                    <span className="bg-moto-green/15 text-moto-green-strong inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                      Erledigt
                    </span>
                  ) : (
                    <span className="bg-moto-amber/15 text-moto-amber-strong inline-flex rounded-full px-2 py-0.5 text-xs font-medium">
                      Offen
                    </span>
                  )}
                </td>
                <td className="px-4 py-3 text-right">
                  {!scan.resolvedAt && (
                    <button
                      type="button"
                      onClick={() => onResolve(scan)}
                      className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
                    >
                      Erledigen
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ResolveScanModal({
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
  error: string;
  loading: boolean;
  onNoteChange: (note: string) => void;
  onClose: () => void;
  onResolve: () => void;
}>) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="mx-4 w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <h3 className="text-lg font-semibold text-gray-900">Scan erledigen</h3>
        <p className="mt-2 text-sm text-gray-600">
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
          value={note}
          onChange={(event) => onNoteChange(event.target.value)}
          className="mt-1 min-h-24 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-2 focus:ring-gray-100 focus:outline-none"
        />
        {error && <p className="text-moto-red-strong mt-3 text-sm">{error}</p>}
        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            disabled={loading}
            className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={onResolve}
            disabled={loading}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-60"
          >
            {loading ? "Speichern..." : "Erledigen"}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function OperatorUnregisteredTagsPage() {
  return (
    <Suspense fallback={<CardSkeletons />}>
      <OperatorUnregisteredTagsPageContent />
    </Suspense>
  );
}
