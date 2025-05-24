"use client";

import { SessionProvider } from "next-auth/react";
import { ModalProvider } from "~/components/dashboard/modal-context";


export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <ModalProvider>
        {children}
      </ModalProvider>
    </SessionProvider>
  );
}
