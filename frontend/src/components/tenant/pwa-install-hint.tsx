"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { useTranslations } from "next-intl";
import { Download, EllipsisVertical, Share, X } from "lucide-react";
import { Button, ButtonLink } from "~/components/ui/button";
import { trackEvent } from "~/lib/analytics";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  canPromptInstall,
  createChromeIntentUrl,
  dismissInstallHint,
  isAndroidDevice,
  isInstallHintEligible,
  isInstallationCompleted,
  isSamsungInternet,
  subscribeInstallPrompt,
  triggerInstallPrompt,
} from "~/lib/pwa-install-prompt";

export { isAndroidDevice, isSamsungInternet } from "~/lib/pwa-install-prompt";

const logger = createLogger({ component: "PwaInstallHint" });

export type InstallPlatform = "ios" | "android" | "samsung";

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

/** True when running inside an installed PWA (standalone display mode). */
export function isStandaloneDisplay(win: Window): boolean {
  if (win.matchMedia("(display-mode: standalone)").matches) return true;
  const nav = win.navigator as Navigator & { standalone?: boolean };
  return nav.standalone === true;
}

export function SamsungChromeInstructions({
  pageUrl,
}: Readonly<{ pageUrl: URL }>) {
  const t = useTranslations("pwaInstallHint");
  return (
    <>
      <p className="mt-1">{t("samsungDescription")}</p>
      <p className="mt-2">
        {t("samsungStep")} <span className="font-medium">{t("install")}</span>.
      </p>
      <ButtonLink
        href={createChromeIntentUrl(pageUrl)}
        size="md"
        className="mt-3"
      >
        {t("openInChrome")}
      </ButtonLink>
      <p className="mt-2 text-gray-600">
        {t("samsungFallback", { host: pageUrl.host })}
      </p>
    </>
  );
}

/**
 * Floating, dismissible install promotion for iOS Safari and Android users
 * browsing the tenant app in a normal browser tab: add moto to the home
 * screen from THIS subdomain so the installed app keeps its standalone scope
 * (issue #2010 — both platforms show browser chrome after a cross-origin
 * redirect, so installing on the root host breaks fullscreen).
 *
 * Mounted in the protected layout on purpose. The audience is staff on phones
 * and tablets, and most of them never reach the admin-only dashboard, so a
 * single in-page slot would not reach them.
 *
 * Geometry, all of it previously wrong:
 * - Switches at `lg`, where AppShell drops its mobile bottom padding and the
 *   bottom nav disappears. The centered card and right-aligned search-page
 *   FAB do not intersect at `lg` widths, so no FAB clearance is needed there.
 * - Below lg the offset clears the bottom nav (~4.75rem visible) plus a 1rem
 *   gap, and adds `--moto-floating-fab-offset` only on pages that actually
 *   show a check-in or create FAB. An active mobile check-in bar publishes
 *   `--moto-checkin-bar-offset` in the same way. The old fixed 10.5rem reserved
 *   FAB space everywhere.
 * - Horizontally centered at every width instead of jumping from an
 *   edge-to-edge mobile banner to a right-anchored card at `sm`.
 *
 * Never rendered on desktop, inside an already-installed PWA, before the
 * second visit, or after the user dismissed it.
 */
