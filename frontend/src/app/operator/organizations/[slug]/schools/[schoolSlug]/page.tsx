"use client";

import { use, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrganizationSummary,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { PersonsTable } from "~/components/operator/persons-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorSchoolDetailPage" });

function numberFormat(value: number): string {
  return new Intl.NumberFormat("de-DE").format(value);
}

const TAB_ITEMS = [
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
  { id: "feedback", label: "Feedback" },
  { id: "ankuendigungen", label: "Ankündigungen" },
] as const;

type TabId = (typeof TAB_ITEMS)[number]["id"];

interface PageProps {
  readonly params: Promise<{ slug: string; schoolSlug: string }>;
}

export default function OperatorSchoolDetailPage({ params }: PageProps) {
  const { slug, schoolSlug } = use(params);
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  const [activeTab, setActiveTab] = useState<TabId>("konten");

  const { data: organizations } = useSWR(
    isAuthenticated ? "operator-organization-summaries" : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const organization: OrganizationSummary | undefined = useMemo(
    () => organizations?.find((o) => o.slug === slug),
    [organizations, slug],
  );

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

  const school: SchoolSummary | undefined = useMemo(
    () => schools?.find((s) => s.slug === schoolSlug),
    [schools, schoolSlug],
  );

  useSetBreadcrumb({ pageTitle: school?.name ?? "Schule" });

  const accountsActive =
    activeTab === "konten" && isAuthenticated && school != null;
  const { data: schoolAccounts, isLoading: accountsLoading } = useSWR(
    accountsActive ? ["operator-school-accounts-", school?.id] : null,
    () => operatorProvisioningService.listSchoolAccounts(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && school != null;
  const { data: schoolDevices, isLoading: devicesLoading } = useSWR(
    devicesActive ? ["operator-school-devices-", school?.id] : null,
    () => operatorProvisioningService.listSchoolDevices(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && school != null;
  const { data: schoolPersons, isLoading: personsLoading } = useSWR(
    personsActive ? ["operator-school-persons-", school?.id] : null,
    () => operatorProvisioningService.listSchoolPersons(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const selectedSchoolForTable = useMemo(() => {
    if (!school) return null;
    return {
      id: school.id,
      organizationId: school.organizationId,
      name: school.name,
      slug: school.slug,
      subdomain: school.subdomain,
      address: school.address,
      city: school.city,
      zip: school.zip,
      phone: school.phone,
      email: school.email,
      active: school.active,
      hidden: school.hidden,
      deletedAt: school.deletedAt,
      createdAt: school.createdAt,
      updatedAt: school.updatedAt,
    };
  }, [school]);

  if ((!organizations || schoolsLoading) && !school) {
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

  if (!school) {
    logger.warn("school_not_found_by_slug", { slug, schoolSlug });
    return (
      <div className="w-full">
        <Link
          href={`/operator/organizations/${encodeURIComponent(slug)}`}
          className="text-sm text-gray-600 hover:text-gray-900"
        >
          ← Zurück zu {organization.name}
        </Link>
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 text-center">
          <p className="text-gray-600">Schule nicht gefunden.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      <Link
        href={`/operator/organizations/${encodeURIComponent(slug)}`}
        className="mb-4 inline-flex items-center gap-2 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span aria-hidden>←</span>
        <span>Zurück zu {organization.name}</span>
      </Link>

      <EntityHeaderCard
        title={school.name}
        subdomain={school.subdomain}
        active={school.active}
        createdAt={school.createdAt}
        subtitle={
          <span>
            Träger:{" "}
            <span className="font-medium text-gray-800">
              {organization.name}
            </span>
          </span>
        }
        stats={[
          { label: "Konten", value: numberFormat(school.kontenCount) },
          { label: "Geräte", value: numberFormat(school.geraeteCount) },
          { label: "Personen", value: numberFormat(school.personenCount) },
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

          <TabsPrimitive.Content value="konten" className="mt-4">
            {accountsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolAccounts && schoolAccounts.length > 0 ? (
              <AccountsTable
                accounts={schoolAccounts}
                selectedSchool={selectedSchoolForTable}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Konten für diese Schule.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="geraete" className="mt-4">
            {devicesLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolDevices && schoolDevices.length > 0 ? (
              <DevicesTable devices={schoolDevices} />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Geräte für diese Schule.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="personen" className="mt-4">
            {personsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolPersons && schoolPersons.length > 0 ? (
              <PersonsTable persons={schoolPersons} />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Personen für diese Schule.
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
    </div>
  );
}
