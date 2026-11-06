"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { Suspense, useCallback, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth, useInvalidateMatching } from "~/lib/swr";
import { Trash2 } from "lucide-react";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { PersonTags } from "~/components/operator/persons-table";
import { StaffPersonMerge } from "~/components/operator/staff-person-merge";
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
    urlSchoolId,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
    organizationsLoadError,
    schoolsLoadError,
  } = useOrgSchoolFilter("/operator/persons");

  const invalidate = useInvalidateMatching();
  const refreshPersons = useCallback(
    () => invalidate(operatorProvisioningKeys.personChanges),
    [invalidate],
  );

  const [deletePersonTarget, setDeletePersonTarget] =
    useState<OperatorPerson | null>(null);

  const {
    data: schoolPersons,
    error: schoolPersonsError,
    isLoading: schoolPersonsLoading,
    mutate: mutateSchoolPersons,
  } = useSWRAuth(
    isAuthenticated && urlSchoolId
      ? operatorProvisioningKeys.schoolPersons(urlSchoolId)
      : null,
    () => operatorProvisioningService.listSchoolPersons(urlSchoolId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const personsLoadError = useSwrLoadError(
    schoolPersonsError,
    "die Liste der Personen",
    () => void mutateSchoolPersons(),
  );

  const count = selectedSchool ? schoolPersons?.length : undefined;

  return (
    <TenantPage
      title="Personen"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats
            items={[
              { value: count, label: count === 1 ? "Person" : "Personen" },
            ]}
          />
        )
      }
      searchSlot={
        <OrgSchoolFilter
          idPrefix="person"
          organizations={activeOrganizations}
          filteredSchools={filteredSchools}
          filterOrgId={filterOrgId}
          selectedSchool={selectedSchool}
          onOrgChange={handleOrgFilterChange}
          onSchoolChange={handleSchoolFilterChange}
        />
      }
      overlays={
        <>
          <DeletePersonModal
            person={deletePersonTarget}
            onClose={() => setDeletePersonTarget(null)}
            onDeleted={(deletedPerson) =>
              mutateSchoolPersons(
                (persons) =>
                  persons?.filter((person) => person.id !== deletedPerson.id),
                { revalidate: false },
              ).then(async () => {
                await refreshPersons();
              })
            }
          />
        </>
      }
    >
      <LoadErrorAlert error={organizationsLoadError} />
      <LoadErrorAlert error={schoolsLoadError} />
      <LoadErrorAlert error={personsLoadError} />

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
      ) : !schoolPersons || schoolPersonsError ? null : schoolPersons.length ===
        0 ? (
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
                className="moto-content-surface flex items-center justify-between rounded-xl border p-4"
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
                <OverflowMenu
                  items={[
                    {
                      label: "Person löschen",
                      icon: <Trash2 className="h-4 w-4" aria-hidden="true" />,
                      destructive: true,
                      onClick: () => setDeletePersonTarget(person),
                    },
                  ]}
                  triggerSize="sm"
                  ariaLabel={`Aktionen für ${person.firstName} ${person.lastName}`}
                />
              </div>
            ))}
          </div>
        </>
      )}

      {selectedSchool && (
        <StaffPersonMerge
          key={selectedSchool.id}
          schoolId={selectedSchool.id}
          schoolName={selectedSchool.name}
          onMerged={refreshPersons}
        />
      )}
    </TenantPage>
  );
}

export default function OperatorPersonsPage() {
  return (
    <Suspense fallback={<TenantPage title="Personen" loading />}>
      <OperatorPersonsPageContent />
    </Suspense>
  );
}
