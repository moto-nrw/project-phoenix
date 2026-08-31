"use client";

import { Suspense, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { PersonTags } from "~/components/operator/persons-table";
import { DeletePersonModal } from "~/components/operator/delete-person-modal";
import { SimpleEmptyState } from "../provisioning/provisioning-shared";
import {
  SkeletonRegion,
  CardGridSkeleton,
} from "~/components/ui/page-skeletons";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";

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

  const [deletePersonTarget, setDeletePersonTarget] =
    useState<OperatorPerson | null>(null);

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
      <PageHeaderWithSearch title="Personen" concept="people" tabs={tabs} />

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
        <SkeletonRegion label="Personen werden geladen">
          <CardGridSkeleton
            cards={6}
            rowsPerCard={2}
            className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
          />
        </SkeletonRegion>
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
                    <PersonTags person={person} />
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
                  className="hover:bg-moto-red-soft hover:text-moto-red-hover ml-3 shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors"
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

      <DeletePersonModal
        person={deletePersonTarget}
        onClose={() => setDeletePersonTarget(null)}
        onDeleted={() => mutateSchoolPersons().then(() => undefined)}
      />
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
