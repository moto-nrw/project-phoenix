import { toAssignableRoleOptions, type RoleOption } from "~/lib/auth-helpers";
import { authService } from "~/lib/auth-service";

/**
 * Die Systemrolle eines Kontos, wie der Bearbeiten-Zustand der Personalakte
 * sie braucht (#3116): die wählbaren Rollen, die aktuell zugewiesenen und ob
 * das Konto eine Lehrkraft ist.
 */
export interface AccountRoleAssignment {
  readonly options: readonly RoleOption[];
  readonly currentRoleIds: readonly number[];
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
    // lehrkraft ist bewusst NICHT wählbar: der Rollen-Tausch würde nur die
    // auth-Rolle wechseln und das Betreuungsprofil samt aktiver
    // Gruppen-Aufsichten stehen lassen, genau die Kombination, die Einladung
    // und Operator-Provisioning verbieten.
    options: toAssignableRoleOptions(allRoles).filter(
      (option) => option.systemName !== "lehrkraft",
    ),
    currentRoleIds: accountRoles.map((role) => Number(role.id)),
    currentIsLehrkraft: accountRoles.some(
      (role) => role.name.toLowerCase() === "lehrkraft",
    ),
  };
}

/**
 * Tauscht die Rolle eines Kontos. Die neue Rolle wird zuerst zugewiesen und
 * die alten danach entfernt, damit ein Abbruch mitten in der Folge das Konto
 * mit einer Rolle zu viel zurücklässt, nie ohne Rolle.
 */
export async function replaceAccountRole(
  accountId: string,
  targetRoleId: number,
  currentRoleIds: readonly number[],
): Promise<void> {
  await authService.assignRoleToAccount(accountId, String(targetRoleId));
  for (const oldId of currentRoleIds.filter((id) => id !== targetRoleId)) {
    await authService.removeRoleFromAccount(accountId, String(oldId));
  }
}
