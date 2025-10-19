"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, Input } from "~/components/ui";
import { authService } from "~/lib/auth-service";
import { createInvitation } from "~/lib/invitation-api";
import type { CreateInvitationRequest, PendingInvitation } from "~/lib/invitation-helpers";
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
  roleId: 0,
  firstName: "",
  lastName: "",
};

export function InvitationForm({ onCreated }: InvitationFormProps) {
  const [form, setForm] = useState<CreateInvitationRequest>(initialForm);
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successInfo, setSuccessInfo] = useState<{ email: string; link: string } | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (cancelled) return;
        const options = roleList
          .map<RoleOption>((role) => ({ id: Number(role.id), name: role.name ?? `Rolle ${role.id}` }))
          .filter((role) => !Number.isNaN(role.id));
        setRoles(options);
        if (options.length > 0) {
          setForm((prev) => ({ ...prev, roleId: prev.roleId ?? options[0]!.id }));
        }
      } catch (err) {
        console.error("Failed to load roles", err);
        if (!cancelled) {
          setError("Rollen konnten nicht geladen werden. Bitte aktualisiere die Seite.");
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

  const handleChange = (key: keyof CreateInvitationRequest) => (value: string | number) => {
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
    if (!form.roleId) {
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

      const link = inviteBaseUrl ? `${inviteBaseUrl}/invite?token=${encodeURIComponent(invitation.token ?? "")}` : invitation.token ?? "";
      setSuccessInfo({ email: invitation.email, link });
      setForm((prev) => ({ ...initialForm, roleId: prev.roleId }));
      if (onCreated) {
        onCreated(invitation);
      }
    } catch (err) {
      const apiError = err as ApiError | undefined;
      setError(apiError?.message ?? "Die Einladung konnte nicht erstellt werden. Bitte versuche es erneut.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="rounded-3xl border border-gray-100 bg-white/90 p-6 shadow-xl backdrop-blur-xl">
      <h2 className="text-xl font-semibold text-gray-900">Neue Einladung senden</h2>
      <p className="mt-1 text-sm text-gray-600">Lade neue Benutzer per E-Mail ein und weise direkt eine Rolle zu.</p>

      <form onSubmit={handleSubmit} className="mt-6 space-y-4">
        {error && <Alert type="error" message={error} />}
        {successInfo && (
          <div className="space-y-2">
            <Alert
              type="success"
              message={`Einladung an ${successInfo.email} wurde gesendet.`}
            />
            <div className="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-xs font-mono text-gray-600">
              {successInfo.link}
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
          <label htmlFor="invitation-role" className="mb-1 block text-sm font-medium text-gray-700">
            Rolle auswählen
          </label>
          <select
            id="invitation-role"
            className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm focus:border-[#5080d8] focus:outline-none focus:ring-2 focus:ring-[#5080d8]/40 disabled:cursor-not-allowed disabled:bg-gray-100"
            value={form.roleId}
            onChange={(event) => handleChange("roleId")(Number(event.target.value))}
            disabled={isSubmitting || isLoadingRoles}
          >
            {roles.map((role) => (
              <option key={role.id} value={role.id}>
                {role.name}
              </option>
            ))}
          </select>
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
          className="w-full rounded-xl bg-[#5080d8] py-3 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-[#3a6bc4] hover:shadow-xl disabled:cursor-not-allowed disabled:bg-gray-400"
        >
          {isSubmitting ? "Einladung wird gesendet…" : "Einladung senden"}
        </button>
      </form>
    </div>
  );
}