export function PwaInstallHint() {
  const t = useTranslations("pwaInstallHint");
  const [platform, setPlatform] = useState<InstallPlatform | null>(null);

  // Chrome may fire beforeinstallprompt before or after this mounts, so read
  // it from the module-level store rather than a local listener.
  const oneTapInstallReady = useSyncExternalStore(
    subscribeInstallPrompt,
    canPromptInstall,
    () => false,
  );
  const installationCompleted = useSyncExternalStore(
    subscribeInstallPrompt,
    isInstallationCompleted,
    () => false,
  );

  useEffect(() => {
    let detected: InstallPlatform | null = null;
    if (isIosSafari(window.navigator)) detected = "ios";
    else if (isSamsungInternet(window.navigator)) detected = "samsung";
    else if (isAndroidDevice(window.navigator)) detected = "android";
    if (!detected) return;
    if (isStandaloneDisplay(window)) return;
    if (!isInstallHintEligible(window)) return;
    setPlatform(detected);
  }, []);

  useEffect(() => {
    if (!installationCompleted) return;
    // Anonymous install-funnel signal (#2189): the appinstalled event fired.
    trackEvent("pwa_installed");
    setPlatform(null);
  }, [installationCompleted]);

  // Install-funnel signal (#2189): the install-hint card became visible —
  // any platform, so the shown count covers the manual iOS card too.
  useEffect(() => {
    if (!platform) return;
    trackEvent("pwa_install_prompt_shown");
  }, [platform]);

  if (!platform || installationCompleted) return null;

  const dismiss = () => {
    setPlatform(null);
    dismissInstallHint(window);
  };

  const install = () => {
    void triggerInstallPrompt()
      .then((outcome) => {
        logger.info("pwa_install_prompt_answered", { outcome });
        if (outcome === "accepted") trackEvent("pwa_install_prompt_accepted");
        if (outcome === "dismissed") trackEvent("pwa_install_prompt_dismissed");
        // A declined prompt leaves the card in place so the user can retry.
        if (outcome === "accepted") dismiss();
      })
      .catch((err: unknown) => {
        logger.error("pwa_install_prompt_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  };

  // Android with a captured event gets a real one-tap install. Without one
  // (event missed, or the browser exposes no install action) it falls back to
  // the same manual instructions iOS needs, which is all Safari ever allows.
  const showOneTapInstall = platform === "android" && oneTapInstallReady;
  const PlatformIcon = showOneTapInstall
    ? Download
    : platform === "ios"
      ? Share
      : EllipsisVertical;

  return (
    // Deliberately NOT `moto-content-surface`: that utility is unlayered and
    // hardcodes a white background plus a gray-200 border, which Tailwind
    // utilities cannot override and which made this card blend into the page.
    // A brand-green tint (GROUP_ROOM_SHADES.bgHover) and a 2px brand border
    // separate the promotion from the neutral content cards around it.
    <section
      aria-labelledby="pwa-install-hint-title"
      className="fixed bottom-[calc(5.75rem+var(--moto-floating-fab-offset,0rem)+var(--moto-checkin-bar-offset,0rem)+env(safe-area-inset-bottom))] left-1/2 z-40 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 rounded-2xl border-2 p-4 shadow-lg lg:bottom-8"
      style={{
        borderColor: GROUP_ROOM_SHADES.base,
        backgroundColor: GROUP_ROOM_SHADES.bgHover,
      }}
    >
      <div className="flex items-start gap-3">
        {/* White bubble, because the previous #83CD2D/10 tint is invisible on
            the green card background. */}
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-white">
          <PlatformIcon
            className="h-4 w-4"
            style={{ color: GROUP_ROOM_SHADES.text }}
            aria-hidden="true"
          />
        </div>
        <div className="min-w-0 flex-1 text-sm text-gray-700">
          <h2
            id="pwa-install-hint-title"
            className="font-semibold text-gray-900"
          >
            {t("title")}
          </h2>
          {platform === "samsung" ? (
            <SamsungChromeInstructions
              pageUrl={new URL(window.location.href)}
            />
          ) : showOneTapInstall ? (
            <p className="mt-1">{t("oneTapDescription")}</p>
          ) : platform === "ios" ? (
            <p className="mt-1">
              {t("iosBefore")} <span className="font-medium">{t("share")}</span>{" "}
              {t("iosBetween")}{" "}
              <span className="font-medium">{t("homeScreen")}</span>.
            </p>
          ) : (
            <p className="mt-1">
              {t("androidBefore")}{" "}
              <span className="font-medium">{t("install")}</span>{" "}
              {t("androidOr")}{" "}
              <span className="font-medium">{t("addToHome")}</span>.
            </p>
          )}
          {showOneTapInstall && (
            <Button
              type="button"
              variant="primary"
              size="md"
              className="mt-3"
              onClick={install}
            >
              {t("install")}
            </Button>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={t("close")}
          onClick={dismiss}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </section>
  );
}
