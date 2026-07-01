"use client";

import type { ReactNode } from "react";
import { createContext, useContext, useMemo } from "react";
import type { Session } from "next-auth";
import { mockSessionData } from "../fixtures/session";

type SessionStatus = "authenticated" | "loading" | "unauthenticated";

interface StorybookSessionContext {
  data: Session | null;
  status: SessionStatus;
  update: () => Promise<Session | null>;
}

const defaultSession = mockSessionData();
let currentSession: Session | null = defaultSession;

const SessionContext = createContext<StorybookSessionContext>({
  data: defaultSession,
  status: "authenticated",
  update: async () => currentSession,
});

export function SessionProvider({
  children,
  session = defaultSession,
}: Readonly<{
  children: ReactNode;
  session?: Session | null;
}>) {
  currentSession = session;
  const value = useMemo<StorybookSessionContext>(
    () => ({
      data: session,
      status: session ? "authenticated" : "unauthenticated",
      update: async () => session,
    }),
    [session],
  );

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession() {
  return useContext(SessionContext);
}

export async function getSession() {
  return currentSession;
}

export async function signIn() {
  return undefined;
}

export async function signOut() {
  currentSession = null;
  return undefined;
}
