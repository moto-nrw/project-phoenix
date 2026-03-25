"use client";

import { useEffect, useMemo, useState, type ChangeEvent } from "react";
import { Modal } from "~/components/ui/modal";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  AssignableRole,
  CreateSchoolAccountRequest,
  ProvisionedAccount,
} from "~/lib/operator/provisioning-helpers";

const logger = createLogger({ component: "OperatorManualCreateAccountModal" });

interface ManualCreateAccountModalProps {
  readonly isOpen: boolean;
  readonly schoolName: string;
  readonly onClose: () => void;
  readonly onCreate: (
    data: CreateSchoolAccountRequest,
  ) => Promise<ProvisionedAccount | void>;
}

interface LinkConfirmationState {
  email: string;
  data: CreateSchoolAccountRequest;
}

const initialForm = {
  firstName: "",
  lastName: "",
  email: "",
  password: "",
  confirmPassword: "",
  roleId: "",
  position: "",
};

export function ManualCreateAccountModal({
  isOpen,
  schoolName,
  onClose,
  onCreate,
}: ManualCreateAccountModalProps) {
  const [form, setForm] = useState(initialForm);
  const [roles, setRoles] = useState<AssignableRole[]>([]);
  const [loadingRoles, setLoadingRoles] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [linkConfirmation, setLinkConfirmation] =
    useState<LinkConfirmationState | null>(null);

  useEffect(() => {
    if (!isOpen) {
      setForm(initialForm);
      setError(null);
      setLinkConfirmation(null);
      return;
    }

    let cancelled = false;
    async function loadRoles() {
      try {
        setLoadingRoles(true);
        const roleList =
          await operatorProvisioningService.listAssignableRoles();
        if (cancelled) return;
        setRoles(roleList.filter((role) => role.name !== "guardian"));
      } catch (err) {
        logger.error("failed to load assignable roles", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          setError("Rollen konnten nicht geladen werden.");
        }
      } finally {
        if (!cancelled) {
          setLoadingRoles(false);
        }
      }
    }

    void loadRoles();
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  const needsPosition = useMemo(() => {
    const selectedRole = roles.find((role) => role.id === form.roleId);
    return selectedRole?.name === "user" || selectedRole?.name === "teacher";
  }, [form.roleId, roles]);

  const handleChange =
    (field: keyof typeof form) =>
    (event: ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
      setForm((current) => ({ ...current, [field]: event.target.value }));
    };

  const validate = (): string | null => {
    if (!form.firstName.trim()) return "Vorname ist erforderlich.";
    if (!form.lastName.trim()) return "Nachname ist erforderlich.";
    if (!form.email.trim()) return "E-Mail ist erforderlich.";
    if (!form.roleId) return "Bitte wähle eine Rolle aus.";
    if (!linkConfirmation) {
      if (!form.password) return "Passwort ist erforderlich.";
      if (form.password.length < 8)
        return "Passwort muss mindestens 8 Zeichen lang sein.";
      if (form.password !== form.confirmPassword)
        return "Passwörter stimmen nicht überein.";
    }
    return null;
  };

  const buildPayload = (linkExisting: boolean): CreateSchoolAccountRequest => ({
    email: form.email.trim(),
    password: linkExisting ? undefined : form.password,
    confirm_password: linkExisting ? undefined : form.confirmPassword,
    first_name: form.firstName.trim(),
    last_name: form.lastName.trim(),
    role_id: Number(form.roleId),
    position: form.position.trim() || undefined,
    link_existing: linkExisting,
  });

  const submit = async (linkExisting: boolean) => {
    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }

    try {
      setSubmitting(true);
      setError(null);
      const result = await onCreate(buildPayload(linkExisting));
      if (result?.status === "account_exists") {
        setLinkConfirmation({
          email: result.email,
          data: buildPayload(false),
        });
      }
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Konto konnte nicht angelegt werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  if (linkConfirmation) {
    return (
      <Modal
        isOpen={isOpen}
        onClose={onClose}
        title="Bestehendes Konto verknüpfen"
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-700">
            Ein Konto mit der E-Mail{" "}
            <span className="font-semibold text-gray-900">
              {linkConfirmation.email}
            </span>{" "}
            existiert bereits.
          </p>
          <p className="text-sm text-gray-600">
            Möchten Sie dieses Konto mit {schoolName} verknüpfen?
          </p>

          {error && (
            <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {error}
            </div>
          )}

          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={() => {
                void submit(true);
              }}
              disabled={submitting}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
            >
              {submitting ? "Wird verknüpft..." : "Verknüpfen"}
            </button>
          </div>
        </div>
      </Modal>
    );
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Konto anlegen">
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void submit(false);
        }}
        className="space-y-4"
      >
        <p className="text-sm text-gray-600">{schoolName}</p>

        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="space-y-1 text-sm">
            <span className="font-medium text-gray-700">Vorname</span>
            <input
              value={form.firstName}
              onChange={handleChange("firstName")}
              className="w-full rounded-lg border border-gray-300 px-3 py-2"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium text-gray-700">Nachname</span>
            <input
              value={form.lastName}
              onChange={handleChange("lastName")}
              className="w-full rounded-lg border border-gray-300 px-3 py-2"
            />
          </label>
        </div>

        <label className="space-y-1 text-sm">
          <span className="font-medium text-gray-700">E-Mail</span>
          <input
            type="email"
            value={form.email}
            onChange={handleChange("email")}
            className="w-full rounded-lg border border-gray-300 px-3 py-2"
          />
        </label>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="space-y-1 text-sm">
            <span className="font-medium text-gray-700">Passwort</span>
            <input
              type="password"
              value={form.password}
              onChange={handleChange("password")}
              className="w-full rounded-lg border border-gray-300 px-3 py-2"
            />
          </label>
          <label className="space-y-1 text-sm">
            <span className="font-medium text-gray-700">
              Passwort bestätigen
            </span>
            <input
              type="password"
              value={form.confirmPassword}
              onChange={handleChange("confirmPassword")}
              className="w-full rounded-lg border border-gray-300 px-3 py-2"
            />
          </label>
        </div>

        <label className="space-y-1 text-sm">
          <span className="font-medium text-gray-700">Systemrolle</span>
          <select
            value={form.roleId}
            onChange={handleChange("roleId")}
            disabled={loadingRoles}
            className="w-full rounded-lg border border-gray-300 px-3 py-2"
          >
            <option value="">Bitte wählen</option>
            {roles.map((role) => (
              <option key={role.id} value={role.id}>
                {getRoleDisplayName(role.name)}
              </option>
            ))}
          </select>
        </label>

        {needsPosition && (
          <label className="space-y-1 text-sm">
            <span className="font-medium text-gray-700">
              Pädagogische Position
            </span>
            <input
              value={form.position}
              onChange={handleChange("position")}
              className="w-full rounded-lg border border-gray-300 px-3 py-2"
            />
          </label>
        )}

        <div className="flex gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700"
          >
            Abbrechen
          </button>
          <button
            type="submit"
            disabled={submitting || loadingRoles}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            {submitting ? "Wird erstellt..." : "Konto anlegen"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
