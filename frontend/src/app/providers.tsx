"use client";

import { ModalProvider } from "@/components/dashboard/modal-context";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { AlertProvider } from "~/contexts/AlertContext";
import { ToastProvider } from "~/contexts/ToastContext";
import { AuthWrapper } from "~/components/auth-wrapper";

/**
 * Application Providers
 *
 * BetterAuth uses cookies for session management, so no SessionProvider is needed.
 * Session state is managed via the auth-client module.
 */
export function Providers({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <AuthWrapper>
      <ProfileProvider>
        <SupervisionProvider>
          <ModalProvider>
            <AlertProvider>
              <ToastProvider>{children}</ToastProvider>
            </AlertProvider>
          </ModalProvider>
        </SupervisionProvider>
      </ProfileProvider>
    </AuthWrapper>
  );
}
