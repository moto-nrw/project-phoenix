"use client";

import { useEffect } from "react";
import { ModalProvider } from "@/components/dashboard/modal-context";
import { NotificationBridge } from "~/components/notifications/notification-bridge";
import { ServiceWorkerRegistrar } from "~/components/notifications/service-worker-registrar";
import { ToastProvider } from "~/contexts/ToastContext";
import { schedulePostHogInitialization } from "~/lib/posthog-client";

/**
 * Root providers — auth-free.
 *
 * SessionProvider has been moved into scope-specific layouts:
 * - Tenant: [tenant]/layout.tsx (basePath="/api/auth")
 * - Operator: operator/layout.tsx (basePath="/api/operator/auth")
 *
 * This prevents operator cookies from leaking to tenant subdomains.
 */
export function Providers({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  useEffect(schedulePostHogInitialization, []);

  return (
    <ModalProvider>
      <ToastProvider>
        <NotificationBridge />
        <ServiceWorkerRegistrar />
        {children}
      </ToastProvider>
    </ModalProvider>
  );
}
