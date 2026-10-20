"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Mail } from "lucide-react";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { Alert } from "~/components/ui/alert";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { authService } from "~/lib/auth-service";
import { toAssignableRoleOptions, type RoleOption } from "~/lib/auth-helpers";
import { formatDate } from "~/lib/date-helpers";
import { createInvitation, findFormerEmployment } from "~/lib/invitation-api";
import {
  FORMER_EMPLOYMENT_PART_LABELS,
  type CreateInvitationRequest,
  type EmploymentAction,
  type FormerEmployment,
  type FormerEmploymentPart,
  type PendingInvitation,
} from "~/lib/invitation-helpers";
import { createLogger } from "~/lib/logger";
import { useDebounce } from "~/lib/use-debounce";
import { FieldErrorHint } from "~/components/ui/field-error-hint";

const logger = createLogger({ component: "InvitationForm" });

interface InvitationFormProps {
  readonly onCreated?: (invitation: PendingInvitation) => void;
  readonly existingPositions?: readonly string[];
}

const EMPTY_POSITIONS: readonly string[] = [];

// Only an address that could be complete is worth a lookup; a half-typed one
// would ask once per keystroke pause.
const COMPLETE_ADDRESS = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;
const FORMER_EMPLOYMENT_DEBOUNCE_MS = 400;
const FORMER_EMPLOYMENT_LOOKUP_TIMEOUT_MS = 5_000;

/**
 * The former employment an invitation to the typed address would continue
 * at this school (ADR 0021, #3402), or null. A result belongs to the address
 * it was asked for, so it disappears as soon as the address changes.
 */
function useFormerEmployment(email: string): {
  formerEmployment: FormerEmployment | null;
  pending: boolean;
} {
  const current = email.trim().toLowerCase();
  const address = useDebounce(current, FORMER_EMPLOYMENT_DEBOUNCE_MS);
  const [answer, setAnswer] = useState<{
    address: string;
    formerEmployment: FormerEmployment | null;
  } | null>(null);

  useEffect(() => {
    if (!COMPLETE_ADDRESS.test(address)) return;
    let cancelled = false;
    const controller = new AbortController();
    const timeoutID = window.setTimeout(
      () => controller.abort(),
      FORMER_EMPLOYMENT_LOOKUP_TIMEOUT_MS,
    );
    findFormerEmployment(address, controller.signal)
      .then((formerEmployment) => {
        if (!cancelled) setAnswer({ address, formerEmployment });
      })
      .catch((err: unknown) => {
        // Without a successful lookup there is no choice to offer. Keep the
        // existing invitation behavior; sending reports its own form errors.
        if (cancelled) return;
        setAnswer({ address, formerEmployment: null });
        logger.warn("former_employment_lookup_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => window.clearTimeout(timeoutID));
    return () => {
      cancelled = true;
      window.clearTimeout(timeoutID);
      controller.abort();
    };
  }, [address]);

  return {
    formerEmployment:
      answer?.address === current ? answer.formerEmployment : null,
    pending: COMPLETE_ADDRESS.test(current) && answer?.address !== current,
  };
}

function formerEmploymentParts(parts: readonly FormerEmploymentPart[]): string {
  return parts.map((part) => FORMER_EMPLOYMENT_PART_LABELS[part]).join(", ");
}

function FormerEmploymentHint({
  formerEmployment,
  action,
  onActionChange,
  disabled,
}: {
  readonly formerEmployment: FormerEmployment;
  readonly action: EmploymentAction;
  readonly onActionChange: (action: EmploymentAction) => void;
  readonly disabled: boolean;
}) {
  const resumes = formerEmploymentParts(formerEmployment.resumes);
  const notReturning = formerEmploymentParts(formerEmployment.notReturning);
  return (
    <Alert
      type="info"
      title="Diese Person war schon bei Ihnen beschäftigt"
      message={`In moto eingetragen vom ${formatDate(formerEmployment.startedOn)} bis ${formatDate(formerEmployment.endedOn)}.`}
      details={
        <div className="mt-2 space-y-2">
          <SegmentedControl
            ariaLabel="Beschäftigung bei Rückkehr"
            className="max-sm:grid-flow-row"
            value={action}
            onChange={onActionChange}
            fullWidth
            items={[
              { value: "resume", label: "Wiederaufnahme", disabled },
              { value: "new", label: "Neuaufnahme", disabled },
            ]}
          />
          <p>
            {action === "resume"
              ? "Die frühere Beschäftigung wird fortgeführt."
              : "Eine neue Beschäftigung beginnt. Die frühere bleibt beendet."}
          </p>
          {action === "new" ? (
            <p>
              Stammdaten, Qualifikationen, Stundenkonto und Urlaub beginnen neu.
            </p>
          ) : null}
          {action === "resume" && resumes ? (
            <p>
              <span className="font-semibold">
                Kommt bei Wiederaufnahme zurück:
              </span>{" "}
              <span>{resumes}</span>
            </p>
          ) : null}
          {notReturning ? (
            <p>
              <span className="font-semibold">Kommt nicht zurück:</span>{" "}
              <span>{notReturning}</span>
            </p>
          ) : null}
          <p>Gruppen, Klassen und Vertretungen legen Sie danach neu fest.</p>
        </div>
      }
    />
  );
}

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
  const clearErrors = errors.clear;

  const { formerEmployment, pending: checkingAddress } = useFormerEmployment(
    form.email,
  );

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
        clearErrors();
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
    [clearErrors, showError],
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
      setForm((prev) => ({
        ...prev,
        ...(key === "email" ? { employmentAction: undefined } : {}),
        [key]: value,
      }));
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
        ...(formerEmployment
          ? { employmentAction: form.employmentAction ?? "resume" }
          : {}),
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
        // Erneut absenden: derselbe Weg wie „Einladung senden“, mit den
        // Eingaben von jetzt, nicht denen vom Zeitpunkt des Fehlers.
        retry: () => formRef.current?.requestSubmit(),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!checkingAddress) void sendInvitation();
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

        {formerEmployment ? (
          <FormerEmploymentHint
            formerEmployment={formerEmployment}
            action={form.employmentAction ?? "resume"}
            onActionChange={(employmentAction) =>
              setForm((prev) => ({ ...prev, employmentAction }))
            }
            disabled={isSubmitting}
          />
        ) : null}

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
          <FieldErrorHint id="invitation-role-error" message={roleError} live />
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
          disabled={isSubmitting || isLoadingRoles || checkingAddress}
          className="w-full rounded-xl bg-gray-900 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 active:scale-98 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting
            ? "Wird gesendet…"
            : checkingAddress
              ? "Adresse wird geprüft…"
              : "Einladung senden"}
        </button>
      </form>
    </div>
  );
}
