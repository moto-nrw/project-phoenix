"use client";

import dynamic from "next/dynamic";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import type { PushPortal } from "~/lib/push-api";

const NotificationSetupDialog = dynamic(
  () =>
    import("~/components/notifications/notification-setup-dialog").then(
      (module) => module.NotificationSetupDialog,
    ),
  { ssr: false },
);

/**
 * Hängt den geführten Einstieg an die Sitzung des jeweiligen Portals (#2831).
 *
 * Das Elternportal hatte diesen Einstieg von Anfang an, das OGS- und das
 * Schul-Portal nicht. Die Bindung an die Sitzung steht hier, damit der Dialog
 * selbst nichts über NextAuth wissen muss und in Tests ohne Sitzung läuft.
 */
export function PortalNotificationSetup({
  portal,
}: Readonly<{ portal: PushPortal }>) {
  const shell = useShellAuthSafe();
  const accountId = shell?.user?.id;
  if (shell?.status !== "authenticated" || accountId === undefined) return null;
  return <NotificationSetupDialog portal={portal} accountId={accountId} />;
}
