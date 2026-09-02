"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { useToast } from "~/contexts/ToastContext";
import { isAssignableStaffRole } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import {
  AccountTenantAccessApiError,
  accountTenantAccessService,
  type AccountTenantAccess,
} from "~/lib/operator/account-tenant-access-api";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";

const logger = createLogger({ component: "AccountTenantAccessModal" });

const ROLE_LABELS: Record<string, string> = {
  admin: "Verwaltung",
  user: "Betreuung",
  guest: "Gast",
  lehrkraft: "Lehrkraft",
};

function roleLabel(name: string): string {
  return ROLE_LABELS[name.toLowerCase()] ?? name;
}

function statusTone(status: AccountTenantAccess["status"]) {
  if (status === "active") return "green" as const;
  if (status === "pending") return "orange" as const;
  return "gray" as const;
}

function statusLabel(status: AccountTenantAccess["status"]): string {
  if (status === "active") return "Aktiv";
  if (status === "pending") return "Ausstehend";
  return "Kein Zugang";
}

function accessRevocationBlocked(entry: AccountTenantAccess): boolean {
  return entry.roles.some(
    (role) =>
      role.baseRole?.trim().toLowerCase() === "guardian" ||
      (role.isSystem &&
        ["guardian", "user", "teacher"].includes(role.name.toLowerCase())),
  );
}

interface AccountTenantAccessModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly accountId: string;
  readonly accountLabel: string;
  readonly accountEmail: string;
  readonly onUpdated?: () => void | Promise<void>;
}

/**
 * Schulzugänge eines Kontos verwalten (Issue #1021): welche Schulen ein Konto
 * erreicht, mit welcher Rolle, plus Zugang ergänzen und entziehen.
 */
