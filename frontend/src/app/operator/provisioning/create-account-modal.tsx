import { useState, useCallback, useEffect, useRef } from "react";
import { CheckCircle2 } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { getRoleDisplayName, isAssignableStaffRole } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import { FormField, SelectWithChevron } from "./provisioning-shared";

const logger = createLogger({ component: "CreateAccountModal" });

interface RoleOption {
  id: string;
  label: string;
  systemName: string;
}

export function CreateAccountModal({
  isOpen,
  onClose,
  schoolId,
  schoolName,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly schoolId: string | null;
  readonly schoolName: string;
  readonly onCreated: () => void;
}) {
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [roleId, setRoleId] = useState<string | undefined>(undefined);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [position, setPosition] = useState("");
  const [caregiverEnabled, setCaregiverEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;

  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [isLoadingRoles, setIsLoadingRoles] = useState(true);
  const rolesLoad = useApiLoadError();
  const { show: showRolesError, clear: clearRolesError } = rolesLoad;
  // "Wiederholen" loads the roles again.
  const [rolesReload, setRolesReload] = useState(0);

  const [result, setResult] = useState<{
    id: string;
    email: string;
  } | null>(null);

  const inputClasses =
    "focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none";

  // Load roles on mount
  useEffect(() => {
    let cancelled = false;
    async function fetchRoles() {
      try {
        setIsLoadingRoles(true);
        clearRolesError();
        const roleList = await operatorProvisioningService.listSystemRoles();
        if (cancelled) return;
        const options = roleList
          .filter((role) => isAssignableStaffRole(role.name, role.isSystem))
          .map<RoleOption>((role) => ({
            id: role.id,
            label: role.name
              ? getRoleDisplayName(role.name)
              : `Rolle ${role.id}`,
            systemName: role.name,
          }));
        setRoles(options);
      } catch (err) {
        logger.error("failed_to_load_roles", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (cancelled) return;
        void showRolesError(err, {
          object: "die Liste der Rollen",
          retry: () => setRolesReload((count) => count + 1),
        });
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
  }, [rolesReload, showRolesError, clearRolesError]);

  // Reset form when opening
  useEffect(() => {
    if (isOpen) {
      setFirstName("");
      setLastName("");
      setEmail("");
      setRoleId(undefined);
      setPassword("");
      setConfirmPassword("");
      setPosition("");
      setCaregiverEnabled(false);
      clearError();
      setResult(null);
    }
  }, [isOpen, clearError]);

  const selectedRole = roles.find((role) => role.id === roleId);
  const showCaregiverToggle = selectedRole?.systemName === "admin";

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      // The System-Rolle select is a CustomSelect: its `required` is ARIA-only
      // and does not join native form constraint validation, so an Enter-submit
      // with no role lands here and must show a visible error instead of
      // returning silently. The remaining fields keep their native `required`.
      if (!roleId) {
        const hint = "Bitte wählen Sie eine System-Rolle aus.";
        invalid("Bitte prüfen Sie die markierten Felder.", { role_id: hint });
        return;
      }
      if (
        !schoolId ||
        !firstName.trim() ||
        !lastName.trim() ||
        !email.trim() ||
        !password ||
        !confirmPassword
      )
        return;

      if (password !== confirmPassword) {
        const hint = "Passwörter stimmen nicht überein.";
        invalid("Bitte prüfen Sie die markierten Felder.", {
          confirm_password: hint,
        });
        return;
      }

      setSaving(true);
      clearError();
      try {
        const created = await operatorProvisioningService.createSchoolAccount(
          schoolId,
          {
            email: email.trim(),
            first_name: firstName.trim(),
            last_name: lastName.trim(),
            password,
            confirm_password: confirmPassword,
            role_id: roleId,
            position: position || undefined,
            caregiver_enabled:
              showCaregiverToggle && caregiverEnabled ? true : undefined,
          },
        );
        setResult(created);
        onCreated();
      } catch (err) {
        logger.error("account_create_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showError(err, { object: "das Anlegen des Kontos" });
      } finally {
        setSaving(false);
      }
    },
    [
      schoolId,
      firstName,
      lastName,
      email,
      roleId,
      password,
      confirmPassword,
      position,
      caregiverEnabled,
      showCaregiverToggle,
      onCreated,
      invalid,
      clearError,
      showError,
    ],
  );

  const handleClose = useCallback(() => {
    onClose();
    setResult(null);
  }, [onClose]);

  const isFormValid =
    firstName.trim() &&
    lastName.trim() &&
    email.trim() &&
    roleId &&
    password &&
    confirmPassword;

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      title={`Konto erstellen — ${schoolName}`}
      footer={
        result ? (
          <button
            type="button"
            onClick={handleClose}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            Schließen
          </button>
        ) : (
          <>
            <button
              type="button"
              onClick={handleClose}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={(e) => void handleSubmit(e)}
              disabled={saving || !isFormValid}
              className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? "Wird erstellt..." : "Konto erstellen"}
            </button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-3">
          <div className="bg-moto-green/10 flex items-center gap-2 rounded-lg px-4 py-3">
            <CheckCircle2
              className="text-moto-green h-5 w-5"
              aria-hidden="true"
            />
            <span className="text-moto-green-strong text-sm font-medium">
              Konto erstellt
            </span>
          </div>
          <div className="space-y-2 text-sm text-gray-600">
            <p>
              <span className="font-medium">E-Mail:</span> {result.email}
            </p>
          </div>
        </div>
      ) : (
        <form
          ref={formRef}
          onSubmit={(e) => void handleSubmit(e)}
          className="space-y-4"
          id="create-account-form"
        >
          <FormErrorAlert message={formErrors.error} />
          <LoadErrorAlert error={rolesLoad.error} />
          <div className="grid grid-cols-2 gap-4">
            <FormField
              label="Vorname"
              htmlFor="create-account-first-name"
              required
              error={formErrors.fieldError("first_name")}
            >
              <input
                id="create-account-first-name"
                name="first_name"
                type="text"
                autoComplete="given-name"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                maxLength={255}
                className={inputClasses}
                required
              />
            </FormField>
            <FormField
              label="Nachname"
              htmlFor="create-account-last-name"
              required
              error={formErrors.fieldError("last_name")}
            >
              <input
                id="create-account-last-name"
                name="last_name"
                type="text"
                autoComplete="family-name"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                maxLength={255}
                className={inputClasses}
                required
              />
            </FormField>
          </div>
          <FormField
            label="E-Mail"
            htmlFor="create-account-email"
            required
            error={formErrors.fieldError("email")}
          >
            <input
              id="create-account-email"
              name="email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              maxLength={255}
              className={inputClasses}
              required
            />
          </FormField>
          <FormField
            label="System-Rolle"
            htmlFor="create-account-role"
            required
            error={formErrors.fieldError("role_id")}
          >
            <SelectWithChevron
              id="create-account-role"
              name="role_id"
              value={roleId ?? ""}
              onChange={(e) =>
                setRoleId(e.target.value === "" ? undefined : e.target.value)
              }
              disabled={isLoadingRoles}
              required
            >
              <option value="" disabled>
                {isLoadingRoles ? "Lade Rollen..." : "Rolle auswählen..."}
              </option>
              {roles.map((role) => (
                <option key={role.id} value={role.id}>
                  {role.label}
                </option>
              ))}
            </SelectWithChevron>
          </FormField>
          {showCaregiverToggle && (
            <div className="flex items-start gap-3 rounded-lg border border-gray-200 px-4 py-3">
              <input
                id="create-account-caregiver-enabled"
                type="checkbox"
                checked={caregiverEnabled}
                onChange={(e) => setCaregiverEnabled(e.target.checked)}
                className="text-moto-blue focus:ring-moto-blue mt-0.5 h-4 w-4 rounded border-gray-300"
              />
              <div className="space-y-1">
                <label
                  htmlFor="create-account-caregiver-enabled"
                  className="text-sm font-medium text-gray-900"
                >
                  Auch als Betreuer einsetzen
                </label>
                <p className="text-sm text-gray-600">
                  Vergibt zusätzlich die Betreuer-Rolle und legt das nötige
                  Staff-/Teacher-Profil an.
                </p>
              </div>
            </div>
          )}
          <FormField
            label="Passwort"
            htmlFor="create-account-password"
            required
            error={formErrors.fieldError("password")}
          >
            <input
              id="create-account-password"
              name="password"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              maxLength={255}
              className={inputClasses}
              required
            />
            <p className="mt-1 text-xs text-gray-500">
              Mind. 8 Zeichen, Groß-/Kleinbuchstaben, Zahl und Sonderzeichen
            </p>
          </FormField>
          <FormField
            label="Passwort bestätigen"
            htmlFor="create-account-confirm-password"
            required
            error={formErrors.fieldError("confirm_password")}
          >
            <input
              id="create-account-confirm-password"
              name="confirm_password"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              maxLength={255}
              className={inputClasses}
              required
            />
          </FormField>
          <FormField
            label="Position"
            htmlFor="create-account-position"
            error={formErrors.fieldError("position")}
          >
            <SelectWithChevron
              id="create-account-position"
              name="position"
              value={position}
              onChange={(e) => setPosition(e.target.value)}
            >
              <option value="">Position auswählen...</option>
              <option value="Pädagogische Fachkraft">
                Pädagogische Fachkraft
              </option>
              <option value="OGS-Büro">OGS-Büro</option>
              <option value="Extern">Extern</option>
            </SelectWithChevron>
          </FormField>
        </form>
      )}
    </Modal>
  );
}
