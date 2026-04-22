"use client";

import { use, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrganizationSummary,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
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

export default function OperatorOrganizationDetailPage({ params }: PageProps) {
  const { slug } = use(params);
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<TabId>("schulen");

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

  const handleSchoolClick = useCallback(
    (school: SchoolSummary) => {
      router.push(
        `/operator/organizations/${encodeURIComponent(slug)}/schools/${encodeURIComponent(school.slug)}`,
      );
    },
    [router, slug],
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
          onValueChange={(value) => setActiveTab(value as TabId)}
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

          {(
            [
              "konten",
              "geraete",
              "personen",
              "feedback",
              "ankuendigungen",
            ] as const
          ).map((tabId) => (
            <TabsPrimitive.Content key={tabId} value={tabId} className="mt-4">
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Wird in einem folgenden Schritt gefiltert angezeigt.
              </div>
            </TabsPrimitive.Content>
          ))}
        </Tabs>
      </div>
    </div>
  );
}
