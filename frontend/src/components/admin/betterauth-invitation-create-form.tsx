"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { Input, Button } from "~/components/ui";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "~/components/ui/card";
import { authClient, useSession } from "~/lib/auth-client";
import { authService } from "~/lib/auth-service";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import type { Role } from "~/lib/auth-helpers";

interface BetterAuthInvitationCreateFormProps {
  readonly onCreated?: () => void;
}

/**
 * Map Go backend role names to BetterAuth role names.
 * Go backend uses descriptive names, BetterAuth uses camelCase identifiers.
 */
const GO_TO_BETTERAUTH_ROLE_MAP: Record<string, string> = {
  supervisor: "supervisor",
  "ogs-administrator": "ogsAdmin",
  ogsadministrator: "ogsAdmin",
  "buero-administrator": "bueroAdmin",
  bueroadministrator: "bueroAdmin",
  "traeger-administrator": "traegerAdmin",
  traegeradministrator: "traegerAdmin",
  // Fallback mappings for other common patterns
  admin: "ogsAdmin",
  administrator: "ogsAdmin",
};

/**
 * Convert Go backend role name to BetterAuth role name.
 * Returns the original if no mapping found.
 */
function toBetterAuthRole(goRoleName: string): string {
  const normalized = goRoleName.toLowerCase().replaceAll(/[\s_]/g, "-");
  return GO_TO_BETTERAUTH_ROLE_MAP[normalized] ?? goRoleName;
}

interface FormState {
  email: string;
  roleId: string;
  firstName: string;
  lastName: string;
}

const initialFormState: FormState = {
  email: "",
  roleId: "",
  firstName: "",
  lastName: "",
};

/**
 * Form for creating BetterAuth organization invitations.
 * Uses authClient.organization.inviteMember() to create invitations.
 * Roles are fetched from Go backend (authorization source of truth).
 */
