import { toAssignableRoleOptions, type RoleOption } from "~/lib/auth-helpers";
import { authService } from "~/lib/auth-service";

/**
 * Die Systemrolle eines Kontos, wie der Bearbeiten-Zustand der Personalakte
 * sie braucht (#3116): die wählbaren Rollen, die aktuell zugewiesenen und ob
 * das Konto eine Lehrkraft ist.
 */
export interface AccountRoleAssignment {
  readonly options: readonly RoleOption[];
  readonly currentRoleIds: readonly string[];
  /**
   * Ein Lehrkraft-Konto (#1772) hat kein Betreuungsprofil. Ein Tausch auf
   * Betreuung oder Verwaltung würde die volle Rolle ohne
   * `users.teachers`-Zeile hinterlassen; solche Wechsel laufen über
   * Offboarding und Neuanlage, deshalb ist die Rolle dann nicht änderbar.
   */
  readonly currentIsLehrkraft: boolean;
}

export async function loadAccountRoleAssignment(
  accountId: string,
): Promise<AccountRoleAssignment> {
  const [accountRoles, allRoles] = await Promise.all([
    authService.getAccountRoles(accountId),
    authService.getRoles(),
  ]);
  return {
    // Die Systemrolle lehrkraft ist bewusst NICHT wählbar: der Rollen-Tausch
    // würde nur die auth-Rolle wechseln und das Betreuungsprofil samt aktiver
    // Gruppen-Aufsichten stehen lassen, genau die Kombination, die Einladung
    // und Operator-Provisioning verbieten. Gleichnamige Rollen der Schule
    // bleiben dagegen regulär wählbar.
    options: toAssignableRoleOptions(
      allRoles.filter(
        (role) =>
          (role.isSystem !== true || role.name.toLowerCase() !== "lehrkraft") &&
          role.baseRole?.toLowerCase() !== "guardian",
      ),
    ),
    currentRoleIds: accountRoles.map((role) => role.id),
    currentIsLehrkraft: accountRoles.some(
      (role) =>
        role.isSystem === true && role.name.toLowerCase() === "lehrkraft",
    ),
  };
}

/**
 * Tauscht die Rolle eines Kontos atomar im Backend.
 */
export async function replaceAccountRole(
  accountId: string,
  targetRoleId: string,
): Promise<void> {
  await authService.replaceAccountRole(accountId, targetRoleId);
}
