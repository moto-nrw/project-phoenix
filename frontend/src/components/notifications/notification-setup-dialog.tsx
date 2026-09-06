"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
} from "react";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { PushInstallSteps } from "~/components/settings/push-install-steps";
import {
  fetchNotificationPreferences,
  setNotificationPreference,
  type NotificationPreferenceType,
} from "~/lib/notification-preferences-api";
import { createLogger } from "~/lib/logger";
import {
  isPushConfigurationMissing,
  isPushSupported,
  isStandaloneApp,
  needsIOSInstall,
  subscribePush,
  syncExistingPushSubscription,
  verifyPushConfiguration,
  type PushPortal,
} from "~/lib/push-api";
import {
  canPromptInstall,
  isAndroidDevice,
  isInstallationCompleted,
  isSamsungInternet,
  subscribeInstallPrompt,
  triggerInstallPrompt,
} from "~/lib/pwa-install-prompt";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "NotificationSetupDialog" });
const INSTALL_GUIDE_REMIND_MS = 24 * 60 * 60 * 1000;
const REMIND_LATER_MS = 7 * 24 * 60 * 60 * 1000;

type SetupMode = "enable" | "install-ios" | "install-android" | "denied";

interface StoredDecision {
  readonly done?: boolean;
  readonly remindAfter?: number;
}

function readDecision(key: string): StoredDecision {
  try {
    const value = localStorage.getItem(key);
    return value ? (JSON.parse(value) as StoredDecision) : {};
  } catch {
    return {};
  }
}

function writeDecision(key: string, decision: StoredDecision): void {
  try {
    localStorage.setItem(key, JSON.stringify(decision));
  } catch {
    // The setup still works when browser storage is unavailable.
  }
}

export function setupStorageKey(
  portal: PushPortal,
  accountId: string,
  tenantSlug?: string | null,
): string {
  const tenantScope = portal === "tenant" && tenantSlug ? `.${tenantSlug}` : "";
  return `moto.${portal}.notification-setup.v1${tenantScope}.${accountId}`;
}

/**
 * Guided first-run setup for notifications, shared by all three portals
 * (#2831). It answers the two questions people cannot answer themselves —
 * "is moto installed?" and "may moto notify me?" — and offers exactly the
 * next step that is missing on this device.
 *
 * Ran only in the parents portal before. Staff and Lehrkräfte were left to
 * find the switch in the settings on their own, which is the misunderstanding
 * the issue starts from.
 *
 * `restartToken` reopens the dialog from the settings card: a changed value
 * re-runs the inspection, even when this browser stored "done" long ago.
 */
