"use client";

import { useEffect, useMemo, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { Input } from "~/components/ui";
import { authService } from "~/lib/auth-service";
import { createInvitation } from "~/lib/invitation-api";
import type {
  CreateInvitationRequest,
  PendingInvitation,
} from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";

interface InvitationFormProps {
  onCreated?: (invitation: PendingInvitation) => void;
}

interface RoleOption {
  id: number;
  name: string;
}

const initialForm: CreateInvitationRequest = {
  email: "",
  roleId: undefined,
  firstName: "",
  lastName: "",
};

export function InvitationForm({ onCreated }: InvitationFormProps) {
  const [form, setForm] = useState<CreateInvitationRequest>(initialForm);
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successInfo, setSuccessInfo] = useState<{
    email: string;
    link: string;
  } | null>(null);
  const { success: toastSuccess } = useToast();

  useEffect(() => {
    let cancelled = false;
    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (cancelled) return;
        const options = roleList
          .map<RoleOption>((role) => ({
            id: Number(role.id),
            name: role.name ?? `Rolle ${role.id}`,
          }))
          .filter((role) => !Number.isNaN(role.id));
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

    void fetchRoles();
    return () => {
      cancelled = true;
    };
  }, []);

  const inviteBaseUrl = useMemo(() => {
    if (typeof window !== "undefined") {
      return window.location.origin;
    }
    return "";
  }, []);

  const handleChange =
    (key: keyof CreateInvitationRequest) => (value: string | number | undefined) => {
      setForm((prev) => ({ ...prev, [key]: value }));
    };

  const toOptional = (value?: string): string | undefined => {
    if (value === undefined || value === null) return undefined;
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : undefined;
  };

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setError(null);
    setSuccessInfo(null);

    if (!form.email.trim()) {
      setError("Bitte gib eine gültige E-Mail-Adresse ein.");
      return;
    }
    if (!form.roleId || form.roleId <= 0) {
      setError("Bitte wähle eine Rolle aus.");
      return;
    }

    try {
      setIsSubmitting(true);
      const invitation = await createInvitation({
        email: form.email.trim(),
        roleId: form.roleId,
        firstName: toOptional(form.firstName),
        lastName: toOptional(form.lastName),
      });

      const link = inviteBaseUrl
        ? `${inviteBaseUrl}/invite?token=${encodeURIComponent(invitation.token ?? "")}`
        : (invitation.token ?? "");
      setSuccessInfo({ email: invitation.email, link });
      toastSuccess(`Einladung an ${invitation.email} wurde gesendet.`);
      setForm((prev) => ({ ...initialForm, roleId: prev.roleId }));
      if (onCreated) {
        onCreated(invitation);
      }
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(
        apiError?.message ??
          "Die Einladung konnte nicht erstellt werden. Bitte versuche es erneut.",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="rounded-2xl border border-gray-200/50 bg-white/90 p-4 shadow-sm backdrop-blur-sm md:p-6">
      <div className="mb-4 flex items-center gap-2 md:gap-3">
        <div className="rounded-xl bg-gray-100 p-2">
          <svg
            className="h-4 w-4 text-gray-600 md:h-5 md:w-5"
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
          <h2 className="text-base font-semibold text-gray-900 md:text-lg">
            Neue Einladung
          </h2>
          <p className="text-xs text-gray-600 md:text-sm">
            Per E-Mail einladen
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
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
            <div className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
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
              value={form.roleId ?? ""}
              onChange={(event) =>
                handleChange("roleId")(Number(event.target.value) || undefined)
              }
              disabled={isSubmitting || isLoadingRoles}
            >
              <option value="" disabled>
                Rolle auswählen...
              </option>
              {roles.map((role) => (
                <option key={role.id} value={role.id}>
                  {role.name}
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
            onChange={(event) => handleChange("firstName")(event.target.value)}
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

        <button
          type="submit"
          disabled={isSubmitting || isLoadingRoles}
          className="w-full rounded-xl bg-gray-900 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 active:scale-98 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting ? "Wird gesendet…" : "Einladung senden"}
        </button>
      </form>
    </div>
  );
}
