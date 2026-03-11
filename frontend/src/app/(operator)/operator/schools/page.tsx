"use client";

import { useState, useMemo, useCallback, useRef, useEffect } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages use useOperatorAuth, not NextAuth
import useSWR from "swr";
import { MoreVertical } from "lucide-react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Skeleton } from "~/components/ui/skeleton";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { School } from "~/lib/operator/provisioning-helpers";
import { CreateOrganizationModal } from "~/components/operator/create-organization-modal";
import { CreateSchoolModal } from "~/components/operator/create-school-modal";
import { InviteAdminModal } from "~/components/operator/invite-admin-modal";
export default function OperatorSchoolsPage() {
  const { isAuthenticated } = useOperatorAuth();
  useSetBreadcrumb({ pageTitle: "Schulen" });

  const [orgModalOpen, setOrgModalOpen] = useState(false);
  const [schoolModalOpen, setSchoolModalOpen] = useState(false);
  const [inviteTarget, setInviteTarget] = useState<School | null>(null);

  const {
    data: organizations,
    isLoading: isLoadingOrgs,
    mutate: mutateOrgs,
  } = useSWR(
    isAuthenticated ? "operator-organizations" : null,
    () => operatorProvisioningService.fetchOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: schools,
    isLoading: isLoadingSchools,
    mutate: mutateSchools,
  } = useSWR(
    isAuthenticated ? "operator-schools" : null,
    () => operatorProvisioningService.fetchSchools(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const isLoading = isLoadingOrgs || isLoadingSchools;

  // Group schools by organization
  const schoolsByOrg = useMemo(() => {
    if (!schools || !organizations) return new Map<string, School[]>();
    const map = new Map<string, School[]>();
    for (const org of organizations) {
      map.set(
        org.id,
        schools.filter((s) => s.organizationId === org.id),
      );
    }
    return map;
  }, [schools, organizations]);

  const handleOrgCreated = useCallback(() => {
    void mutateOrgs();
  }, [mutateOrgs]);

  const handleSchoolCreated = useCallback(() => {
    void mutateSchools();
  }, [mutateSchools]);

  const handleInvited = useCallback(() => {
    // No school list refresh needed, just close modal
  }, []);

  const openSchoolModal = useCallback(() => {
    setSchoolModalOpen(true);
  }, []);

  const openOrgModal = useCallback(() => {
    setOrgModalOpen(true);
  }, []);

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Schulen"
        badge={schools ? { count: schools.length, label: "Gesamt" } : undefined}
        actionButton={
          <button
            type="button"
            onClick={openSchoolModal}
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Neue Schule
          </button>
        }
        mobileActionButton={
          <button
            type="button"
            onClick={openSchoolModal}
            className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
            aria-label="Neue Schule"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 4v16m8-8H4"
              />
            </svg>
          </button>
        }
      />

      {isLoading && <SchoolSkeletons />}

      {!isLoading && (!schools || schools.length === 0) && (
        <div className="flex flex-col items-center gap-3 py-12 text-center">
          <svg
            className="h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75z"
            />
          </svg>
          <p className="text-lg font-medium text-gray-900">Keine Schulen</p>
          <p className="text-sm text-gray-500">
            Erstellen Sie eine neue Schule, um loszulegen.
          </p>
          <button
            type="button"
            onClick={openSchoolModal}
            className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Neue Schule
          </button>
        </div>
      )}

      {!isLoading && schools && schools.length > 0 && organizations && (
        <div className="mt-4 space-y-6">
          {organizations.map((org) => {
            const orgSchools = schoolsByOrg.get(org.id) ?? [];
            if (orgSchools.length === 0) return null;
            return (
              <div key={org.id}>
                <div className="mb-3 flex items-center gap-2">
                  <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
                    {org.name}
                  </h2>
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                    {orgSchools.length}
                  </span>
                </div>
                <div className="space-y-3">
                  {orgSchools.map((school) => (
                    <SchoolCard
                      key={school.id}
                      school={school}
                      onInviteAdmin={() => setInviteTarget(school)}
                    />
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Modals */}
      <CreateOrganizationModal
        isOpen={orgModalOpen}
        onClose={() => setOrgModalOpen(false)}
        onCreated={handleOrgCreated}
      />

      <CreateSchoolModal
        isOpen={schoolModalOpen}
        onClose={() => setSchoolModalOpen(false)}
        onCreated={handleSchoolCreated}
        organizations={organizations ?? []}
        onCreateOrganization={openOrgModal}
      />

      <InviteAdminModal
        isOpen={!!inviteTarget}
        onClose={() => setInviteTarget(null)}
        school={inviteTarget}
        onInvited={handleInvited}
      />
    </div>
  );
}

function SchoolCard({
  school,
  onInviteAdmin,
}: {
  readonly school: School;
  readonly onInviteAdmin: () => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu on click outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    if (menuOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () =>
        document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [menuOpen]);

  return (
    <div className="relative rounded-3xl border border-gray-100/50 bg-white/90 p-5 pr-12 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      {/* Kebab menu */}
      <div className="absolute top-3 right-3" ref={menuRef}>
        <button
          type="button"
          onClick={() => setMenuOpen(!menuOpen)}
          className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
          aria-label="Menü öffnen"
        >
          <MoreVertical className="h-5 w-5" />
        </button>

        {menuOpen && (
          <div className="absolute top-full right-0 z-50 mt-1 w-44 rounded-xl border border-gray-100 bg-white py-1 shadow-lg">
            <button
              type="button"
              onClick={() => {
                setMenuOpen(false);
                onInviteAdmin();
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
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
                  d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75"
                />
              </svg>
              Admin einladen
            </button>
          </div>
        )}
      </div>

      {/* School name + subdomain */}
      <h3 className="text-base font-semibold text-gray-900">{school.name}</h3>
      <div className="mt-1 flex items-center gap-2">
        <span className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
          {school.subdomain}.moto-app.de
        </span>
        <span className="text-xs text-gray-400">·</span>
        <span className="text-xs text-gray-500">{school.slug}</span>
      </div>

      {/* Address line */}
      {(school.address ?? school.city) && (
        <p className="mt-2 text-sm text-gray-500">
          {[school.address, [school.zip, school.city].filter(Boolean).join(" ")]
            .filter(Boolean)
            .join(", ")}
        </p>
      )}

      {/* Contact line */}
      {(school.phone ?? school.email) && (
        <div className="mt-1 flex items-center gap-3 text-xs text-gray-400">
          {school.phone && <span>{school.phone}</span>}
          {school.email && <span>{school.email}</span>}
        </div>
      )}
    </div>
  );
}

function SchoolSkeletons() {
  return (
    <div className="mt-4 space-y-6">
      {Array.from({ length: 2 }, (_, i) => (
        <div key={i}>
          <Skeleton className="mb-3 h-4 w-40 rounded" />
          <div className="space-y-3">
            {Array.from({ length: 2 }, (_, j) => (
              <div
                key={j}
                className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]"
              >
                <div className="space-y-3">
                  <Skeleton className="h-5 w-3/5 rounded" />
                  <div className="flex gap-2">
                    <Skeleton className="h-5 w-36 rounded-full" />
                    <Skeleton className="h-5 w-20 rounded" />
                  </div>
                  <Skeleton className="h-4 w-2/3 rounded" />
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