export function BetterAuthInvitationCreateForm({
  onCreated,
}: BetterAuthInvitationCreateFormProps) {
  const { data: session } = useSession();
  const { success: toastSuccess } = useToast();

  const [form, setForm] = useState<FormState>(initialFormState);
  const [roles, setRoles] = useState<Role[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successInfo, setSuccessInfo] = useState<{
    email: string;
    link: string;
  } | null>(null);

  // Fetch roles from Go backend
  useEffect(() => {
    let cancelled = false;

    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (cancelled) return;
        setRoles(roleList);
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

    void fetchRoles();
    return () => {
      cancelled = true;
    };
  }, []);

  // Get the base URL for invitation links
  const inviteBaseUrl = useMemo(() => {
    if (typeof globalThis !== "undefined" && "location" in globalThis) {
      return globalThis.location.origin;
    }
    return "";
  }, []);

  const handleChange = useCallback(
    (key: keyof FormState) => (value: string) => {
      setForm((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccessInfo(null);

    // Validation
    if (!form.email.trim()) {
      setError("Bitte gib eine gültige E-Mail-Adresse ein.");
      return;
    }

    if (!form.roleId) {
      setError("Bitte wähle eine Rolle aus.");
      return;
    }

    // Get active organization ID from session
    const organizationId = session?.session.activeOrganizationId;
    if (!organizationId) {
      setError("Keine aktive Organisation gefunden.");
      return;
    }

    // Find selected role and map to BetterAuth role name
    const selectedRole = roles.find((r) => r.id === form.roleId);
    if (!selectedRole) {
      setError("Ausgewählte Rolle nicht gefunden.");
      return;
    }

    const betterAuthRole = toBetterAuthRole(selectedRole.name);

    try {
      setIsSubmitting(true);

      // Create invitation via BetterAuth
      // Cast role to expected type - we use custom roles defined in auth.ts
      const result = await authClient.organization.inviteMember({
        email: form.email.trim().toLowerCase(),
        role: betterAuthRole as "admin" | "member" | "owner",
        organizationId,
      });

      if (result.error) {
        // Handle specific error cases
        if (result.error.code === "USER_ALREADY_MEMBER") {
          setError("Diese Person ist bereits Mitglied der Organisation.");
        } else if (result.error.code === "INVITATION_EXISTS") {
          setError(
            "Für diese E-Mail-Adresse existiert bereits eine ausstehende Einladung.",
          );
        } else {
          setError(
            result.error.message ??
              "Die Einladung konnte nicht erstellt werden.",
          );
        }
        return;
      }

      // Build invitation link with name params for pre-filling acceptance form
      const invitationId = result.data?.id;
      if (invitationId) {
        const params = new URLSearchParams({
          email: form.email.trim(),
          role: getRoleDisplayName(selectedRole.name),
        });
        if (form.firstName.trim()) {
          params.set("firstName", form.firstName.trim());
        }
        if (form.lastName.trim()) {
          params.set("lastName", form.lastName.trim());
        }

        const link = `${inviteBaseUrl}/accept-invitation/${invitationId}?${params.toString()}`;
        setSuccessInfo({ email: form.email.trim(), link });
      }

      toastSuccess(`Einladung an ${form.email} wurde gesendet.`);
      setForm(initialFormState);
      onCreated?.();
    } catch (err) {
      console.error("Failed to create invitation:", err);
      setError("Die Einladung konnte nicht erstellt werden.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <div className="rounded-xl bg-gray-100 p-2">
            <svg
              className="h-5 w-5 text-gray-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>
          <div>
            <CardTitle>Neue Einladung</CardTitle>
            <CardDescription>Per E-Mail einladen</CardDescription>
          </div>
        </div>
      </CardHeader>

      <CardContent>
        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {error && (
            <div className="rounded-xl border border-red-200/50 bg-red-50/50 p-3">
              <div className="flex items-start gap-2">
                <svg
                  className="mt-0.5 h-4 w-4 flex-shrink-0 text-red-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <p className="text-sm text-red-700">{error}</p>
              </div>
            </div>
          )}

          {successInfo && (
            <div className="space-y-2">
              <div className="rounded-lg border border-green-200 bg-green-50 px-3 py-2">
                <p className="text-sm text-green-700">
                  Einladung an <strong>{successInfo.email}</strong> wurde
                  gesendet.
                </p>
              </div>
              <div className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
                <p className="mb-1 text-xs font-medium text-gray-600">
                  Einladungslink (zum manuellen Teilen):
                </p>
                <p className="font-mono text-xs break-all text-gray-600">
                  {successInfo.link}
                </p>
              </div>
            </div>
          )}

          <Input
            id="invitation-email"
            name="email"
            label="E-Mail-Adresse"
            type="email"
            value={form.email}
            onChange={(event) => handleChange("email")(event.target.value)}
            disabled={isSubmitting}
            required
          />

          <div>
            <label
              htmlFor="invitation-role"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Rolle
            </label>
            <div className="relative">
              <select
                id="invitation-role"
                className="w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm text-gray-900 transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500"
                value={form.roleId}
                onChange={(event) => handleChange("roleId")(event.target.value)}
                disabled={isSubmitting || isLoadingRoles}
              >
                <option value="" disabled>
                  Rolle auswählen...
                </option>
                {roles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {getRoleDisplayName(role.name)}
                  </option>
                ))}
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                <svg
                  className="h-4 w-4 text-gray-400"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M19 9l-7 7-7-7"
                  />
                </svg>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              id="invitation-first-name"
              name="firstName"
              label="Vorname (optional)"
              value={form.firstName}
              onChange={(event) =>
                handleChange("firstName")(event.target.value)
              }
              disabled={isSubmitting}
            />
            <Input
              id="invitation-last-name"
              name="lastName"
              label="Nachname (optional)"
              value={form.lastName}
              onChange={(event) => handleChange("lastName")(event.target.value)}
              disabled={isSubmitting}
            />
          </div>

          <Button
            type="submit"
            disabled={isSubmitting || isLoadingRoles}
            isLoading={isSubmitting}
            loadingText="Wird gesendet…"
            className="w-full"
          >
            Einladung senden
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
