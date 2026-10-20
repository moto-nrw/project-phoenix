"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { MobilePageHelpLink } from "~/components/help/mobile-page-help-link";

const MOBILE_PAGE_HELP_TARGET = "[data-mobile-page-help-target]";

/** Places contextual help into the current portal page header. */
export function MobilePageHelpPortal() {
  const [target, setTarget] = useState<HTMLElement | null>(null);

  useEffect(() => {
    const updateTarget = () => {
      const next = document.querySelector<HTMLElement>(MOBILE_PAGE_HELP_TARGET);
      setTarget((current) => (current === next ? current : next));
    };

    updateTarget();
    const observer = new MutationObserver(updateTarget);
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, []);

  return target ? createPortal(<MobilePageHelpLink />, target) : null;
}
