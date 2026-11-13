"use client";

import { createContext, useContext } from "react";

/**
 * Whom a reader turns to when an error needs another person, such as a
 * missing right (#6871). Staff, parents and school accounts ask their
 * school; the Träger portal sends its people to the Leitung of their
 * Träger-Büro. Only the catalog's class texts change, codes keep their own.
 */
export type ErrorContact = "school" | "traeger";

const ErrorContactContext = createContext<ErrorContact>("school");

export function ErrorContactProvider({
  contact,
  children,
}: Readonly<{ contact: ErrorContact; children: React.ReactNode }>) {
  return (
    <ErrorContactContext.Provider value={contact}>
      {children}
    </ErrorContactContext.Provider>
  );
}

export function useErrorContact(): ErrorContact {
  return useContext(ErrorContactContext);
}
