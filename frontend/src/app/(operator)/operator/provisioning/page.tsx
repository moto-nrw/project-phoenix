"use client";

import { useState, useMemo, useCallback } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages use useOperatorAuth, not NextAuth
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Modal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { generateSlug, isValidSlug } from "~/lib/operator/provisioning-helpers";
import type {
  Organization,
  School,
  Invitation,
} from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "OperatorProvisioningPage" });

type ActiveTab = "organizations" | "schools";

export default function OperatorProvisioningPage() {
  const { isAuthenticated } = useOperatorAuth();
  useSetBreadcrumb({ pageTitle: "Schulverwaltung" });

  const [activeTab, setActiveTab] = useState<ActiveTab>("organizations");
  const [createOrgOpen, setCreateOrgOpen] = useState(false);
  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteSchoolId, setInviteSchoolId] = useState<string | null>(null);
  const [inviteSchoolName, setInviteSchoolName] = useState("");

  // Organization form state
  const [orgName, setOrgName] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [orgSlugManual, setOrgSlugManual] = useState(false);
  const [orgSaving, setOrgSaving] = useState(false);
  const [orgError, setOrgError] = useState("");

  // School form state
  const [schoolOrgId, setSchoolOrgId] = useState("");
  const [schoolName, setSchoolName] = useState("");
  const [schoolSlug, setSchoolSlug] = useState("");
  const [schoolSlugManual, setSchoolSlugManual] = useState(false);
  const [schoolSubdomain, setSchoolSubdomain] = useState("");
  const [schoolSubdomainManual, setSchoolSubdomainManual] = useState(false);
  const [schoolAddress, setSchoolAddress] = useState("");
  const [schoolCity, setSchoolCity] = useState("");
  const [schoolZip, setSchoolZip] = useState("");
  const [schoolPhone, setSchoolPhone] = useState("");
  const [schoolEmail, setSchoolEmail] = useState("");
  const [schoolSaving, setSchoolSaving] = useState(false);
  const [schoolError, setSchoolError] = useState("");

  // Invite form state
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteFirstName, setInviteFirstName] = useState("");
  const [inviteLastName, setInviteLastName] = useState("");
  const [invitePosition, setInvitePosition] = useState("");
  const [inviteSaving, setInviteSaving] = useState(false);
  const [inviteError, setInviteError] = useState("");
  const [inviteResult, setInviteResult] = useState<Invitation | null>(null);

  const {
    data: organizations,
    isLoading: orgsLoading,
    mutate: mutateOrgs,
  } = useSWR(
    isAuthenticated ? "operator-organizations" : null,
    () => operatorProvisioningService.listOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: schools,
    isLoading: schoolsLoading,
    mutate: mutateSchools,
  } = useSWR(
    isAuthenticated ? "operator-schools" : null,
    () => operatorProvisioningService.listSchools(),
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
          id: "organizations",
          label: "Träger",
          count: organizations?.length,
        },
        { id: "schools", label: "Schulen", count: schools?.length },
      ],
      activeTab,
      onTabChange: (tabId: string) => setActiveTab(tabId as ActiveTab),
    }),
    [activeTab, organizations?.length, schools?.length],
  );

  // Organization handlers
  const resetOrgForm = useCallback(() => {
    setOrgName("");
    setOrgSlug("");
    setOrgSlugManual(false);
    setOrgError("");
  }, []);

  const openCreateOrg = useCallback(() => {
    resetOrgForm();
    setCreateOrgOpen(true);
  }, [resetOrgForm]);

  const handleOrgNameChange = useCallback(
    (value: string) => {
      setOrgName(value);
      if (!orgSlugManual) {
        setOrgSlug(generateSlug(value));
      }
    },
    [orgSlugManual],
  );

  const handleCreateOrg = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!orgName.trim() || !orgSlug.trim()) return;
      if (!isValidSlug(orgSlug)) {
        setOrgError(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        );
        return;
      }
      setOrgSaving(true);
      setOrgError("");
      try {
        await operatorProvisioningService.createOrganization({
          name: orgName.trim(),
          slug: orgSlug.trim(),
        });
        setCreateOrgOpen(false);
        resetOrgForm();
        await mutateOrgs();
      } catch (error) {
        if (isOperatorApiError(error) && error.status === 409) {
          setOrgError("Ein Träger mit diesem Slug existiert bereits.");
        } else {
          setOrgError(
            error instanceof Error ? error.message : "Fehler beim Erstellen.",
          );
          logger.error("organization_create_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      } finally {
        setOrgSaving(false);
      }
    },
    [orgName, orgSlug, mutateOrgs, resetOrgForm],
  );

  // School handlers
  const resetSchoolForm = useCallback(() => {
    setSchoolOrgId("");
    setSchoolName("");
    setSchoolSlug("");
    setSchoolSlugManual(false);
    setSchoolSubdomain("");
    setSchoolSubdomainManual(false);
    setSchoolAddress("");
    setSchoolCity("");
    setSchoolZip("");
    setSchoolPhone("");
    setSchoolEmail("");
    setSchoolError("");
  }, []);

  const openCreateSchool = useCallback(() => {
    resetSchoolForm();
    // Pre-select first org if only one exists
    if (organizations?.length === 1) {
      setSchoolOrgId(organizations[0]!.id);
    }
    setCreateSchoolOpen(true);
  }, [resetSchoolForm, organizations]);

  const handleSchoolNameChange = useCallback(
    (value: string) => {
      setSchoolName(value);
      const slug = generateSlug(value);
      if (!schoolSlugManual) {
        setSchoolSlug(slug);
      }
      if (!schoolSubdomainManual) {
        setSchoolSubdomain(slug);
      }
    },
    [schoolSlugManual, schoolSubdomainManual],
  );

  const handleCreateSchool = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (
        !schoolOrgId ||
        !schoolName.trim() ||
        !schoolSlug.trim() ||
        !schoolSubdomain.trim()
      )
        return;
      if (!isValidSlug(schoolSlug)) {
        setSchoolError(
          "Slug darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        );
        return;
      }
      if (!isValidSlug(schoolSubdomain)) {
        setSchoolError(
          "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten.",
        );
        return;
      }
      setSchoolSaving(true);
      setSchoolError("");
      try {
        await operatorProvisioningService.createSchool({
          organization_id: parseInt(schoolOrgId, 10),
          name: schoolName.trim(),
          slug: schoolSlug.trim(),
          subdomain: schoolSubdomain.trim(),
          ...(schoolAddress && { address: schoolAddress.trim() }),
          ...(schoolCity && { city: schoolCity.trim() }),
          ...(schoolZip && { zip: schoolZip.trim() }),
          ...(schoolPhone && { phone: schoolPhone.trim() }),
          ...(schoolEmail && { email: schoolEmail.trim() }),
        });
        setCreateSchoolOpen(false);
        resetSchoolForm();
        await mutateSchools();
      } catch (error) {
        if (isOperatorApiError(error) && error.status === 409) {
          const msg = error.message.toLowerCase();
          if (msg.includes("subdomain")) {
            setSchoolError(
              "Eine Schule mit dieser Subdomain existiert bereits.",
            );
          } else {
            setSchoolError(
              "Eine Schule mit diesem Slug existiert bereits in dieser Organisation.",
            );
          }
        } else {
          setSchoolError(
            error instanceof Error ? error.message : "Fehler beim Erstellen.",
          );
          logger.error("school_create_failed", {
            error: error instanceof Error ? error.message : String(error),
          });
        }
      } finally {
        setSchoolSaving(false);
      }
    },
    [
      schoolOrgId,
      schoolName,
      schoolSlug,
      schoolSubdomain,
      schoolAddress,
      schoolCity,
      schoolZip,
      schoolPhone,
      schoolEmail,
      mutateSchools,
      resetSchoolForm,
    ],
  );

  // Invite handlers
  const resetInviteForm = useCallback(() => {
    setInviteEmail("");
    setInviteFirstName("");
    setInviteLastName("");
    setInvitePosition("");
    setInviteError("");
    setInviteResult(null);
  }, []);

  const openInviteAdmin = useCallback(
    (school: School) => {
      resetInviteForm();
      setInviteSchoolId(school.id);
      setInviteSchoolName(school.name);
      setInviteOpen(true);
    },
    [resetInviteForm],
  );

  const handleInvite = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      if (!inviteSchoolId || !inviteEmail.trim()) return;
      setInviteSaving(true);
      setInviteError("");
      try {
        const result = await operatorProvisioningService.inviteSchoolAdmin(
          inviteSchoolId,
          {
            email: inviteEmail.trim(),
            ...(inviteFirstName && { first_name: inviteFirstName.trim() }),
            ...(inviteLastName && { last_name: inviteLastName.trim() }),
            ...(invitePosition && { position: invitePosition.trim() }),
          },
        );
        setInviteResult(result);
      } catch (error) {
        setInviteError(
          error instanceof Error ? error.message : "Fehler beim Einladen.",
        );
        logger.error("admin_invite_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        setInviteSaving(false);
      }
    },
    [
      inviteSchoolId,
      inviteEmail,
      inviteFirstName,
      inviteLastName,
      invitePosition,
    ],
  );

  const isLoading =
    activeTab === "organizations" ? orgsLoading : schoolsLoading;

  const actionButton =
    activeTab === "organizations" ? (
      <button
        type="button"
        onClick={openCreateOrg}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neuer Träger
      </button>
    ) : (
      <button
        type="button"
        onClick={openCreateSchool}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neue Schule
      </button>
    );

  const mobileActionButton =
    activeTab === "organizations" ? (
      <button
        type="button"
        onClick={openCreateOrg}
        className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
        aria-label="Neuer Träger"
      >
        <PlusIcon />
      </button>
    ) : (
      <button
        type="button"
        onClick={openCreateSchool}
        className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
        aria-label="Neue Schule"
      >
        <PlusIcon />
      </button>
    );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {isLoading && <CardSkeletons />}

      {/* Organizations Tab */}
      {activeTab === "organizations" && !orgsLoading && (
        <>
          {organizations?.length === 0 && (
            <EmptyState
              title="Keine Träger"
              description="Erstellen Sie einen neuen Träger, um Schulen zu verwalten."
              buttonLabel="Neuer Träger"
              onAction={openCreateOrg}
            />
          )}
          {organizations && organizations.length > 0 && (
            <div className="mt-4 space-y-4">
              {organizations.map((org) => (
                <OrganizationCard key={org.id} organization={org} />
              ))}
            </div>
          )}
        </>
      )}

      {/* Schools Tab */}
      {activeTab === "schools" && !schoolsLoading && (
        <>
          {schools?.length === 0 && (
            <EmptyState
              title="Keine Schulen"
              description="Erstellen Sie eine neue Schule unter einem Träger."
              buttonLabel="Neue Schule"
              onAction={openCreateSchool}
            />
          )}
          {schools && schools.length > 0 && (
            <div className="mt-4 space-y-4">
              {schools.map((school) => (
                <SchoolCard
                  key={school.id}
                  school={school}
                  onInviteAdmin={openInviteAdmin}
                />
              ))}
            </div>
          )}
        </>
      )}

      {/* Create Organization Modal */}
      <Modal
        isOpen={createOrgOpen}
        onClose={() => setCreateOrgOpen(false)}
        title="Neuer Träger"
        footer={
          <>
            <button
              type="button"
              onClick={() => setCreateOrgOpen(false)}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={(e) => void handleCreateOrg(e)}
              disabled={orgSaving || !orgName.trim() || !orgSlug.trim()}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {orgSaving ? "Wird erstellt..." : "Erstellen"}
            </button>
          </>
        }
      >
        <form
          onSubmit={(e) => void handleCreateOrg(e)}
          className="space-y-4"
          id="create-org-form"
        >
          <FormField label="Name" htmlFor="org-name" required>
            <input
              id="org-name"
              type="text"
              value={orgName}
              onChange={(e) => handleOrgNameChange(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <FormField label="Slug" htmlFor="org-slug" required>
            <input
              id="org-slug"
              type="text"
              value={orgSlug}
              onChange={(e) => {
                setOrgSlugManual(true);
                setOrgSlug(e.target.value);
              }}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
            <p className="mt-1 text-xs text-gray-500">
              URL-freundlicher Bezeichner (z.B. &quot;stadt-koeln&quot;)
            </p>
          </FormField>
          {orgError && <FormError message={orgError} />}
        </form>
      </Modal>

      {/* Create School Modal */}
      <Modal
        isOpen={createSchoolOpen}
        onClose={() => setCreateSchoolOpen(false)}
        title="Neue Schule"
        footer={
          <>
            <button
              type="button"
              onClick={() => setCreateSchoolOpen(false)}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={(e) => void handleCreateSchool(e)}
              disabled={
                schoolSaving ||
                !schoolOrgId ||
                !schoolName.trim() ||
                !schoolSlug.trim() ||
                !schoolSubdomain.trim()
              }
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {schoolSaving ? "Wird erstellt..." : "Erstellen"}
            </button>
          </>
        }
      >
        <form
          onSubmit={(e) => void handleCreateSchool(e)}
          className="space-y-4"
          id="create-school-form"
        >
          <FormField label="Träger" htmlFor="school-org" required>
            <select
              id="school-org"
              value={schoolOrgId}
              onChange={(e) => setSchoolOrgId(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            >
              <option value="">Träger auswählen...</option>
              {organizations?.map((org) => (
                <option key={org.id} value={org.id}>
                  {org.name}
                </option>
              ))}
            </select>
          </FormField>
          <FormField label="Name" htmlFor="school-name" required>
            <input
              id="school-name"
              type="text"
              value={schoolName}
              onChange={(e) => handleSchoolNameChange(e.target.value)}
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              required
            />
          </FormField>
          <div className="grid grid-cols-2 gap-4">
            <FormField label="Slug" htmlFor="school-slug" required>
              <input
                id="school-slug"
                type="text"
                value={schoolSlug}
                onChange={(e) => {
                  setSchoolSlugManual(true);
                  setSchoolSlug(e.target.value);
                }}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                required
              />
            </FormField>
            <FormField label="Subdomain" htmlFor="school-subdomain" required>
              <input
                id="school-subdomain"
                type="text"
                value={schoolSubdomain}
                onChange={(e) => {
                  setSchoolSubdomainManual(true);
                  setSchoolSubdomain(e.target.value);
                }}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                required
              />
            </FormField>
          </div>

          <div className="border-t border-gray-100 pt-4">
            <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
              Kontaktdaten (optional)
            </p>
            <div className="space-y-3">
              <FormField label="Adresse" htmlFor="school-address">
                <input
                  id="school-address"
                  type="text"
                  value={schoolAddress}
                  onChange={(e) => setSchoolAddress(e.target.value)}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <div className="grid grid-cols-2 gap-4">
                <FormField label="PLZ" htmlFor="school-zip">
                  <input
                    id="school-zip"
                    type="text"
                    value={schoolZip}
                    onChange={(e) => setSchoolZip(e.target.value)}
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                  />
                </FormField>
                <FormField label="Stadt" htmlFor="school-city">
                  <input
                    id="school-city"
                    type="text"
                    value={schoolCity}
                    onChange={(e) => setSchoolCity(e.target.value)}
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                  />
                </FormField>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormField label="Telefon" htmlFor="school-phone">
                  <input
                    id="school-phone"
                    type="tel"
                    value={schoolPhone}
                    onChange={(e) => setSchoolPhone(e.target.value)}
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                  />
                </FormField>
                <FormField label="E-Mail" htmlFor="school-email">
                  <input
                    id="school-email"
                    type="email"
                    value={schoolEmail}
                    onChange={(e) => setSchoolEmail(e.target.value)}
                    className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                  />
                </FormField>
              </div>
            </div>
          </div>
          {schoolError && <FormError message={schoolError} />}
        </form>
      </Modal>

      {/* Invite Admin Modal */}
      <Modal
        isOpen={inviteOpen}
        onClose={() => {
          setInviteOpen(false);
          setInviteResult(null);
        }}
        title={`Admin einladen — ${inviteSchoolName}`}
        footer={
          inviteResult ? (
            <button
              type="button"
              onClick={() => {
                setInviteOpen(false);
                setInviteResult(null);
              }}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
            >
              Schließen
            </button>
          ) : (
            <>
              <button
                type="button"
                onClick={() => setInviteOpen(false)}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
              >
                Abbrechen
              </button>
              <button
                type="button"
                onClick={(e) => void handleInvite(e)}
                disabled={inviteSaving || !inviteEmail.trim()}
                className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {inviteSaving ? "Wird gesendet..." : "Einladung senden"}
              </button>
            </>
          )
        }
      >
        {inviteResult ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2 rounded-lg bg-green-50 px-4 py-3">
              <svg
                className="h-5 w-5 text-green-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <span className="text-sm font-medium text-green-800">
                Einladung erstellt
              </span>
            </div>
            <div className="space-y-2 text-sm text-gray-600">
              <p>
                <span className="font-medium">E-Mail:</span>{" "}
                {inviteResult.email}
              </p>
              <p>
                <span className="font-medium">Status:</span>{" "}
                <DeliveryStatusBadge status={inviteResult.deliveryStatus} />
              </p>
              {inviteResult.emailError && (
                <p className="text-red-600">
                  <span className="font-medium">Fehler:</span>{" "}
                  {inviteResult.emailError}
                </p>
              )}
            </div>
          </div>
        ) : (
          <form
            onSubmit={(e) => void handleInvite(e)}
            className="space-y-4"
            id="invite-admin-form"
          >
            <FormField label="E-Mail" htmlFor="invite-email" required>
              <input
                id="invite-email"
                type="email"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                required
              />
            </FormField>
            <div className="grid grid-cols-2 gap-4">
              <FormField label="Vorname" htmlFor="invite-first-name">
                <input
                  id="invite-first-name"
                  type="text"
                  value={inviteFirstName}
                  onChange={(e) => setInviteFirstName(e.target.value)}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
              <FormField label="Nachname" htmlFor="invite-last-name">
                <input
                  id="invite-last-name"
                  type="text"
                  value={inviteLastName}
                  onChange={(e) => setInviteLastName(e.target.value)}
                  className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                />
              </FormField>
            </div>
            <FormField label="Position" htmlFor="invite-position">
              <input
                id="invite-position"
                type="text"
                value={invitePosition}
                onChange={(e) => setInvitePosition(e.target.value)}
                placeholder="z.B. Schulleiter"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
              />
            </FormField>
            {inviteError && <FormError message={inviteError} />}
          </form>
        )}
      </Modal>
    </div>
  );
}

// --- Sub-components ---

function OrganizationCard({
  organization,
}: {
  readonly organization: Organization;
}) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            {organization.name}
          </h3>
          <p className="mt-0.5 font-mono text-sm text-gray-500">
            {organization.slug}
          </p>
        </div>
        <StatusBadge active={organization.active} />
      </div>
      <p className="mt-2 text-xs text-gray-400">
        Erstellt {getRelativeTime(organization.createdAt)}
      </p>
    </div>
  );
}