export function AccountTenantAccessModal({
  isOpen,
  onClose,
  accountId,
  accountLabel,
  accountEmail,
  onUpdated,
}: AccountTenantAccessModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();

  const [access, setAccess] = useState<AccountTenantAccess[]>([]);
  const [schools, setSchools] = useState<{ id: string; label: string }[]>([]);
  const [rolesBySchool, setRolesBySchool] = useState<
    Record<string, { id: string; name: string }[]>
  >({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const [addSchoolId, setAddSchoolId] = useState("");
  const [addRoleId, setAddRoleId] = useState("");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [revokeTarget, setRevokeTarget] = useState<AccountTenantAccess | null>(
    null,
  );
  const loadRequestRef = useRef(0);

  const load = useCallback(async () => {
    if (!accountId) return;

    const requestId = ++loadRequestRef.current;
    const isCurrentRequest = () => requestId === loadRequestRef.current;
    try {
      setLoading(true);
      setErrorMessage("");
      setAccess([]);
      setSchools([]);
      setRolesBySchool({});
      setAddSchoolId("");
      setAddRoleId("");
      setFirstName("");
      setLastName("");
      setRevokeTarget(null);
      const [entries, schoolList] = await Promise.all([
        accountTenantAccessService.list(accountId),
        operatorProvisioningService.listSchoolSummaries(),
      ]);
      if (!isCurrentRequest()) return;
      setAccess(entries);
      setSchools(
        schoolList
          .filter((school) => school.active && !school.deletedAt)
          .map((school) => ({
            id: school.id,
            label: `${school.name} (${school.organizationName})`,
          })),
      );
      const rolesForActiveSchools = await Promise.allSettled(
        entries
          .filter((entry) => entry.status === "active" && entry.schoolActive)
          .map(
            async (entry) =>
              [
                entry.tenantId,
                await accountTenantAccessService.listAssignableRoles(
                  accountId,
                  entry.tenantId,
                ),
              ] as const,
          ),
      );
      if (!isCurrentRequest()) return;
      setRolesBySchool(
        Object.fromEntries(
          rolesForActiveSchools.flatMap((result) => {
            if (result.status === "fulfilled") return [result.value];
            logger.warn("failed to load assignable school roles", {
              error:
                result.reason instanceof Error
                  ? result.reason.message
                  : String(result.reason),
              accountId,
            });
            return [];
          }),
        ),
      );
    } catch (error) {
      if (!isCurrentRequest()) return;
      logger.error("failed to load school access", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
      });
      setErrorMessage(
        error instanceof AccountTenantAccessApiError
          ? error.message
          : "Die Schulzugänge konnten nicht geladen werden.",
      );
    } finally {
      if (isCurrentRequest()) setLoading(false);
    }
  }, [accountId]);

  useEffect(() => {
    if (isOpen) {
      void load();
    } else {
      loadRequestRef.current += 1;
      setAddSchoolId("");
      setAddRoleId("");
      setFirstName("");
      setLastName("");
      setErrorMessage("");
      setRolesBySchool({});
    }
  }, [isOpen, load]);

  useEffect(() => {
    if (!isOpen || !addSchoolId || rolesBySchool[addSchoolId]) return;
    void accountTenantAccessService
      .listAssignableRoles(accountId, addSchoolId)
      .then((schoolRoles) =>
        setRolesBySchool((current) => ({
          ...current,
          [addSchoolId]: schoolRoles,
        })),
      )
      .catch((error) => {
        logger.error("failed to load assignable school roles", {
          error: error instanceof Error ? error.message : String(error),
          accountId,
          schoolId: addSchoolId,
        });
        setErrorMessage("Die Rollen der Schule konnten nicht geladen werden.");
      });
  }, [accountId, addSchoolId, isOpen, rolesBySchool]);

  // Schools the account can still be added to.
  const availableSchools = useMemo(() => {
    const activeTenants = new Set(
      access
        .filter((entry) => entry.status === "active")
        .map((entry) => entry.tenantId),
    );
    return schools.filter((school) => !activeTenants.has(school.id));
  }, [access, schools]);

  const knowsName = access.some((entry) => entry.hasPerson);
  const needsName = !knowsName;

  function rolesForSchool(schoolId: string) {
    // Shared exclusion set (guardian, legacy teacher). lehrkraft (#1772) is
    // assignable since the class day view shipped; it lives in the school
    // portal since the cutover (#2207).
    return (rolesBySchool[schoolId] ?? []).filter((role) =>
      isAssignableStaffRole(role.name),
    );
  }

  function entryIsLehrkraft(entry: AccountTenantAccess): boolean {
    return entry.roles.some((role) => role.name.toLowerCase() === "lehrkraft");
  }

  // Rollenwechsel-Ziele eines bestehenden Zugangs: Lehrkraft ist für
  // Betreuungs-/Verwaltungs-Einträge gesperrt — UpdateAccountTenantRole
  // tauscht nur die auth-Rolle und würde das Betreuungsprofil
  // (users.teachers) samt aktiver Gruppen-Aufsichten stehen lassen, während
  // das Konto nur noch class_day-Rechte trägt (Spiegel der Härtung in
  // role-management-modal). Die Gegenrichtung bleibt erlaubt: für einen
  // Lehrkraft-Eintrag legt das Backend beim Wechsel auf Betreuung/Verwaltung
  // das fehlende Profil über ensureSchoolIdentity an. Beim Ergänzen eines
  // NEUEN Schulzugangs (rolesForSchool direkt) ist Lehrkraft weiterhin
  // wählbar.
  function roleChangeOptions(entry: AccountTenantAccess) {
    const options = rolesForSchool(entry.tenantId);
    if (entryIsLehrkraft(entry)) return options;
    return options.filter((role) => role.name.toLowerCase() !== "lehrkraft");
  }

  async function runMutation(
    action: () => Promise<AccountTenantAccess[]>,
    successMessage: string,
    failureMessage: string,
  ) {
    try {
      setSaving(true);
      setErrorMessage("");
      setAccess(await action());
      toastSuccess(successMessage);
      await onUpdated?.();
      return true;
    } catch (error) {
      logger.error(failureMessage, {
        error: error instanceof Error ? error.message : String(error),
        accountId,
      });
      const message =
        error instanceof AccountTenantAccessApiError
          ? error.message
          : failureMessage;
      setErrorMessage(message);
      toastError(message);
      return false;
    } finally {
      setSaving(false);
    }
  }

  async function handleGrant() {
    if (!addSchoolId || !addRoleId) return;
    const ok = await runMutation(
      () =>
        accountTenantAccessService.grant(accountId, {
          schoolId: addSchoolId,
          roleId: addRoleId,
          firstName: firstName.trim() || undefined,
          lastName: lastName.trim() || undefined,
        }),
      "Schulzugang wurde erteilt.",
      "Der Schulzugang konnte nicht erteilt werden.",
    );
    if (ok) {
      setAddSchoolId("");
      setAddRoleId("");
      setFirstName("");
      setLastName("");
    }
  }

  async function handleRoleChange(entry: AccountTenantAccess, roleId: string) {
    await runMutation(
      () =>
        accountTenantAccessService.updateRole(
          accountId,
          entry.tenantId,
          roleId,
        ),
      `Rolle an ${entry.schoolName} wurde geändert.`,
      "Die Rolle konnte nicht geändert werden.",
    );
  }

  async function handleRevoke() {
    const target = revokeTarget;
    if (!target) return;
    const ok = await runMutation(
      () => accountTenantAccessService.revoke(accountId, target.tenantId),
      `Zugang zu ${target.schoolName} wurde entzogen.`,
      "Der Zugang konnte nicht entzogen werden.",
    );
    if (ok) setRevokeTarget(null);
  }

  // The role select shows the administrative role; Betreuung is managed by
  // "Betreuung verwalten" and therefore never removed implicitly.
  function primaryRoleId(entry: AccountTenantAccess): string {
    const assignableRoleIDs = new Set(
      rolesForSchool(entry.tenantId).map((role) => role.id),
    );
    const assignableRoles = entry.roles.filter((role) =>
      assignableRoleIDs.has(role.id),
    );
    return (
      assignableRoles.find((role) => role.name.toLowerCase() !== "user")?.id ??
      assignableRoles[0]?.id ??
      ""
    );
  }

  const activeEntries = access.filter((entry) => entry.status === "active");
  const formerEntries = access.filter((entry) => entry.status !== "active");

  return (
    <>
      <FormModal
        // Ausgeblendet, solange die Entzugs-Bestätigung offen ist — nie zwei
        // eigenständige Dialoge übereinander (#2774).
        isOpen={isOpen && revokeTarget === null}
        onClose={onClose}
        title={`Schulzugänge: ${accountLabel}`}
        size="xl"
        footer={
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Schließen
          </Button>
        }
      >
        {loading ? (
          <div className="py-8 text-sm text-gray-500">Wird geladen...</div>
        ) : (
          <div className="space-y-6">
            <p className="text-sm text-gray-600">
              {accountEmail} kann sich an folgenden Schulen anmelden. Die Rolle
              gilt jeweils nur für die dort aufgeführte Schule.
            </p>

            <section className="space-y-2">
              <ConceptSectionHeader
                title="Aktive Zugänge"
                concept="schools"
                // FormModal rendert seinen Titel als h3.
                level={4}
              />
              {activeEntries.length === 0 ? (
                <EmptyState
                  icon={<MotoConceptIcon concept="schools" size={22} />}
                  title="Kein aktiver Schulzugang"
                  description="Dieses Konto ist derzeit keiner Schule zugeordnet und kann sich nicht anmelden."
                />
              ) : (
                <ul className="divide-y divide-gray-200 rounded-2xl border border-gray-200 bg-white">
                  {activeEntries.map((entry) => (
                    <li
                      key={entry.tenantId}
                      className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between"
                    >
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="truncate text-sm font-medium text-gray-900">
                            {entry.schoolName}
                          </span>
                          <StatusBadge
                            tone={statusTone(entry.status)}
                            label={statusLabel(entry.status)}
                          />
                          {!entry.hasStaff && (
                            <StatusBadge
                              tone="orange"
                              label="Ohne Personaldaten"
                            />
                          )}
                        </div>
                        <p className="mt-0.5 truncate text-xs text-gray-500">
                          {entry.organizationName}
                          {entry.roles.length > 0 &&
                            ` • ${entry.roles.map((role) => roleLabel(role.name)).join(", ")}`}
                        </p>
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <div className="w-44">
                          <CustomSelect
                            value={primaryRoleId(entry)}
                            options={roleChangeOptions(entry).map((role) => ({
                              value: role.id,
                              label: roleLabel(role.name),
                            }))}
                            onChange={(value) =>
                              void handleRoleChange(entry, value)
                            }
                            disabled={
                              saving ||
                              !entry.schoolActive ||
                              !rolesBySchool[entry.tenantId]
                            }
                            ariaLabel={`Rolle an ${entry.schoolName}`}
                            placeholder="Rolle wählen"
                          />
                        </div>
                        <Button
                          type="button"
                          variant="outline_danger"
                          size="md"
                          disabled={saving || accessRevocationBlocked(entry)}
                          onClick={() => setRevokeTarget(entry)}
                          title={
                            accessRevocationBlocked(entry)
                              ? "Diese Rolle wird über ihren eigenen Verwaltungsablauf entfernt."
                              : undefined
                          }
                        >
                          Entziehen
                        </Button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className="space-y-3 rounded-2xl border border-gray-200 bg-gray-50 p-4">
              {/* h4 wie die beiden Geschwister-Sektionen: FormModal rendert
                  seinen Titel als h3. */}
              <h4 className="flex items-center gap-2 text-sm font-semibold text-gray-900">
                <Plus className="h-4 w-4 text-gray-500" />
                Schulzugang ergänzen
              </h4>
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <label
                    htmlFor="account-access-school"
                    className="mb-1 block text-xs font-medium text-gray-600"
                  >
                    Schule
                  </label>
                  <CustomSelect
                    id="account-access-school"
                    value={addSchoolId}
                    options={availableSchools.map((school) => ({
                      value: school.id,
                      label: school.label,
                    }))}
                    onChange={(schoolId) => {
                      setAddSchoolId(schoolId);
                      setAddRoleId("");
                    }}
                    disabled={saving || availableSchools.length === 0}
                    placeholder={
                      availableSchools.length === 0
                        ? "Alle Schulen bereits zugeordnet"
                        : "Schule wählen"
                    }
                  />
                </div>
                <div>
                  <label
                    htmlFor="account-access-role"
                    className="mb-1 block text-xs font-medium text-gray-600"
                  >
                    Rolle
                  </label>
                  <CustomSelect
                    id="account-access-role"
                    value={addRoleId}
                    options={rolesForSchool(addSchoolId).map((role) => ({
                      value: role.id,
                      label: roleLabel(role.name),
                    }))}
                    onChange={setAddRoleId}
                    disabled={
                      saving || !addSchoolId || !rolesBySchool[addSchoolId]
                    }
                    placeholder="Rolle wählen"
                  />
                </div>
              </div>

              {needsName && (
                <div className="grid gap-3 sm:grid-cols-2">
                  <Input
                    label="Vorname"
                    name="firstName"
                    value={firstName}
                    onChange={(event) => setFirstName(event.target.value)}
                    placeholder="Vorname"
                  />
                  <Input
                    label="Nachname"
                    name="lastName"
                    value={lastName}
                    onChange={(event) => setLastName(event.target.value)}
                    placeholder="Nachname"
                  />
                </div>
              )}

              <p className="flex items-start gap-2 text-xs text-gray-500">
                <MotoConceptIcon
                  concept="permissions"
                  size={16}
                  className="mt-0.5"
                />
                Die Betreuungsrolle bleibt beim Rollenwechsel erhalten. Sie wird
                über &quot;Betreuung verwalten&quot; vergeben und entfernt.
              </p>

              <div className="flex justify-end">
                <Button
                  type="button"
                  variant="primary"
                  size="md"
                  disabled={saving || !addSchoolId || !addRoleId}
                  onClick={() => void handleGrant()}
                >
                  {saving ? "Wird gespeichert..." : "Zugang erteilen"}
                </Button>
              </div>
            </section>

            {formerEntries.length > 0 && (
              <section className="space-y-2">
                <ConceptSectionHeader
                  title="Frühere Zugänge"
                  concept="changeHistory"
                  // FormModal rendert seinen Titel als h3.
                  level={4}
                />
                <ul className="divide-y divide-gray-200 rounded-2xl border border-gray-200 bg-white">
                  {formerEntries.map((entry) => (
                    <li
                      key={entry.tenantId}
                      className="flex items-center justify-between gap-3 p-4"
                    >
                      <div className="min-w-0">
                        <span className="truncate text-sm text-gray-700">
                          {entry.schoolName}
                        </span>
                        <p className="mt-0.5 truncate text-xs text-gray-500">
                          {entry.organizationName}
                        </p>
                      </div>
                      <StatusBadge
                        tone={statusTone(entry.status)}
                        label={statusLabel(entry.status)}
                      />
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <Alert type="error" message={errorMessage} />
          </div>
        )}
      </FormModal>

      <ConfirmationModal
        isOpen={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        onConfirm={() => void handleRevoke()}
        title="Schulzugang entziehen"
        confirmText="Zugang entziehen"
        isConfirmLoading={saving}
        confirmVariant="danger"
      >
        <p className="text-sm text-gray-600">
          {`${accountLabel} verliert den Zugang zu ${revokeTarget?.schoolName ?? ""} und alle dort vergebenen Rollen. Vorhandene Personaldaten bleiben für die Historie erhalten und werden von der Schule über "Personal löschen" entfernt.`}
        </p>
        {revokeTarget &&
          access.filter((e) => e.status === "active").length === 1 && (
            <p className="text-moto-orange-strong mt-3 text-sm">
              Das ist der letzte aktive Schulzugang. Das Konto wird dadurch
              deaktiviert und kann sich nirgendwo mehr anmelden.
            </p>
          )}
      </ConfirmationModal>
    </>
  );
}
