"use client";

import dynamic from "next/dynamic";

const NotificationSetupDialog = dynamic(
  () =>
    import("~/components/notifications/notification-setup-dialog").then(
      (module) => module.NotificationSetupDialog,
    ),
  { ssr: false },
);

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
