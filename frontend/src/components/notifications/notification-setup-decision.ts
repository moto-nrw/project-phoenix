import type { PushPortal } from "~/lib/push-api";

interface StoredDecision {
  readonly done?: boolean;
  readonly remindAfter?: number;
}

export function setupStorageKey(
  portal: PushPortal,
  accountId: string,
  tenantSlug?: string | null,
): string {
  const tenantScope = portal === "tenant" && tenantSlug ? `.${tenantSlug}` : "";
  return `moto.${portal}.notification-setup.v1${tenantScope}.${accountId}`;
}

export function shouldStartNotificationSetup(storageKey: string): boolean {
  try {
    const value = localStorage.getItem(storageKey);
    const decision = value ? (JSON.parse(value) as StoredDecision) : {};
    return !(
      decision.done ||
      (decision.remindAfter !== undefined && decision.remindAfter > Date.now())
    );
  } catch {
    // The setup stays available when browser storage is unavailable.
    return true;
  }
}
