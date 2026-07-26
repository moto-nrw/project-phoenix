"use client";

import { useEffect, useState } from "react";
import { Share, X } from "lucide-react";
import { Button } from "~/components/ui/button";

const DISMISS_KEY = "moto-ios-install-hint-dismissed";

/** True on iPhone/iPad (iPadOS reports itself as MacIntel with touch). */
export function isIosDevice(nav: Navigator): boolean {
  if (/iphone|ipad|ipod/i.test(nav.userAgent)) return true;
  return nav.platform === "MacIntel" && nav.maxTouchPoints > 1;
}

/** True when running inside an installed PWA (standalone display mode). */
export function isStandaloneDisplay(win: Window): boolean {
  if (win.matchMedia("(display-mode: standalone)").matches) return true;
  const nav = win.navigator as Navigator & { standalone?: boolean };
  return nav.standalone === true;
}

/**
 * Subtle, dismissible hint for iOS users browsing the tenant app in Safari:
 * add moto to the home screen from THIS subdomain so the installed app keeps
 * its standalone scope (issue #2010 — iOS shows browser chrome after any
 * cross-origin redirect, so installing on the root host breaks fullscreen).
 * Never rendered inside an already-installed PWA or on non-iOS devices.
 */
export function IosInstallHint() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!isIosDevice(window.navigator)) return;
    if (isStandaloneDisplay(window)) return;
    try {
      if (window.localStorage.getItem(DISMISS_KEY) === "1") return;
    } catch {
      // Private mode can block storage access; show the hint anyway.
    }
    setVisible(true);
  }, []);

  if (!visible) return null;

  const dismiss = () => {
    setVisible(false);
    try {
      window.localStorage.setItem(DISMISS_KEY, "1");
    } catch {
      // Without storage the hint reappears next visit; acceptable.
    }
  };

  return (
    <div className="moto-content-surface fixed inset-x-4 bottom-4 z-40 rounded-2xl border p-4 shadow-lg sm:right-6 sm:left-auto sm:max-w-sm">
      <div className="flex items-start gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10">
          <Share className="h-4 w-4 text-[#669f21]" aria-hidden="true" />
        </div>
        <div className="min-w-0 text-sm text-gray-700">
          <p className="font-semibold text-gray-900">moto als App nutzen</p>
          <p className="mt-1">
            Tippen Sie in Safari auf <span className="font-medium">Teilen</span>{" "}
            und dann auf{" "}
            <span className="font-medium">Zum Home-Bildschirm</span>. So öffnet
            sich moto immer im Vollbild.
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Hinweis schließen"
          onClick={dismiss}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}
