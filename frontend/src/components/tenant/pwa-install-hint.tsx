"use client";

import { useEffect, useState } from "react";
import { EllipsisVertical, Share, X } from "lucide-react";
import { Button } from "~/components/ui/button";

const DISMISS_KEY = "moto-pwa-install-hint-dismissed";

export type InstallPlatform = "ios" | "android";

/** True on iPhone/iPad (iPadOS reports itself as MacIntel with touch). */
export function isIosDevice(nav: Navigator): boolean {
  if (/iphone|ipad|ipod/i.test(nav.userAgent)) return true;
  return nav.platform === "MacIntel" && nav.maxTouchPoints > 1;
}

/** True only in Safari, where the iOS home-screen action is available. */
export function isIosSafari(nav: Navigator): boolean {
  if (!isIosDevice(nav)) return false;
  return (
    /version\/[\d.]+.*safari\//i.test(nav.userAgent) &&
    !/(crios|fxios|edgios|opios)/i.test(nav.userAgent)
  );
}

/** True on Android phones and tablets. */
export function isAndroidDevice(nav: Navigator): boolean {
  return /android/i.test(nav.userAgent);
}

/** True when running inside an installed PWA (standalone display mode). */
export function isStandaloneDisplay(win: Window): boolean {
  if (win.matchMedia("(display-mode: standalone)").matches) return true;
  const nav = win.navigator as Navigator & { standalone?: boolean };
  return nav.standalone === true;
}

/**
 * Subtle, dismissible hint for iOS Safari and Android users browsing the
 * tenant app in a normal browser tab: add moto to the home screen from THIS
 * subdomain so the installed app keeps its standalone scope (issue #2010 —
 * both platforms show browser chrome after a cross-origin redirect, so
 * installing on the root host breaks fullscreen). Never rendered inside an
 * already-installed PWA or on desktop devices.
 */
export function PwaInstallHint() {
  const [platform, setPlatform] = useState<InstallPlatform | null>(null);

  useEffect(() => {
    let detected: InstallPlatform | null = null;
    if (isIosSafari(window.navigator)) detected = "ios";
    else if (isAndroidDevice(window.navigator)) detected = "android";
    if (!detected) return;
    if (isStandaloneDisplay(window)) return;
    try {
      if (window.localStorage.getItem(DISMISS_KEY) === "1") return;
    } catch {
      // Private mode can block storage access; show the hint anyway.
    }
    setPlatform(detected);
  }, []);

  if (!platform) return null;

  const dismiss = () => {
    setPlatform(null);
    try {
      window.localStorage.setItem(DISMISS_KEY, "1");
    } catch {
      // Without storage the hint reappears next visit; acceptable.
    }
  };

  const PlatformIcon = platform === "ios" ? Share : EllipsisVertical;

  return (
    <div className="moto-content-surface fixed inset-x-4 bottom-[calc(5.5rem+env(safe-area-inset-bottom))] z-40 rounded-2xl border p-4 shadow-lg sm:right-6 sm:left-auto sm:max-w-sm lg:bottom-6">
      <div className="flex items-start gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D]/10">
          <PlatformIcon className="h-4 w-4 text-[#669f21]" aria-hidden="true" />
        </div>
        <div className="min-w-0 text-sm text-gray-700">
          <p className="font-semibold text-gray-900">moto als App nutzen</p>
          {platform === "ios" ? (
            <p className="mt-1">
              Tippen Sie in Safari auf{" "}
              <span className="font-medium">Teilen</span> und dann auf{" "}
              <span className="font-medium">Zum Home-Bildschirm</span>. So
              öffnet sich moto immer im Vollbild.
            </p>
          ) : (
            <p className="mt-1">
              Öffnen Sie das Browser-Menü und tippen Sie auf{" "}
              <span className="font-medium">App installieren</span> oder{" "}
              <span className="font-medium">
                Zum Startbildschirm hinzufügen
              </span>
              . So öffnet sich moto immer im Vollbild.
            </p>
          )}
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
