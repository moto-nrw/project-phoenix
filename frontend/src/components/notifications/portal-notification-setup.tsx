"use client";

import { DeferredNotificationSetup } from "~/components/notifications/deferred-notification-setup";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import type { PushPortal } from "~/lib/push-api";

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
  return <DeferredNotificationSetup portal={portal} accountId={accountId} />;
}
