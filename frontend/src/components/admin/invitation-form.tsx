"use client";

import { useEffect, useMemo, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { authService } from "~/lib/auth-service";
import { toAssignableRoleOptions, type RoleOption } from "~/lib/auth-helpers";
import { createInvitation } from "~/lib/invitation-api";
import type {
  CreateInvitationRequest,
  PendingInvitation,
} from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { createLogger } from "~/lib/logger";
import { trackEvent } from "~/lib/analytics";

const logger = createLogger({ component: "InvitationForm" });

interface InvitationFormProps {
  readonly onCreated?: (invitation: PendingInvitation) => void;
  readonly existingPositions?: readonly string[];
}

const EMPTY_POSITIONS: readonly string[] = [];

const initialForm: CreateInvitationRequest = {
  email: "",
  roleId: undefined,
  firstName: "",
  lastName: "",
  position: "",
};

export function InvitationForm({
  onCreated,
  existingPositions = EMPTY_POSITIONS,
}: InvitationFormProps) {
  const [form, setForm] = useState<CreateInvitationRequest>(initialForm);
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);
  const errorRef = useScrollToError(error);

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
        setRoles(toAssignableRoleOptions(roleList));
      } catch (err) {
        logger.error("failed to load roles", {
          error: err instanceof Error ? err.message : String(err),
        });
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
    if (typeof globalThis !== "undefined" && "location" in globalThis) {
      return globalThis.location.origin;
    }
    return "";
  }, []);

  const handleChange =
    (key: keyof CreateInvitationRequest) =>
    (value: string | number | undefined) => {
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
    setErrorFieldName(null);
    setSuccessInfo(null);

    if (!form.email.trim()) {
      setError("Bitte gib eine gültige E-Mail-Adresse ein.");
      setErrorFieldName("email");
      return;
    }
    if (!form.roleId || form.roleId <= 0) {
      setError("Bitte wähle eine Rolle aus.");
      setErrorFieldName("roleId");
      return;
    }

    try {
      setIsSubmitting(true);
      const invitation = await createInvitation({
        email: form.email.trim(),
        roleId: form.roleId,
        firstName: toOptional(form.firstName),
        lastName: toOptional(form.lastName),
        position: toOptional(form.position),
      });

      trackEvent("user_invited");

      const link = inviteBaseUrl
        ? `${inviteBaseUrl}/invite?token=${encodeURIComponent(invitation.token ?? "")}`
        : (invitation.token ?? "");
      setSuccessInfo({ email: invitation.email, link });
      toastSuccess(`Einladung an ${invitation.email} wurde gesendet.`);
      setForm(() => initialForm);
      if (onCreated) {
        onCreated(invitation);
      }
    } catch (err) {
      const apiError = err as ApiError | undefined;

      // Handle specific error cases with user-friendly messages
      if (apiError?.status === 409) {
        if (apiError.code === "ACCOUNT_ALREADY_HAS_TENANT_ACCESS") {
          setError("Dieser Account hat bereits Zugang zu dieser Einrichtung.");
        } else {
          setError(
            "Für diese E-Mail-Adresse existiert bereits ein Account. Bitte verwende eine andere E-Mail-Adresse.",
          );
        }
        setErrorFieldName("email");
      } else {
        setError(
          apiError?.message ??
            "Die Einladung konnte nicht erstellt werden. Bitte versuche es erneut.",
        );
      }
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

      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        {error && (
          <div
            ref={errorRef}
            className="rounded-xl border border-red-200/50 bg-red-50/50 p-3"
          >
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
          className={errorFieldName === "email" ? "ring-red-400" : ""}
        />

        <div>
          <label
            htmlFor="invitation-role"
            className={`mb-1 block text-sm font-medium ${errorFieldName === "roleId" ? "text-red-600" : "text-gray-700"}`}
          >
            Rolle
          </label>
          <CustomSelect
            id="invitation-role"
            value={form.roleId === undefined ? "" : String(form.roleId)}
            onChange={(next) =>
              handleChange("roleId")(Number(next) || undefined)
            }
            options={roles.map((role) => ({
              value: String(role.id),
              label: role.name,
            }))}
            placeholder="Rolle auswählen..."
            invalid={errorFieldName === "roleId"}
            disabled={isSubmitting || isLoadingRoles}
          />
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

        <div>
          <label
            htmlFor="invitation-position"
            className="mb-1 block text-sm font-medium text-gray-700"
          >
            Position (optional)
          </label>
          <input
            type="text"
            id="invitation-position"
            list="invitation-position-suggestions"
            className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 transition-colors focus:border-gray-400 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500"
            value={form.position ?? ""}
            onChange={(event) =>
              handleChange("position")(event.target.value || undefined)
            }
            placeholder="z.B. Pädagogische Fachkraft, OGS-Büro"
            disabled={isSubmitting}
          />
          {existingPositions.length > 0 && (
            <datalist id="invitation-position-suggestions">
              {existingPositions.map((pos) => (
                <option key={pos} value={pos} />
              ))}
            </datalist>
          )}
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
