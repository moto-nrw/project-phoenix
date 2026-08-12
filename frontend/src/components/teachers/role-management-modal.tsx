"use client";

import { useCallback, useEffect, useState } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { CustomSelect } from "~/components/ui/custom-select";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import {
  getRoleDisplayName,
  toAssignableRoleOptions,
  type Role,
  type RoleOption,
} from "~/lib/auth-helpers";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RoleManagementModal" });

interface RoleManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly accountId: string;
  readonly accountLabel: string;
  readonly onUpdated?: () => void | Promise<void>;
}

export function RoleManagementModal({
  isOpen,
  onClose,
  accountId,
  accountLabel,
  onUpdated,
}: RoleManagementModalProps) {
  const { success: toastSuccess } = useToast();
  const [currentRoles, setCurrentRoles] = useState<Role[]>([]);
  const [roleOptions, setRoleOptions] = useState<RoleOption[]>([]);
  const [targetRoleId, setTargetRoleId] = useState<number | undefined>(
    undefined,
  );
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const loadState = useCallback(async () => {
    if (!accountId) return;

    try {
      setLoading(true);
      setErrorMessage("");

      const [accountRoles, allRoles] = await Promise.all([
        authService.getAccountRoles(accountId),
        authService.getRoles(),
      ]);

      setCurrentRoles(accountRoles);
      // lehrkraft (#1772) ist hier bewusst NICHT wählbar: der Rollen-Tausch
      // würde nur die auth-Rolle wechseln und das Betreuungsprofil
      // (users.teachers) samt aktiver Gruppen-Aufsichten stehen lassen —
      // genau die Kombination, die Einladung und Operator-Provisioning
      // verbieten. Lehrkraft-Zugänge entstehen über die Einladung bzw. die
      // Personal-Anlage.
      setRoleOptions(
        toAssignableRoleOptions(allRoles).filter(
          (option) => option.systemName !== "lehrkraft",
        ),
      );
      setTargetRoleId(
        accountRoles.length > 0 ? Number(accountRoles[0]?.id) : undefined,
      );
    } catch (error) {
      logger.error("failed to load account role state", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
      });
      setErrorMessage("Die Rollen konnten nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [accountId]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    void loadState();
  }, [isOpen, loadState]);

  const currentRoleIds = currentRoles.map((role) => Number(role.id));
  const isUnchanged =
    targetRoleId !== undefined && currentRoleIds.includes(targetRoleId);
  // Ein Lehrkraft-Konto (#1772) hat kein Betreuungsprofil — ein Tausch auf
  // Betreuer/Admin hier würde die volle Rolle ohne users.teachers-Zeile
  // hinterlassen. Solche Wechsel laufen über Offboarding + Neuanlage.
  const currentIsLehrkraft = currentRoles.some(
    (role) => role.name.toLowerCase() === "lehrkraft",
  );

  async function handleSave() {
    if (!accountId || targetRoleId === undefined || isUnchanged) {
      return;
    }

    try {
      setSaving(true);
      setErrorMessage("");

      // Assign the new role before removing the old one so a mid-sequence
      // failure leaves the account with an extra role rather than none.
      await authService.assignRoleToAccount(accountId, String(targetRoleId));
      for (const oldId of currentRoleIds.filter((id) => id !== targetRoleId)) {
        await authService.removeRoleFromAccount(accountId, String(oldId));
      }

      toastSuccess("Rolle wurde aktualisiert.");
      await onUpdated?.();
      await loadState();
      onClose();
    } catch (error) {
      logger.error("failed to update account role", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        targetRoleId,
      });
      setErrorMessage("Die Rolle konnte nicht aktualisiert werden.");
    } finally {
      setSaving(false);
    }
  }

  const currentRoleLabel =
    currentRoles.length > 0
      ? currentRoles.map((role) => getRoleDisplayName(role.name)).join(", ")
      : "Keine Rolle hinterlegt";

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Rolle verwalten: ${accountLabel}`}
      size="md"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Schließen
          </button>
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={
              saving ||
              loading ||
              isUnchanged ||
              !targetRoleId ||
              currentIsLehrkraft
            }
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {saving ? "Wird gespeichert..." : "Speichern"}
          </button>
        </>
      }
    >
      {loading ? (
        <div className="py-8 text-sm text-gray-500">Wird geladen...</div>
      ) : (
        <div className="space-y-4">
          <InfoSection
            title="Aktuelle Rolle"
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.roles.icon}
                tone={MOTO_CONCEPTS.roles.tone}
                size={18}
              />
            }
            accentColor="purple"
          >
            <DataGrid>
              <DataField label="Rolle">{currentRoleLabel}</DataField>
            </DataGrid>
          </InfoSection>

          {currentIsLehrkraft ? (
            <Alert
              type="info"
              message="Lehrkraft-Zugänge haben kein Betreuungsprofil und können hier nicht umgestellt werden. Für einen Wechsel den Zugang deaktivieren und die Person neu anlegen bzw. einladen."
            />
          ) : (
            <div>
              <label
                id="target-role-select-label"
                htmlFor="target-role-select"
                className="mb-1 block text-xs font-medium text-gray-700"
              >
                Neue Rolle
              </label>
              <CustomSelect
                id="target-role-select"
                ariaLabelledBy="target-role-select-label"
                value={targetRoleId === undefined ? "" : String(targetRoleId)}
                onChange={(next) =>
                  setTargetRoleId(next === "" ? undefined : Number(next))
                }
                options={roleOptions.map((role) => ({
                  value: String(role.id),
                  label: role.name,
                }))}
                placeholder="Rolle auswählen..."
                disabled={saving}
              />
            </div>
          )}

          <Alert type="error" message={errorMessage} />
        </div>
      )}
    </FormModal>
  );
}
