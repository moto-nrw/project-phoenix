"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Mail } from "lucide-react";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { authService } from "~/lib/auth-service";
import { toAssignableRoleOptions, type RoleOption } from "~/lib/auth-helpers";
import { createInvitation } from "~/lib/invitation-api";
import type {
  CreateInvitationRequest,
  PendingInvitation,
} from "~/lib/invitation-helpers";
import { createLogger } from "~/lib/logger";

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
  // Every failure of this form, validation included, comes from the server
  // and is shown on the shared error path (#2511): the message in the alert
  // at the top, each failed field marked at the field.
  const formRef = useRef<HTMLFormElement>(null);
  const errors = useApiFormError(formRef);
  const showError = errors.show;
  // „Wiederholen“ sendet die aktuellen Eingaben, nicht die vom Fehler.
  const latestSendRef = useRef<() => Promise<void>>(async () => undefined);

  const [successInfo, setSuccessInfo] = useState<{
    email: string;
    link: string;
  } | null>(null);
  const { success: toastSuccess } = useToast();

  const loadRoles = useCallback(
    async (isCancelled: () => boolean = () => false) => {
      try {
        setIsLoadingRoles(true);
        const roleList = await authService.getRoles();
        if (isCancelled()) return;
        setRoles(toAssignableRoleOptions(roleList));
      } catch (err) {
        logger.error("failed to load roles", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!isCancelled()) {
          void showError(err, {
            object: "die Rollenauswahl",
            retry: () => void loadRoles(),
          });
        }
      } finally {
        if (!isCancelled()) {
          setIsLoadingRoles(false);
        }
      }
    },
    [showError],
  );

  useEffect(() => {
    let cancelled = false;
    void loadRoles(() => cancelled);
    return () => {
      cancelled = true;
    };
  }, [loadRoles]);

  const inviteBaseUrl = useMemo(() => {
    if (typeof globalThis !== "undefined" && "location" in globalThis) {
      return globalThis.location.origin;
    }
    return "";
  }, []);

  const handleChange =
    (key: keyof CreateInvitationRequest) => (value: string | undefined) => {
      setForm((prev) => ({ ...prev, [key]: value }));
    };

  const toOptional = (value?: string): string | undefined => {
    if (value === undefined || value === null) return undefined;
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : undefined;
  };

  const sendInvitation = async () => {
    errors.clear();
    setSuccessInfo(null);
    try {
      setIsSubmitting(true);
      const invitation = await createInvitation({
        email: form.email.trim(),
        roleId: form.roleId,
        firstName: toOptional(form.firstName),
        lastName: toOptional(form.lastName),
        position: toOptional(form.position),
      });

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
      logger.warn("invitation_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await errors.show(err, {
        object: "die Einladung",
        retry: () => void latestSendRef.current(),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  latestSendRef.current = sendInvitation;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    void sendInvitation();
  };

  const roleError = errors.fieldError("role_id");

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm md:p-6">
      <div className="mb-4 flex items-center gap-2 md:gap-3">
        <div className="rounded-xl bg-gray-100 p-2">
          <Mail
            className="h-4 w-4 text-gray-600 md:h-5 md:w-5"
            aria-hidden="true"
          />
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

      <form
        ref={formRef}
        onSubmit={handleSubmit}
        noValidate
        className="space-y-4"
      >
        <FormErrorAlert message={errors.error} />

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
          error={errors.fieldError("email")}
        />

        <div>
          <label
            id="invitation-role-label"
            htmlFor="invitation-role"
            className={`mb-1 block text-sm font-medium ${roleError ? "text-moto-red-strong" : "text-gray-700"}`}
          >
            Rolle
          </label>
          <CustomSelect
            id="invitation-role"
            name="role_id"
            ariaLabelledBy="invitation-role-label"
            ariaDescribedBy={roleError ? "invitation-role-error" : undefined}
            value={form.roleId ?? ""}
            onChange={(next) => handleChange("roleId")(next || undefined)}
            options={roles.map((role) => ({
              value: role.id,
              label: role.name,
            }))}
            placeholder="Rolle auswählen..."
            invalid={Boolean(roleError)}
            disabled={isSubmitting || isLoadingRoles}
          />
          {roleError ? (
            <p
              id="invitation-role-error"
              role="alert"
              className="text-moto-red-strong mt-1 text-xs"
            >
              {roleError}
            </p>
          ) : null}
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input
            id="invitation-first-name"
            name="first_name"
            label="Vorname (optional)"
            value={form.firstName}
            onChange={(event) => handleChange("firstName")(event.target.value)}
            disabled={isSubmitting}
            error={errors.fieldError("first_name")}
          />
          <Input
            id="invitation-last-name"
            name="last_name"
            label="Nachname (optional)"
            value={form.lastName}
            onChange={(event) => handleChange("lastName")(event.target.value)}
            disabled={isSubmitting}
            error={errors.fieldError("last_name")}
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
            name="position"
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
          data-setup-tour="invite-submit"
          disabled={isSubmitting || isLoadingRoles}
          className="w-full rounded-xl bg-gray-900 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 active:scale-98 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting ? "Wird gesendet…" : "Einladung senden"}
        </button>
      </form>
    </div>
  );
}
