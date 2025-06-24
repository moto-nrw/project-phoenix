"use client";

import { SessionProvider } from "next-auth/react";
import { ModalProvider } from "@/components/dashboard/modal-context";
import { SupervisionProvider } from "~/lib/supervision-context";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider 
      // Check session every 4 minutes (240 seconds)
      // This ensures we attempt refresh before the 15-minute token expires
      refetchInterval={4 * 60}
      // Also refetch when window regains focus
      refetchOnWindowFocus={true}
    >
      <SupervisionProvider>
        <ModalProvider>
          {children}
        </ModalProvider>
      </SupervisionProvider>
    </SessionProvider>
  );
}
