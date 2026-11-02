import type { ReactNode } from "react";
import { SessionProvider } from "next-auth/react";
import { SWRConfig } from "~/lib/swr/testing";

/** Real SWR and parent session, isolated per render. Only API transport is mocked. */
export function ParentResourceTestProvider({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <SessionProvider
      session={{
        user: { id: "parent-test", scope: "parent" },
        expires: "2099-01-01",
      }}
      refetchOnWindowFocus={false}
    >
      <SWRConfig value={{ provider: () => new Map(), errorRetryCount: 0 }}>
        {children}
      </SWRConfig>
    </SessionProvider>
  );
}