export function NotificationSetupDialog({
  portal,
  accountId,
  restartToken = 0,
  onFinished,
}: Readonly<{
  portal: PushPortal;
  accountId: string;
  restartToken?: number;
  onFinished?: () => void;
}>) {
  const t = useTranslations("parentNotificationSetup");
  const tenantSlug = useTenantSlugSafe();
  const storageKey = useMemo(
    () => setupStorageKey(portal, accountId, tenantSlug),
    [accountId, portal, tenantSlug],
  );
  const [mode, setMode] = useState<SetupMode | null>(null);
  const [types, setTypes] = useState<NotificationPreferenceType[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [installAccepted, setInstallAccepted] = useState(false);
  const installPromptReady = useSyncExternalStore(
    subscribeInstallPrompt,
    canPromptInstall,
    () => false,
  );
  const installationCompleted = useSyncExternalStore(
    subscribeInstallPrompt,
    isInstallationCompleted,
    () => false,
  );

  const close = useCallback(
    (mode: SetupMode | null) => {
      setMode(mode);
      if (mode === null) onFinished?.();
    },
    [onFinished],
  );

  useEffect(() => {
    let active = true;
    const decision = readDecision(storageKey);
    // A restart from the settings card ignores the stored decision on purpose:
    // the person just asked for the setup again.
    if (
      restartToken === 0 &&
      (decision.done ||
        (decision.remindAfter !== undefined &&
          decision.remindAfter > Date.now()))
    ) {
      return;
    }

    const inspect = async () => {
      if (needsIOSInstall()) {
        try {
          await verifyPushConfiguration(portal);
          if (active) setMode("install-ios");
        } catch (err) {
          if (!isPushConfigurationMissing(err)) {
            logger.warn("push_configuration_check_failed", {
              portal,
              error: err instanceof Error ? err.message : String(err),
            });
          }
        }
        return;
      }
      if (
        isAndroidDevice(window.navigator) &&
        !isSamsungInternet(window.navigator) &&
        !isStandaloneApp() &&
        !installationCompleted &&
        !installAccepted
      ) {
        try {
          await verifyPushConfiguration(portal);
          if (active) setMode("install-android");
        } catch (err) {
          if (!isPushConfigurationMissing(err)) {
            logger.warn("push_configuration_check_failed", {
              portal,
              error: err instanceof Error ? err.message : String(err),
            });
          }
        }
        return;
      }
      if (!isPushSupported()) return;
      if (Notification.permission === "denied") {
        if (active) setMode("denied");
        return;
      }

      try {
        const [preferences, subscription] = await Promise.all([
          fetchNotificationPreferences(portal),
          syncExistingPushSubscription(portal),
        ]);
        if (!active) return;
        if (!preferences.tenant_enabled) return;
        setTypes(preferences.types);
        const hasEnabledType = preferences.types.some(
          (type) => type.available && type.enabled,
        );
        if (!subscription || !hasEnabledType) setMode("enable");
        else writeDecision(storageKey, { done: true });
      } catch (err) {
        if (isPushConfigurationMissing(err)) return;
        logger.warn("notification_setup_check_failed", {
          portal,
          error: err instanceof Error ? err.message : String(err),
        });
      }
    };

    void inspect();
    return () => {
      active = false;
    };
  }, [
    installAccepted,
    installationCompleted,
    installPromptReady,
    portal,
    restartToken,
    storageKey,
  ]);

  const remindLater = () => {
    writeDecision(storageKey, { remindAfter: Date.now() + REMIND_LATER_MS });
    close(null);
  };

  const finishInstallGuide = () => {
    writeDecision(storageKey, {
      remindAfter: Date.now() + INSTALL_GUIDE_REMIND_MS,
    });
    close(null);
  };

  const installAndroid = async () => {
    setBusy(true);
    setError(null);
    try {
      const outcome = await triggerInstallPrompt();
      if (outcome === "accepted") {
        // The Android install prompt comes before preferences are inspected.
        // Do not expose the enable action until the follow-up inspection has
        // confirmed that this tenant permits notifications and loaded its types.
        setInstallAccepted(true);
        setMode(null);
      }
    } catch (err) {
      logger.warn("app_install_failed", {
        portal,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(t("installError"));
    } finally {
      setBusy(false);
    }
  };

  const enable = async () => {
    setBusy(true);
    setError(null);
    try {
      const pushRequest = subscribePush(portal);
      const preferenceRequests = types
        .filter((type) => type.available && !type.enabled)
        .map((type) => setNotificationPreference(type.key, true, portal));
      await Promise.all([pushRequest, ...preferenceRequests]);
      writeDecision(storageKey, { done: true });
      close(null);
    } catch (err) {
      logger.warn("notification_setup_failed", {
        portal,
        error: err instanceof Error ? err.message : String(err),
      });
      if (Notification.permission === "denied") setMode("denied");
      setError(t("enableError"));
    } finally {
      setBusy(false);
    }
  };

  if (!mode) return null;

  return (
    <Modal
      isOpen
      onClose={remindLater}
      title={
        mode === "install-ios"
          ? t("installTitle")
          : mode === "install-android"
            ? t("installAndroidTitle")
            : t("title")
      }
      closeLabel={t("close")}
      backdropLabel={t("close")}
      isDismissDisabled={busy}
      mobileSheet
    >
      <div className="space-y-5">
        {/* Anleitungen tragen ihre eigenen Schrittsymbole; ein zweites Symbol
            darüber stünde nur allein in einer Zeile herum. Bei den kurzen
            Texten steht es neben dem Satz, wie im Kopf der Einstellungskarte. */}
        {mode === "install-ios" ? (
          <PushInstallSteps platform="ios" />
        ) : mode === "install-android" && !installPromptReady ? (
          <PushInstallSteps platform="android" />
        ) : (
          <div className="flex items-start gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-gray-100">
              <MotoConceptIcon
                concept={
                  mode.startsWith("install-") ? "devices" : "notifications"
                }
                size={26}
              />
            </div>
            <p className="text-[17px] leading-7 text-gray-700">
              {mode === "install-android"
                ? t("installAndroidIntro")
                : mode === "denied"
                  ? t("denied")
                  : portal === "parent"
                    ? t("intro")
                    : t("introStaff")}
            </p>
          </div>
        )}

        {error && <Alert type="error" message={error} />}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          {mode === "enable" && (
            <Button
              type="button"
              variant="surface"
              size="touch"
              disabled={busy}
              onClick={remindLater}
            >
              {t("later")}
            </Button>
          )}
          {mode === "enable" ? (
            <Button
              type="button"
              size="touch"
              disabled={busy}
              onClick={() => void enable()}
            >
              {busy && (
                <Loader2
                  className="mr-2 h-5 w-5 animate-spin"
                  aria-hidden="true"
                />
              )}
              {t("enable")}
            </Button>
          ) : mode === "install-android" && installPromptReady ? (
            <Button
              type="button"
              size="touch"
              isLoading={busy}
              loadingText={t("installing")}
              onClick={() => void installAndroid()}
            >
              {t("installApp")}
            </Button>
          ) : (
            <Button
              type="button"
              size="touch"
              onClick={
                mode.startsWith("install-") ? finishInstallGuide : remindLater
              }
            >
              {mode.startsWith("install-") ? t("closeGuide") : t("understood")}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  );
}
