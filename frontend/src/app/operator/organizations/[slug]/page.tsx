"use client";

import { use, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrgAccount,
  OrganizationSummary,
  SchoolAccount,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { PersonsTable } from "~/components/operator/persons-table";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { CaregiverCapabilityModal } from "~/components/teachers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorOrganizationDetailPage" });

function numberFormat(value: number): string {
  return new Intl.NumberFormat("de-DE").format(value);
}

const TAB_ITEMS = [
  { id: "schulen", label: "Schulen" },
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
  { id: "feedback", label: "Feedback" },
  { id: "ankuendigungen", label: "Ankündigungen" },
] as const;

type TabId = (typeof TAB_ITEMS)[number]["id"];

interface PageProps {
  readonly params: Promise<{ slug: string }>;
}

function isTabId(value: string | null | undefined): value is TabId {
  return TAB_ITEMS.some((t) => t.id === value);
}

export default function OperatorOrganizationDetailPage({ params }: PageProps) {
  const { slug } = use(params);
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "schulen";
  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const setActiveTab = useCallback(
    (next: TabId) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next === "schulen") {
        params.delete("tab");
      } else {
        params.set("tab", next);
      }
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const { data: organizations, isLoading } = useSWR(
    isAuthenticated ? "operator-organization-summaries" : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const organization: OrganizationSummary | undefined = useMemo(
    () => organizations?.find((o) => o.slug === slug),
    [organizations, slug],
  );

  useSetBreadcrumb({ pageTitle: organization?.name ?? "Träger" });

  const { data: schools, isLoading: schoolsLoading } = useSWR(
    isAuthenticated && organization
      ? ["operator-organization-schools", organization.id]
      : null,
    () =>
      operatorProvisioningService.listOrganizationSchools(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const accountsActive =
    activeTab === "konten" && isAuthenticated && organization != null;
  const {
    data: orgAccounts,
    isLoading: accountsLoading,
    mutate: mutateOrgAccounts,
  } = useSWR(
    accountsActive ? ["operator-org-accounts", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationAccounts(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && organization != null;
  const { data: orgDevices, isLoading: devicesLoading } = useSWR(
    devicesActive ? ["operator-org-devices", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationDevices(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && organization != null;
  const { data: orgPersons, isLoading: personsLoading } = useSWR(
    personsActive ? ["operator-org-persons", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationPersons(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const handleSchoolClick = useCallback(
    (school: SchoolSummary) => {
      router.push(
        `/operator/organizations/${encodeURIComponent(slug)}/schools/${encodeURIComponent(school.slug)}`,
      );
    },
    [router, slug],
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

  const schoolColumns: DataTableColumn<SchoolSummary>[] = useMemo(
    () => [
      {
        key: "name",
        header: "Schule",
        render: (row) => (
          <div>
            <div className="font-semibold text-gray-900">{row.name}</div>
            <div className="font-mono text-xs text-gray-500">
              {row.subdomain}
            </div>
          </div>
        ),
      },
      {
        key: "konten",
        header: "Konten",
        align: "right",
        render: (row) => numberFormat(row.kontenCount),
      },
      {
        key: "geraete",
        header: "Geräte",
        align: "right",
        render: (row) => numberFormat(row.geraeteCount),
      },
      {
        key: "personen",
        header: "Personen",
        align: "right",
        render: (row) => numberFormat(row.personenCount),
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <DataTableStatusBadge active={row.active} />,
      },
    ],
    [],
  );

  if (isLoading && !organization) {
    return (
      <div className="w-full py-10 text-center text-gray-500">
        Wird geladen…
      </div>
    );
  }

  if (!organization) {
    logger.warn("organization_not_found_by_slug", { slug });
    return (
      <div className="w-full">
        <Link
          href="/operator/organizations"
          className="text-sm text-gray-600 hover:text-gray-900"
        >
          ← Zurück zur Träger-Übersicht
        </Link>
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 text-center">
          <p className="text-gray-600">Träger nicht gefunden.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      <Link
        href="/operator/organizations"
        className="mb-4 inline-flex items-center gap-2 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span aria-hidden>←</span>
        <span>Zurück zur Träger-Übersicht</span>
      </Link>

      <EntityHeaderCard
        title={organization.name}
        subdomain={organization.slug}
        active={organization.active}
        createdAt={organization.createdAt}
        stats={[
          { label: "Schulen", value: numberFormat(organization.schulenCount) },
          { label: "Konten", value: numberFormat(organization.kontenCount) },
          { label: "Geräte", value: numberFormat(organization.geraeteCount) },
          {
            label: "Personen",
            value: numberFormat(organization.personenCount),
          },
        ]}
      />

      <div className="mt-6">
        <Tabs
          value={activeTab}
          onValueChange={(value) => {
            if (isTabId(value)) setActiveTab(value);
          }}
        >
          <TabsList variant="line">
            {TAB_ITEMS.map((tab) => (
              <TabsTrigger key={tab.id} value={tab.id}>
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>

          <TabsPrimitive.Content value="schulen" className="mt-4">
            {schoolsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : (
              <DataTable
                columns={schoolColumns}
                rows={schools ?? []}
                getRowKey={(row) => row.id}
                onRowClick={handleSchoolClick}
                caption={`${schools?.length ?? 0} Schulen · gefiltert auf ${organization.name}`}
                emptyState="Keine Schulen für diesen Träger."
              />
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="konten" className="mt-4">
            {accountsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgAccounts && orgAccounts.length > 0 ? (
              <AccountsTable
                accounts={orgAccounts}
                showSchool
                onManageCaregiver={openCaregiverModal}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Konten für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="geraete" className="mt-4">
            {devicesLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgDevices && orgDevices.length > 0 ? (
              <DevicesTable devices={orgDevices} showSchool />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Geräte für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="personen" className="mt-4">
            {personsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgPersons && orgPersons.length > 0 ? (
              <PersonsTable persons={orgPersons} showSchool />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Personen für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>

          {(["feedback", "ankuendigungen"] as const).map((tabId) => (
            <TabsPrimitive.Content key={tabId} value={tabId} className="mt-4">
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Wird in einem folgenden Schritt gefiltert angezeigt.
              </div>
            </TabsPrimitive.Content>
          ))}
        </Tabs>
      </div>

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
            await mutateOrgAccounts();
          }}
        />
      ) : null}
    </div>
  );
}
