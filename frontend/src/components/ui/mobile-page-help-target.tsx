"use client";

import { createContext, useContext, useEffect, useRef } from "react";

interface MobilePageHelpTargetRegistration {
  registerTarget(target: HTMLElement): void;
  unregisterTarget(target: HTMLElement): void;
}

export const MobilePageHelpTargetContext =
  createContext<MobilePageHelpTargetRegistration | null>(null);

/** Marks the page-header position where the mobile help link belongs. */
export function MobilePageHelpTarget() {
  const registration = useContext(MobilePageHelpTargetContext);
  const targetRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const target = targetRef.current;
    if (!target || !registration) return;
    registration.registerTarget(target);
    return () => registration.unregisterTarget(target);
  }, [registration]);

  return <div ref={targetRef} className="contents" />;
}
