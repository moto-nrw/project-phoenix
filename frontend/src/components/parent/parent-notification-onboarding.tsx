"use client";

import { NotificationSetupDialog } from "~/components/notifications/notification-setup-dialog";

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
  return <NotificationSetupDialog portal="parent" accountId={accountId} />;
}
