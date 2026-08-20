"use client";

import { ModalProvider } from "@/components/dashboard/modal-context";
import { NotificationBridge } from "~/components/notifications/notification-bridge";
import { ServiceWorkerRegistrar } from "~/components/notifications/service-worker-registrar";
import { ToastProvider } from "~/contexts/ToastContext";
import { RateLimitBridge } from "~/components/rate-limit-bridge";

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
  return (
    <ModalProvider>
      <ToastProvider>
        <RateLimitBridge />
        <NotificationBridge />
        <ServiceWorkerRegistrar />
        {children}
      </ToastProvider>
    </ModalProvider>
  );
}
