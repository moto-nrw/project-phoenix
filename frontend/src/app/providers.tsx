"use client";

import { SessionProvider } from "next-auth/react";
import { ModalProvider } from "@/components/dashboard/modal-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { AlertProvider } from "~/contexts/AlertContext";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider
      // Check session every 4 minutes (240 seconds)
      // This ensures we attempt refresh before the 15-minute token expires
      refetchInterval={4 * 60}
      // Disable focus refetch to avoid duplicate session calls (interval handles refresh)
      refetchOnWindowFocus={false}
    >
      <SupervisionProvider>
        <ModalProvider>
          <AlertProvider>
            {children}
          </AlertProvider>
        </ModalProvider>
      </SupervisionProvider>
    </SessionProvider>
  );
}
