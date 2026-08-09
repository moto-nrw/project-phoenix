"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrgAccount,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import {
  CardSkeletons,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { useSession } from "next-auth/react";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";
import { AccountsTable } from "~/components/operator/accounts-table";
import { AccountTenantAccessModal } from "~/components/operator/account-tenant-access-modal";

const ACCOUNT_SWR_PREFIXES = [
  "operator-all-accounts",
  "operator-school-accounts-",
  "operator-org-accounts-",
];

function OperatorAccountsPageContent() {
  useSetBreadcrumb({ pageTitle: "Konten" });

  const {
    isAuthenticated,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
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
  const { data: session } = useSession();
  const operatorBearerToken = session?.user?.token ?? "";

  const { mutate: globalMutate } = useSWRConfig();

  const { data: schoolAccounts, isLoading: schoolAccountsLoading } = useSWR(
    isAuthenticated && selectedSchool
      ? `operator-school-accounts-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolAccounts(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: orgAccounts, isLoading: orgAccountsLoading } = useSWR(
    isAuthenticated && filterOrgId && !selectedSchool
      ? `operator-org-accounts-${filterOrgId}`
      : null,
    () => operatorProvisioningService.listOrganizationAccounts(filterOrgId),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: allAccounts, isLoading: allAccountsLoading } = useSWR(
    isAuthenticated && !filterOrgId && !selectedSchool
      ? "operator-all-accounts"
      : null,
    () => operatorProvisioningService.listAllAccounts(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const accountsLoading = selectedSchool
    ? schoolAccountsLoading
    : filterOrgId
      ? orgAccountsLoading
      : allAccountsLoading;

  const refreshAccounts = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        ACCOUNT_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate]);

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

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "accounts",
          label: "Konten",
          count: selectedSchool
            ? schoolAccounts?.length
            : filterOrgId
              ? orgAccounts?.length
              : allAccounts?.length,
        },
      ],
      activeTab: "accounts",
      onTabChange: () => undefined,
    }),
    [
      selectedSchool,
      filterOrgId,
      schoolAccounts?.length,
      orgAccounts?.length,
      allAccounts?.length,
    ],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Konten" concept="accounts" tabs={tabs} />

      <OrgSchoolFilter
        idPrefix="account"
        organizations={activeOrganizations}
        filteredSchools={filteredSchools}
        filterOrgId={filterOrgId}
        selectedSchool={selectedSchool}
        onOrgChange={handleOrgFilterChange}
        onSchoolChange={handleSchoolFilterChange}
      />

      {accountsLoading && <CardSkeletons />}

      {!selectedSchool && filterOrgId && !orgAccountsLoading && (
        <>
          {orgAccounts?.length === 0 && (
            <SimpleEmptyState
              title="Keine Konten"
              description="Für diesen Träger gibt es noch keine zugewiesenen Konten."
            />
          )}
          {orgAccounts && orgAccounts.length > 0 && (
            <AccountsTable
              accounts={orgAccounts}
              showSchool
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}

      {!selectedSchool && !filterOrgId && !allAccountsLoading && (
        <>
          {allAccounts?.length === 0 && (
            <SimpleEmptyState
              title="Keine Konten"
              description="Es gibt noch keine Konten im System."
            />
          )}
          {allAccounts && allAccounts.length > 0 && (
            <AccountsTable
              accounts={allAccounts}
              showSchool
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}

      {selectedSchool && !schoolAccountsLoading && (
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
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                {schoolAccounts.length}{" "}
                {schoolAccounts.length === 1 ? "Konto" : "Konten"}
              </span>
            )}
          </div>
          {schoolAccounts?.length === 0 && (
            <SimpleEmptyState
              title="Keine Konten"
              description="Für diese Schule gibt es noch keine zugewiesenen Konten."
            />
          )}
          {schoolAccounts && schoolAccounts.length > 0 && (
            <AccountsTable
              accounts={schoolAccounts}
              selectedSchool={selectedSchool}
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
              onManageTenantAccess={openTenantAccessModal}
            />
          )}
        </>
      )}

      {mfaAccount && mfaSchoolContext ? (
        <MFAAdminOverrideModal
          isOpen={true}
          onClose={() => {
            setMfaAccount(null);
            setMfaSchoolContext(null);
          }}
          scope="operator"
          schoolId={mfaSchoolContext.id}
          bearerToken={operatorBearerToken}
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
    </div>
  );
}

export default function OperatorAccountsPage() {
  return (
    <Suspense fallback={<div className="-mt-1.5 w-full" />}>
      <OperatorAccountsPageContent />
    </Suspense>
  );
}