function SchoolCard({
  school,
  onInviteAdmin,
}: {
  readonly school: School;
  readonly onInviteAdmin: (school: School) => void;
}) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">
            {school.name}
          </h3>
          <div className="mt-0.5 flex items-center gap-2 text-sm text-gray-500">
            <span className="font-mono">{school.subdomain}</span>
            {school.organization && (
              <>
                <span className="text-gray-300">·</span>
                <span>{school.organization.name}</span>
              </>
            )}
          </div>
        </div>
        <StatusBadge active={school.active} />
      </div>

      {(school.address || school.city) && (
        <p className="mt-2 text-xs text-gray-400">
          {[school.address, school.zip, school.city].filter(Boolean).join(", ")}
        </p>
      )}

      <div className="mt-3 flex items-center justify-between">
        <p className="text-xs text-gray-400">
          Erstellt {getRelativeTime(school.createdAt)}
        </p>
        <button
          type="button"
          onClick={() => onInviteAdmin(school)}
          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
        >
          Admin einladen
        </button>
      </div>
    </div>
  );
}

function StatusBadge({ active }: { readonly active: boolean }) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
        active ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
      }`}
    >
      {active ? "Aktiv" : "Inaktiv"}
    </span>
  );
}

function DeliveryStatusBadge({ status }: { readonly status: string }) {
  const styles: Record<string, string> = {
    sent: "bg-green-100 text-green-700",
    pending: "bg-yellow-100 text-yellow-700",
    failed: "bg-red-100 text-red-700",
  };
  const labels: Record<string, string> = {
    sent: "Gesendet",
    pending: "Ausstehend",
    failed: "Fehlgeschlagen",
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-500"}`}
    >
      {labels[status] ?? status}
    </span>
  );
}

