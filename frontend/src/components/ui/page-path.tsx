"use client";

import { createContext, useContext, type ReactNode } from "react";

type ResolvePagePath = (path: string) => string;

const PagePathContext = createContext<ResolvePagePath | null>(null);

/**
 * How a portal turns an app path such as `/rooms` into the address of its
 * page (#6577). The tenant area resolves it for the school; every other
 * portal mounts no provider, and the kit uses the path as given.
 */
export function PagePathProvider({
  resolve,
  children,
}: {
  readonly resolve: ResolvePagePath;
  readonly children: ReactNode;
}) {
  return (
    <PagePathContext.Provider value={resolve}>
      {children}
    </PagePathContext.Provider>
  );
}

function asGiven(path: string): string {
  return path;
}

/** The path resolution of the current portal; without one the path itself. */
export function usePagePath(): ResolvePagePath {
  return useContext(PagePathContext) ?? asGiven;
}
