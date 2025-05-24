"use client";

import { SessionProvider } from "next-auth/react";
import { UserContextProvider } from "~/lib/usercontext-context";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <SessionProvider>
      <UserContextProvider>{children}</UserContextProvider>
    </SessionProvider>
  );
}
