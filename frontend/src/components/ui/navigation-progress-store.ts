"use client";

import { createContext } from "react";

export interface NavigationProgressStore {
  readonly subscribe: (onChange: () => void) => () => void;
  readonly isPending: () => boolean;
  readonly isFallbackSuppressed: () => boolean;
  readonly startNavigation: (target: string) => number;
  readonly startLinkNavigation: (target: string) => number;
  readonly startHistory: (target: string) => void;
  readonly completeNavigation: (currentUrl: string) => void;
  readonly cancelNavigation: (id: number) => void;
}

export const NavigationProgressContext =
  createContext<NavigationProgressStore | null>(null);