function EmptyState({
  title,
  description,
  buttonLabel,
  onAction,
}: {
  readonly title: string;
  readonly description: string;
  readonly buttonLabel: string;
  readonly onAction: () => void;
}) {
  return (
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
          d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21"
        />
      </svg>
      <p className="text-lg font-medium text-gray-900">{title}</p>
      <p className="text-sm text-gray-500">{description}</p>
      <button
        type="button"
        onClick={onAction}
        className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        {buttonLabel}
      </button>
    </div>
  );
}

function FormField({
  label,
  htmlFor,
  required,
  children,
}: {
  readonly label: string;
  readonly htmlFor: string;
  readonly required?: boolean;
  readonly children: React.ReactNode;
}) {
  return (
    <div>
      <label
        htmlFor={htmlFor}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
        {required && <span className="ml-0.5 text-red-500">*</span>}
      </label>
      {children}
    </div>
  );
}

function FormError({ message }: { readonly message: string }) {
  return (
    <div className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
      {message}
    </div>
  );
}

function PlusIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
    </svg>
  );
}

function CardSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]"
        >
          <div className="space-y-3">
            <Skeleton className="h-5 w-3/5 rounded" />
            <Skeleton className="h-4 w-2/5 rounded" />
            <Skeleton className="h-3 w-1/4 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
