"use client";

import { DeferredNotificationSetup } from "~/components/notifications/deferred-notification-setup";

/**
 * Der Einrichtungs-Dialog des Elternportals.
 *
 * Die Logik liegt seit #2831 in `NotificationSetupDialog`, weil OGS-Portal und
 * Schul-Portal denselben Einstieg brauchen. Hier bleibt nur die Bindung an das
 * Elternportal stehen.
 */
export function ParentNotificationOnboarding({
  accountId,
}: Readonly<{ accountId: string }>) {
  return <DeferredNotificationSetup portal="parent" accountId={accountId} />;
}
