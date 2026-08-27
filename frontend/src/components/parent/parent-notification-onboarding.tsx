"use client";

import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
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
} from "~/lib/push-api";
import {
  canPromptInstall,
  isAndroidDevice,
  isInstallationCompleted,
  isSamsungInternet,
  subscribeInstallPrompt,
  triggerInstallPrompt,
} from "~/lib/pwa-install-prompt";

const logger = createLogger({ component: "ParentNotificationOnboarding" });
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

export function ParentNotificationOnboarding({
  accountId,
}: Readonly<{ accountId: string }>) {
  const t = useTranslations("parentNotificationSetup");
  const storageKey = useMemo(
    () => `moto.parent.notification-setup.v1.${accountId}`,
    [accountId],
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

  useEffect(() => {
    let active = true;
    const decision = readDecision(storageKey);
    if (
      decision.done ||
      (decision.remindAfter !== undefined && decision.remindAfter > Date.now())
    ) {
      return;
    }

    const inspect = async () => {
      if (needsIOSInstall()) {
        try {
          await verifyPushConfiguration("parent");
          if (active) setMode("install-ios");
        } catch (err) {
          if (!isPushConfigurationMissing(err)) {
            logger.warn("parent_push_configuration_check_failed", {
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
          await verifyPushConfiguration("parent");
          if (active) setMode("install-android");
        } catch (err) {
          if (!isPushConfigurationMissing(err)) {
            logger.warn("parent_push_configuration_check_failed", {
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
          fetchNotificationPreferences("parent"),
          syncExistingPushSubscription("parent"),
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
        logger.warn("parent_notification_setup_check_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
    };

    void inspect();
    return () => {
      active = false;
    };
  }, [installAccepted, installationCompleted, installPromptReady, storageKey]);

  const remindLater = () => {
    writeDecision(storageKey, { remindAfter: Date.now() + REMIND_LATER_MS });
    setMode(null);
  };

  const finishInstallGuide = () => {
    writeDecision(storageKey, {
      remindAfter: Date.now() + INSTALL_GUIDE_REMIND_MS,
    });
    setMode(null);
  };

  const installAndroid = async () => {
    setBusy(true);
    setError(null);
    try {
      const outcome = await triggerInstallPrompt();
      if (outcome === "accepted") {
        setInstallAccepted(true);
        setMode("enable");
      }
    } catch (err) {
      logger.warn("parent_app_install_failed", {
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
      const pushRequest = subscribePush("parent");
      const preferenceRequests = types
        .filter((type) => type.available && !type.enabled)
        .map((type) => setNotificationPreference(type.key, true, "parent"));
      await Promise.all([pushRequest, ...preferenceRequests]);
      writeDecision(storageKey, { done: true });
      setMode(null);
    } catch (err) {
      logger.warn("parent_notification_setup_failed", {
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
        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-100">
          <MotoConceptIcon
            concept={mode.startsWith("install-") ? "devices" : "notifications"}
            size={30}
          />
        </div>

        {mode === "install-ios" ? (
          <PushInstallSteps />
        ) : mode === "install-android" ? (
          <div className="space-y-3">
            <p className="text-[17px] leading-7 text-gray-700">
              {t("installAndroidIntro")}
            </p>
            {!installPromptReady && (
              <p className="text-base leading-7 text-gray-600">
                {t("installAndroidManual")}
              </p>
            )}
          </div>
        ) : mode === "denied" ? (
          <p className="text-[17px] leading-7 text-gray-700">{t("denied")}</p>
        ) : (
          <p className="text-[17px] leading-7 text-gray-700">{t("intro")}</p>
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
