"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import {
  CardSkeletons,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";

const logger = createLogger({ component: "OperatorPersonsPage" });

function OperatorPersonsPageContent() {
  useSetBreadcrumb({ pageTitle: "Personen" });

  const {
    isAuthenticated,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
  } = useOrgSchoolFilter("/operator/persons");

  const [deletePersonTarget, setDeletePersonTargetRaw] =
    useState<OperatorPerson | null>(null);
  const [deletePersonConfirmInput, setDeletePersonConfirmInput] = useState("");
  const [deletePersonLoading, setDeletePersonLoading] = useState(false);
  const [deletePersonError, setDeletePersonError] = useState("");

  const setDeletePersonTarget = useCallback((person: OperatorPerson | null) => {
    setDeletePersonTargetRaw(person);
    setDeletePersonConfirmInput("");
    setDeletePersonError("");
  }, []);

  const {
    data: schoolPersons,
    isLoading: schoolPersonsLoading,
    mutate: mutateSchoolPersons,
  } = useSWR(
    isAuthenticated && selectedSchool
      ? `operator-school-persons-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolPersons(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const handleDeletePerson = useCallback(async () => {
    if (!deletePersonTarget) return;
    setDeletePersonLoading(true);
    setDeletePersonError("");
    try {
      await operatorProvisioningService.softDeletePerson(deletePersonTarget.id);
      setDeletePersonTarget(null);
      void mutateSchoolPersons();
    } catch (err) {
      logger.error("person_soft_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setDeletePersonError(
        err instanceof Error ? err.message : "Fehler beim Löschen der Person",
      );
    } finally {
      setDeletePersonLoading(false);
    }
  }, [deletePersonTarget, mutateSchoolPersons, setDeletePersonTarget]);

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "persons",
          label: "Personen",
          count: selectedSchool ? schoolPersons?.length : undefined,
        },
      ],
      activeTab: "persons",
      onTabChange: () => undefined,
    }),
    [selectedSchool, schoolPersons?.length],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Personen" tabs={tabs} />

      <OrgSchoolFilter
        idPrefix="person"
        organizations={activeOrganizations}
        filteredSchools={filteredSchools}
        filterOrgId={filterOrgId}
        selectedSchool={selectedSchool}
        onOrgChange={handleOrgFilterChange}
        onSchoolChange={handleSchoolFilterChange}
      />

      {!selectedSchool ? (
        <SimpleEmptyState
          title="Keine Schule ausgewählt"
          description="Wählen Sie eine Schule aus, um deren Personen anzuzeigen."
        />
      ) : schoolPersonsLoading ? (
        <CardSkeletons />
      ) : !schoolPersons || schoolPersons.length === 0 ? (
        <SimpleEmptyState
          title="Keine Personen"
          description={`Keine Personen in ${selectedSchool.name} vorhanden.`}
        />
      ) : (
        <>
          <p className="mb-4 text-sm text-gray-500">
            {schoolPersons.length}{" "}
            {schoolPersons.length === 1 ? "Person" : "Personen"} in{" "}
            <span className="font-medium">{selectedSchool.name}</span>
          </p>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {schoolPersons.map((person) => (
              <div
                key={person.id}
                className="flex items-center justify-between rounded-xl border border-gray-100 bg-white p-4 shadow-sm"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-gray-900">
                    {person.firstName} {person.lastName}
                  </p>
                  <div className="mt-1 flex flex-wrap gap-1.5">
                    {person.isStaff && (
                      <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                        Mitarbeiter
                      </span>
                    )}
                    {person.isStudent && (
                      <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                        Kinder
                      </span>
                    )}
                    {person.hasRfidCard && (
                      <span className="inline-flex items-center rounded-full bg-purple-50 px-2 py-0.5 text-xs font-medium text-purple-700">
                        RFID
                      </span>
                    )}
                  </div>
                  {person.accountEmail && (
                    <p className="mt-1 truncate text-xs text-gray-500">
                      {person.accountEmail}
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => setDeletePersonTarget(person)}
                  className="ml-3 shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600"
                  title="Person löschen"
                >
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
            ))}
          </div>
        </>
      )}

      {deletePersonTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h3 className="text-lg font-semibold text-gray-900">
              Person löschen
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              Möchten Sie{" "}
              <span className="font-medium">
                {deletePersonTarget.firstName} {deletePersonTarget.lastName}
              </span>{" "}
              von{" "}
              <span className="font-medium">
                {deletePersonTarget.schoolName}
              </span>{" "}
              wirklich löschen?
            </p>
            <div className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
              <p className="font-medium">
                Folgende Aktionen werden ausgeführt:
              </p>
              <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
                <li>Account wird deaktiviert und Login gesperrt</li>
                <li>Persönliche Daten werden anonymisiert</li>
                <li>RFID-Karte wird freigegeben</li>
                <li>Diese Aktion kann nicht rückgängig gemacht werden</li>
              </ul>
            </div>

            <div className="mt-4">
              <label
                htmlFor="delete-person-confirm"
                className="block text-sm font-medium text-gray-700"
              >
                Geben Sie den vollständigen Namen ein:
              </label>
              <p className="mb-1 text-sm font-medium text-gray-900">
                {deletePersonTarget.fullName}
              </p>
              <input
                id="delete-person-confirm"
                type="text"
                value={deletePersonConfirmInput}
                onChange={(e) => setDeletePersonConfirmInput(e.target.value)}
                placeholder={deletePersonTarget.fullName}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none"
                autoComplete="off"
              />
            </div>

            {deletePersonError && (
              <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
                {deletePersonError}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setDeletePersonTarget(null)}
                disabled={deletePersonLoading}
                className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
              >
                Abbrechen
              </button>
              <button
                type="button"
                onClick={() => void handleDeletePerson()}
                disabled={
                  deletePersonLoading ||
                  deletePersonConfirmInput !== deletePersonTarget.fullName
                }
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {deletePersonLoading ? "Wird gelöscht..." : "Endgültig löschen"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default function OperatorPersonsPage() {
  return (
    <Suspense fallback={<div className="-mt-1.5 w-full" />}>
      <OperatorPersonsPageContent />
    </Suspense>
  );
}
