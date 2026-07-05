"use client";

import { ModalProvider } from "@/components/dashboard/modal-context";
import { ToastProvider } from "~/contexts/ToastContext";

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
      <ToastProvider>{children}</ToastProvider>
    </ModalProvider>
  );
}
