"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { MobilePageHelpLink } from "~/components/help/mobile-page-help-link";
import { MobilePageHelpTargetContext } from "./mobile-page-help-target";

interface MobilePageHelpPortalProps {
  readonly children: ReactNode;
}

/** Places contextual help into a registered portal page header. */
export function MobilePageHelpPortal({
  children,
}: Readonly<MobilePageHelpPortalProps>) {
  const [target, setTarget] = useState<HTMLElement | null>(null);
  const registerTarget = useCallback((nextTarget: HTMLElement) => {
    setTarget(nextTarget);
  }, []);
  const unregisterTarget = useCallback((currentTarget: HTMLElement) => {
    setTarget((target) => (target === currentTarget ? null : target));
  }, []);
  const registration = useMemo(
    () => ({ registerTarget, unregisterTarget }),
    [registerTarget, unregisterTarget],
  );

  return (
    <MobilePageHelpTargetContext.Provider value={registration}>
      {children}
      {target ? createPortal(<MobilePageHelpLink />, target) : null}
    </MobilePageHelpTargetContext.Provider>
  );
}
