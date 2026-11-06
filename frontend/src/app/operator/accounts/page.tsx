"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { Suspense, useCallback, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrgAccount,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import { SimpleEmptyState } from "../provisioning/provisioning-shared";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";
import { AccountsTable } from "~/components/operator/accounts-table";
import { AccountTenantAccessModal } from "~/components/operator/account-tenant-access-modal";
import { StatusBadge } from "~/components/ui/status-badge";

function OperatorAccountsPageContent() {
  useSetBreadcrumb({ pageTitle: "Konten" });

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
  } = useOrgSchoolFilter("/operator/accounts");

  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const [mfaAccount, setMfaAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [mfaSchoolContext, setMfaSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [tenantAccessAccount, setTenantAccessAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);

  const refreshFamily = useTenantMutateMatching(
    operatorProvisioningKeys.accountFamilies,
  );

  const {
    data: schoolAccounts,
    error: schoolAccountsError,
    isLoading: schoolAccountsLoading,
  } = useSWRAuth(
    isAuthenticated && urlSchoolId
      ? operatorProvisioningKeys.schoolAccounts(urlSchoolId)
      : null,
    () => operatorProvisioningService.listSchoolAccounts(urlSchoolId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: orgAccounts,
    error: orgAccountsError,
    isLoading: orgAccountsLoading,
  } = useSWRAuth(
    isAuthenticated && filterOrgId && !urlSchoolId
      ? operatorProvisioningKeys.orgAccounts(filterOrgId)
      : null,
    () => operatorProvisioningService.listOrganizationAccounts(filterOrgId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: allAccounts,
    error: allAccountsError,
    isLoading: allAccountsLoading,
  } = useSWRAuth(
    isAuthenticated && !filterOrgId && !urlSchoolId
      ? operatorProvisioningKeys.allAccounts()
      : null,
    () => operatorProvisioningService.listAllAccounts(),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const refreshAccounts = useCallback(() => refreshFamily(), [refreshFamily]);

  // Only one of the three lists is active at a time.
  const accountsLoadError = useSwrLoadError(
    urlSchoolId
      ? schoolAccountsError
      : filterOrgId
        ? orgAccountsError
        : allAccountsError,
    "die Liste der Konten",
    () => void refreshAccounts(),
  );

  const openCaregiverModal = useCallback(
    (
      account: SchoolAccount | OrgAccount,
      schoolContext: { id: string; name: string } | null,
    ) => {
      setCaregiverAccount(account);
      setCaregiverSchoolContext(schoolContext);
    },
    [],
  );

  const openMFAModal = useCallback(
    (
      account: SchoolAccount | OrgAccount,
      schoolContext: { id: string; name: string } | null,
    ) => {
      setMfaAccount(account);
      setMfaSchoolContext(schoolContext);
    },
    [],
  );

  const openTenantAccessModal = useCallback(
    (account: SchoolAccount | OrgAccount) => setTenantAccessAccount(account),
    [],
  );

  const count = selectedSchool
    ? schoolAccounts?.length
    : filterOrgId
      ? orgAccounts?.length
      : allAccounts?.length;

  return (
    <TenantPage
      title="Konten"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats
            items={[{ value: count, label: count === 1 ? "Konto" : "Konten" }]}
          />
        )
      }
      searchSlot={
        <OrgSchoolFilter
          idPrefix="account"
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
          {mfaAccount && mfaSchoolContext ? (
            <MFAAdminOverrideModal
              isOpen={true}
              onClose={() => {
                setMfaAccount(null);
                setMfaSchoolContext(null);
              }}
              scope="operator"
              schoolId={mfaSchoolContext.id}
              accountId={mfaAccount.accountId}
              accountLabel={`${mfaAccount.firstName} ${mfaAccount.lastName}`.trim()}
            />
          ) : null}

          {tenantAccessAccount ? (
            <AccountTenantAccessModal
              isOpen={true}
              onClose={() => setTenantAccessAccount(null)}
              accountId={tenantAccessAccount.accountId}
              accountLabel={
                `${tenantAccessAccount.firstName} ${tenantAccessAccount.lastName}`.trim() ||
                tenantAccessAccount.email
              }
              accountEmail={tenantAccessAccount.email}
              onUpdated={async () => {
                await refreshAccounts();
              }}
            />
          ) : null}

          {caregiverAccount && caregiverSchoolContext ? (
            <CaregiverCapabilityModal
              isOpen={true}
              onClose={() => {
                setCaregiverAccount(null);
                setCaregiverSchoolContext(null);
              }}
              scope="operator"
              accountId={caregiverAccount.accountId}
              accountLabel={`${caregiverAccount.firstName} ${caregiverAccount.lastName}`.trim()}
              schoolId={caregiverSchoolContext.id}
              schoolName={caregiverSchoolContext.name}
              onUpdated={async () => {
                await refreshAccounts();
              }}
            />
          ) : null}
        </>
      }
    >
      <LoadErrorAlert error={organizationsLoadError} />
      <LoadErrorAlert error={schoolsLoadError} />
      <LoadErrorAlert error={accountsLoadError} />

      {!urlSchoolId && filterOrgId && (
        <>
          {!orgAccountsError &&
          !orgAccountsLoading &&
          orgAccounts?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Konten"
              description="Für diesen Träger gibt es noch keine zugewiesenen Konten."
            />
          ) : (orgAccounts === undefined && !orgAccountsLoading) ||
            orgAccountsError ? null : (
            <AccountsTable
              accounts={orgAccounts ?? []}
              showSchool
              isLoading={orgAccountsLoading}
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}

      {!urlSchoolId && !filterOrgId && (
        <>
          {!allAccountsError &&
          !allAccountsLoading &&
          allAccounts?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Konten"
              description="Es gibt noch keine Konten im System."
            />
          ) : (allAccounts === undefined && !allAccountsLoading) ||
            allAccountsError ? null : (
            <AccountsTable
              accounts={allAccounts ?? []}
              showSchool
              isLoading={allAccountsLoading}
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}

      {selectedSchool && (
        <>
          <div className="mb-3 flex items-center gap-2">
            <h3 className="text-sm font-semibold text-gray-900">
              {selectedSchool.name}
            </h3>
            {selectedSchool.organization && (
              <span className="text-xs text-gray-400">
                {selectedSchool.organization.name}
              </span>
            )}
            {schoolAccounts && (
              <StatusBadge
                tone="gray"
                compact
                label={`${schoolAccounts.length} ${
                  schoolAccounts.length === 1 ? "Konto" : "Konten"
                }`}
              />
            )}
          </div>
          {!schoolAccountsError &&
          !schoolAccountsLoading &&
          schoolAccounts?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Konten"
              description="Für diese Schule gibt es noch keine zugewiesenen Konten."
            />
          ) : (schoolAccounts === undefined && !schoolAccountsLoading) ||
            schoolAccountsError ? null : (
            <AccountsTable
              accounts={schoolAccounts ?? []}
              selectedSchool={selectedSchool}
              isLoading={schoolAccountsLoading}
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}
    </TenantPage>
  );
}

export default function OperatorAccountsPage() {
  return (
    <Suspense>
      <OperatorAccountsPageContent />
    </Suspense>
  );
}
