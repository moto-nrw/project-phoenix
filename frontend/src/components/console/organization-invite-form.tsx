"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Input } from "~/components/ui";
import { createOrganization } from "~/lib/admin-api";

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

// Invitation response from console API
interface ConsoleInvitationResponse {
  status: string;
  data?: {
    id: number;
    email: string;
    role_id: number;
    token: string;
    expires_at: string;
    first_name?: string;
    last_name?: string;
    position?: string;
  };
  error?: string;
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

// Create invitation via console API (uses BetterAuth auth, not Go backend JWT)
// This calls the Go backend's internal API via the console endpoint
interface CreateConsoleInvitationRequest {
  email: string;
  role_id: number;
  first_name?: string;
  last_name?: string;
  position?: string;
}

interface CreateConsoleInvitationResult {
  token: string;
  id: number;
  email: string;
}

async function createConsoleInvitation(
  data: CreateConsoleInvitationRequest,
): Promise<CreateConsoleInvitationResult> {
  const response = await fetch("/api/console/invitations", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    const result = (await response.json()) as ConsoleInvitationResponse;
    throw new Error(result.error ?? "Failed to create invitation");
  }

  const result = (await response.json()) as ConsoleInvitationResponse;
  if (!result.data) {
    throw new Error("Invalid response: no data");
  }

  return {
    token: result.data.token,
    id: result.data.id,
    email: result.data.email,
  };
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

export function OrganizationInviteForm({
  onSuccess,
}: OrganizationInviteFormProps) {
  // Form state
  const [orgName, setOrgName] = useState("");
  const [invitations, setInvitations] = useState<InvitationEntry[]>([
    createEmptyInvitation(),
  ]);

  // UI state
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<{
    orgName: string;
    inviteCount: number;
    inviteLinks: string[];
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

  // Get base URL for invite links
  const inviteBaseUrl = useMemo(() => {
    if (typeof globalThis !== "undefined" && "location" in globalThis) {
      return globalThis.location.origin;
    }
    return "";
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
  }, [orgName, invitations]);

  // Submit form
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

      // Step 1: Create organization with active status
      const org = await createOrganization({ name: orgName.trim() });

      // Step 2: Send invitations via Go backend's internal API (console endpoint)
      const validInvitations = invitations.filter((inv) => inv.email.trim());
      const inviteLinks: string[] = [];
      const failedInvites: string[] = [];

      for (const inv of validInvitations) {
        try {
          const inviteData: CreateConsoleInvitationRequest = {
            email: inv.email.trim(),
            role_id: inv.roleId ?? 0,
            first_name: inv.firstName.trim() || undefined,
            last_name: inv.lastName.trim() || undefined,
          };

          const invitation = await createConsoleInvitation(inviteData);
          const link = inviteBaseUrl
            ? `${inviteBaseUrl}/invite?token=${encodeURIComponent(invitation.token ?? "")}`
            : (invitation.token ?? "");
          inviteLinks.push(link);
        } catch (invErr) {
          console.error(`Failed to send invitation to ${inv.email}:`, invErr);
          failedInvites.push(inv.email);
        }
      }

      if (
        failedInvites.length > 0 &&
        failedInvites.length === validInvitations.length
      ) {
        // All invitations failed
        setError(
          `Organisation "${org.name}" wurde erstellt, aber alle Einladungen sind fehlgeschlagen. Bitte versuche es erneut.`,
        );
        return;
      }

      if (failedInvites.length > 0) {
        // Some invitations failed
        setError(
          `Einige Einladungen konnten nicht gesendet werden: ${failedInvites.join(", ")}`,
        );
      }

      // Success!
      setSuccess({
        orgName: org.name,
        inviteCount: inviteLinks.length,
        inviteLinks,
      });

      // Reset form
      setOrgName("");
      setInvitations([createEmptyInvitation()]);

      if (onSuccess) {
        onSuccess(org.name, inviteLinks.length);
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

            {success.inviteLinks.length > 0 && (
              <div className="mt-4">
                <p className="mb-2 text-sm font-medium text-green-800">
                  Einladungslinks:
                </p>
                <div className="space-y-2">
                  {success.inviteLinks.map((link, idx) => (
                    <div
                      key={idx}
                      className="rounded-lg border border-green-200 bg-white px-3 py-2"
                    >
                      <p className="font-mono text-xs break-all text-gray-600">
                        {link}
                      </p>
                    </div>
                  ))}
                </div>
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
