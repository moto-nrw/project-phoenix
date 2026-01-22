"use client";

import { useCallback, useEffect, useState } from "react";
import { Input } from "~/components/ui";

// Role response from console API
interface ConsoleRole {
  id: number;
  name: string;
  description: string;
  displayName: string;
}

interface ConsoleRolesResponse {
  status: string;
  data: ConsoleRole[];
}

// Atomic provision API types
interface ProvisionInvitation {
  email: string;
  role: "admin" | "member" | "owner";
  firstName?: string;
  lastName?: string;
}

interface ProvisionRequest {
  orgName: string;
  orgSlug: string;
  invitations: ProvisionInvitation[];
}

interface ProvisionErrorResponse {
  error: string;
  field?: string;
  unavailableEmails?: string[];
}

interface ProvisionSuccessResponse {
  success: boolean;
  organization: {
    id: string;
    name: string;
    slug: string;
    status: string;
    createdAt: string;
  };
  invitations: Array<{
    id: string;
    email: string;
    role: string;
  }>;
}

// Fetch roles from console API (uses BetterAuth auth, not Go backend JWT)
async function fetchConsoleRoles(): Promise<ConsoleRole[]> {
  const response = await fetch("/api/console/roles", {
    credentials: "include",
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch roles: ${response.status}`);
  }

  const result = (await response.json()) as ConsoleRolesResponse;
  return result.data;
}

// Map Go role IDs to BetterAuth role names
function mapRoleIdToName(roleId: number): "admin" | "member" | "owner" {
  // Role IDs from Go backend: 1=Admin, 2=Lehrer, 3=Betreuer, etc.
  // For org provisioning, we use BetterAuth roles: admin, member, owner
  // First invitation should typically be admin (org admin)
  switch (roleId) {
    case 1: // Admin
      return "admin";
    case 2: // Lehrer
    case 3: // Betreuer
    default:
      return "member";
  }
}

/**
 * Atomic organization provisioning via BetterAuth.
 * Creates organization AND invitations in a single atomic operation.
 * If any validation fails (slug taken, emails registered), nothing is created.
 */
async function provisionOrganization(
  data: ProvisionRequest,
): Promise<ProvisionSuccessResponse> {
  const response = await fetch("/api/admin/organizations/provision", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  const result = (await response.json()) as
    | ProvisionSuccessResponse
    | ProvisionErrorResponse;

  if (!response.ok) {
    const errorResult = result as ProvisionErrorResponse;
    throw new Error(errorResult.error ?? "Failed to provision organization");
  }

  return result as ProvisionSuccessResponse;
}

// ============================================================================
// Types
// ============================================================================

interface RoleOption {
  id: number;
  name: string;
  displayName: string;
}

interface InvitationEntry {
  id: string;
  email: string;
  roleId: number | undefined;
  firstName: string;
  lastName: string;
}

interface OrganizationInviteFormProps {
  readonly onSuccess?: (orgName: string, inviteCount: number) => void;
}

// ============================================================================
// Icons
// ============================================================================

function PlusIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 4.5v15m7.5-7.5h-15"
      />
    </svg>
  );
}

function TrashIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"
      />
    </svg>
  );
}

function CheckCircleIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z"
      />
    </svg>
  );
}

function ExclamationIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      strokeWidth={2}
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z"
      />
    </svg>
  );
}

// ============================================================================
// Helper Functions
// ============================================================================

function generateId(): string {
  return Math.random().toString(36).substring(2, 9);
}

function createEmptyInvitation(): InvitationEntry {
  return {
    id: generateId(),
    email: "",
    roleId: undefined,
    firstName: "",
    lastName: "",
  };
}

// ============================================================================
// Component
// ============================================================================

// Helper to generate slug from name
function generateSlugFromName(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[äÄ]/g, "ae")
    .replace(/[öÖ]/g, "oe")
    .replace(/[üÜ]/g, "ue")
    .replace(/[ß]/g, "ss")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function OrganizationInviteForm({
  onSuccess,
}: OrganizationInviteFormProps) {
  // Form state
  const [orgName, setOrgName] = useState("");
  const [orgSlug, setOrgSlug] = useState("");
  const [autoGenerateSlug, setAutoGenerateSlug] = useState(true);
  const [invitations, setInvitations] = useState<InvitationEntry[]>([
    createEmptyInvitation(),
  ]);

  // Auto-generate slug when name changes (if auto-generate is enabled)
  useEffect(() => {
    if (autoGenerateSlug && orgName) {
      setOrgSlug(generateSlugFromName(orgName));
    }
  }, [orgName, autoGenerateSlug]);

  // UI state
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<{
    orgName: string;
    orgSlug: string;
    inviteCount: number;
    inviteEmails: string[];
  } | null>(null);

  // Load roles from console API (uses BetterAuth auth)
  useEffect(() => {
    let cancelled = false;

    async function loadRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await fetchConsoleRoles();
        if (cancelled) return;

        const options = roleList.map<RoleOption>((role) => ({
          id: role.id,
          name: role.name,
          displayName: role.displayName,
        }));

        setRoles(options);
      } catch (err) {
        console.error("Failed to load roles", err);
        if (!cancelled) {
          setError(
            "Rollen konnten nicht geladen werden. Bitte aktualisiere die Seite.",
          );
        }
      } finally {
        if (!cancelled) {
          setIsLoadingRoles(false);
        }
      }
    }

    void loadRoles();
    return () => {
      cancelled = true;
    };
  }, []);

  // Update invitation entry
  const updateInvitation = useCallback(
    (
      id: string,
      field: keyof InvitationEntry,
      value: string | number | undefined,
    ) => {
      setInvitations((prev) =>
        prev.map((inv) => (inv.id === id ? { ...inv, [field]: value } : inv)),
      );
    },
    [],
  );

  // Add new invitation entry
  const addInvitation = useCallback(() => {
    setInvitations((prev) => [...prev, createEmptyInvitation()]);
  }, []);

  // Remove invitation entry
  const removeInvitation = useCallback((id: string) => {
    setInvitations((prev) => {
      if (prev.length <= 1) return prev;
      return prev.filter((inv) => inv.id !== id);
    });
  }, []);

  // Validate form
  const validateForm = useCallback((): string | null => {
    if (!orgName.trim()) {
      return "Bitte gib einen Organisationsnamen ein.";
    }

    // Validate slug format (only lowercase letters, numbers, and hyphens)
    const trimmedSlug = orgSlug.trim();
    if (
      trimmedSlug &&
      !/^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$/.test(trimmedSlug)
    ) {
      return "Die Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten und muss mit einem Buchstaben oder einer Zahl beginnen/enden.";
    }

    const validInvitations = invitations.filter((inv) => inv.email.trim());
    if (validInvitations.length === 0) {
      return "Bitte füge mindestens eine E-Mail-Adresse hinzu.";
    }

    for (const inv of validInvitations) {
      if (!inv.roleId || inv.roleId <= 0) {
        return `Bitte wähle eine Rolle für ${inv.email} aus.`;
      }
    }

    return null;
  }, [orgName, orgSlug, invitations]);

  // Submit form - uses atomic provisioning endpoint
  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    try {
      setIsSubmitting(true);

      // Filter valid invitations
      const validInvitations = invitations.filter((inv) => inv.email.trim());

      // Build provision request
      const provisionData: ProvisionRequest = {
        orgName: orgName.trim(),
        orgSlug: orgSlug.trim(),
        invitations: validInvitations.map((inv) => ({
          email: inv.email.trim(),
          role: mapRoleIdToName(inv.roleId ?? 1),
          firstName: inv.firstName.trim() || undefined,
          lastName: inv.lastName.trim() || undefined,
        })),
      };

      // Single atomic API call - creates org AND invitations together
      // If anything fails (slug taken, email registered), nothing is created
      const result = await provisionOrganization(provisionData);

      // Success! Invitations are sent via email by the backend
      setSuccess({
        orgName: result.organization.name,
        orgSlug: result.organization.slug,
        inviteCount: result.invitations.length,
        inviteEmails: result.invitations.map((inv) => inv.email),
      });

      // Reset form
      setOrgName("");
      setOrgSlug("");
      setAutoGenerateSlug(true);
      setInvitations([createEmptyInvitation()]);

      if (onSuccess) {
        onSuccess(result.organization.name, result.invitations.length);
      }
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Ein Fehler ist aufgetreten.";
      setError(message);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Reset success state
  const handleNewOrganization = useCallback(() => {
    setSuccess(null);
    setError(null);
  }, []);

  // Render success state
  if (success) {
    return (
      <div className="rounded-2xl border border-green-200 bg-green-50 p-6">
        <div className="flex items-start gap-4">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-full bg-green-100">
            <CheckCircleIcon className="size-6 text-green-600" />
          </div>
          <div className="flex-1">
            <h3 className="text-lg font-semibold text-green-800">
              Organisation erfolgreich erstellt!
            </h3>
            <p className="mt-1 text-sm text-green-700">
              <strong>{success.orgName}</strong> wurde erstellt und{" "}
              {success.inviteCount === 1
                ? "1 Einladung wurde"
                : `${success.inviteCount} Einladungen wurden`}{" "}
              versendet.
            </p>
            {success.orgSlug && (
              <p className="mt-2 text-sm text-green-600">
                Subdomain:{" "}
                <span className="font-mono font-medium">
                  {success.orgSlug}.moto.nrw
                </span>
              </p>
            )}

            {success.inviteEmails.length > 0 && (
              <div className="mt-4">
                <p className="mb-2 text-sm font-medium text-green-800">
                  Einladungen gesendet an:
                </p>
                <div className="space-y-2">
                  {success.inviteEmails.map((email, idx) => (
                    <div
                      key={idx}
                      className="rounded-lg border border-green-200 bg-white px-3 py-2"
                    >
                      <p className="text-sm text-gray-700">{email}</p>
                    </div>
                  ))}
                </div>
                <p className="mt-3 text-xs text-green-600">
                  Die Eingeladenen erhalten eine E-Mail mit einem Link zur
                  Registrierung.
                </p>
              </div>
            )}

            <button
              type="button"
              onClick={handleNewOrganization}
              className="mt-6 rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-green-700"
            >
              Weitere Organisation anlegen
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
      <div className="mb-6">
        <h2 className="text-lg font-semibold text-gray-900">
          Neue Organisation anlegen
        </h2>
        <p className="mt-1 text-sm text-gray-600">
          Erstelle eine Organisation und lade Administratoren per E-Mail ein.
        </p>
      </div>

      <form onSubmit={handleSubmit} noValidate className="space-y-6">
        {/* Error message */}
        {error && (
          <div className="flex items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4">
            <ExclamationIcon className="mt-0.5 size-5 shrink-0 text-red-600" />
            <p className="text-sm text-red-700">{error}</p>
          </div>
        )}

        {/* Organization name */}
        <div>
          <Input
            id="org-name"
            name="orgName"
            label="Organisationsname"
            placeholder="z.B. OGS Musterstadt"
            value={orgName}
            onChange={(e) => setOrgName(e.target.value)}
            disabled={isSubmitting}
            required
          />
        </div>

        {/* Organization slug (subdomain) */}
        <div>
          <div className="mb-1 flex items-center justify-between">
            <label
              htmlFor="org-slug"
              className="block text-sm font-medium text-gray-700"
            >
              Subdomain (URL-Slug)
            </label>
            <label className="flex items-center gap-2 text-sm text-gray-500">
              <input
                type="checkbox"
                checked={autoGenerateSlug}
                onChange={(e) => {
                  setAutoGenerateSlug(e.target.checked);
                  if (e.target.checked && orgName) {
                    setOrgSlug(generateSlugFromName(orgName));
                  }
                }}
                className="rounded border-gray-300 text-gray-900 focus:ring-gray-500"
              />
              Automatisch generieren
            </label>
          </div>
          <div className="flex items-center">
            <input
              type="text"
              id="org-slug"
              value={orgSlug}
              onChange={(e) => {
                setAutoGenerateSlug(false);
                setOrgSlug(
                  e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""),
                );
              }}
              disabled={isSubmitting}
              placeholder="z.B. ogs-musterstadt"
              className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:bg-gray-100"
            />
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Die Subdomain wird für die URL der Organisation verwendet (z.B.{" "}
            <span className="font-mono">{orgSlug || "subdomain"}.moto.nrw</span>
            )
          </p>
        </div>

        {/* Invitations section */}
        <div>
          <div className="mb-3 flex items-center justify-between">
            <label className="block text-sm font-medium text-gray-700">
              Einladungen
            </label>
            <button
              type="button"
              onClick={addInvitation}
              disabled={isSubmitting}
              className="inline-flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:opacity-50"
            >
              <PlusIcon className="size-4" />
              Hinzufugen
            </button>
          </div>

          <div className="space-y-4">
            {invitations.map((inv, index) => (
              <div
                key={inv.id}
                className="rounded-xl border border-gray-200 bg-gray-50/50 p-4"
              >
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-sm font-medium text-gray-500">
                    Einladung {index + 1}
                  </span>
                  {invitations.length > 1 && (
                    <button
                      type="button"
                      onClick={() => removeInvitation(inv.id)}
                      disabled={isSubmitting}
                      className="rounded p-1 text-gray-400 transition-colors hover:bg-gray-200 hover:text-gray-600 disabled:opacity-50"
                    >
                      <TrashIcon className="size-4" />
                    </button>
                  )}
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  {/* Email */}
                  <div className="sm:col-span-2">
                    <label
                      htmlFor={`email-${inv.id}`}
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      E-Mail-Adresse
                    </label>
                    <input
                      type="email"
                      id={`email-${inv.id}`}
                      value={inv.email}
                      onChange={(e) =>
                        updateInvitation(inv.id, "email", e.target.value)
                      }
                      disabled={isSubmitting}
                      placeholder="email@example.com"
                      className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:bg-gray-100"
                    />
                  </div>

                  {/* Role */}
                  <div>
                    <label
                      htmlFor={`role-${inv.id}`}
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Rolle
                    </label>
                    <select
                      id={`role-${inv.id}`}
                      value={inv.roleId ?? ""}
                      onChange={(e) =>
                        updateInvitation(
                          inv.id,
                          "roleId",
                          Number(e.target.value) || undefined,
                        )
                      }
                      disabled={isSubmitting || isLoadingRoles}
                      className="w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:bg-gray-100"
                    >
                      <option value="" disabled>
                        Rolle auswahlen...
                      </option>
                      {roles.map((role) => (
                        <option key={role.id} value={role.id}>
                          {role.displayName}
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* First name */}
                  <div>
                    <label
                      htmlFor={`firstName-${inv.id}`}
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Vorname (optional)
                    </label>
                    <input
                      type="text"
                      id={`firstName-${inv.id}`}
                      value={inv.firstName}
                      onChange={(e) =>
                        updateInvitation(inv.id, "firstName", e.target.value)
                      }
                      disabled={isSubmitting}
                      className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:bg-gray-100"
                    />
                  </div>

                  {/* Last name */}
                  <div>
                    <label
                      htmlFor={`lastName-${inv.id}`}
                      className="mb-1 block text-sm font-medium text-gray-700"
                    >
                      Nachname (optional)
                    </label>
                    <input
                      type="text"
                      id={`lastName-${inv.id}`}
                      value={inv.lastName}
                      onChange={(e) =>
                        updateInvitation(inv.id, "lastName", e.target.value)
                      }
                      disabled={isSubmitting}
                      className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:bg-gray-100"
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Submit button */}
        <button
          type="submit"
          disabled={isSubmitting || isLoadingRoles}
          className="w-full rounded-xl bg-gray-900 py-3 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting
            ? "Wird erstellt..."
            : `Organisation erstellen & ${invitations.filter((i) => i.email.trim()).length || 1} Einladung${invitations.filter((i) => i.email.trim()).length === 1 ? "" : "en"} senden`}
        </button>
      </form>
    </div>
  );
}
