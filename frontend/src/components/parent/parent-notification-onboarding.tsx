"use client";

import { useEffect, useMemo, useState } from "react";
import { Bell, Loader2, Share } from "lucide-react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import {
  fetchNotificationPreferences,
  setNotificationPreference,
  type NotificationPreferenceType,
} from "~/lib/notification-preferences-api";
import { createLogger } from "~/lib/logger";
import {
  isPushConfigurationMissing,
  isPushSupported,
  needsIOSInstall,
  subscribePush,
  syncExistingPushSubscription,
} from "~/lib/push-api";

const logger = createLogger({ component: "ParentNotificationOnboarding" });
const REMIND_LATER_MS = 7 * 24 * 60 * 60 * 1000;

type SetupMode = "enable" | "install" | "denied";

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
        if (active) setMode("install");
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
  }, [storageKey]);

  const remindLater = () => {
    writeDecision(storageKey, { remindAfter: Date.now() + REMIND_LATER_MS });
    setMode(null);
  };

  const finishInstallGuide = () => {
    writeDecision(storageKey, { remindAfter: Date.now() + REMIND_LATER_MS });
    setMode(null);
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
      title={mode === "install" ? t("installTitle") : t("title")}
      closeLabel={t("close")}
      backdropLabel={t("close")}
      isDismissDisabled={busy}
    >
      <div className="space-y-5">
        <div className="bg-moto-blue/10 text-moto-blue-strong flex h-12 w-12 items-center justify-center rounded-2xl">
          {mode === "install" ? (
            <Share className="h-6 w-6" aria-hidden="true" />
          ) : (
            <Bell className="h-6 w-6" aria-hidden="true" />
          )}
        </div>

        {mode === "install" ? (
          <div className="space-y-4">
            <p className="text-sm leading-6 text-gray-700">
              {t("installIntro")}
            </p>
            <ol className="space-y-3">
              {[1, 2, 3].map((step) => (
                <li key={step} className="flex gap-3 text-sm text-gray-700">
                  <span className="bg-moto-blue/10 text-moto-blue-strong flex h-6 w-6 shrink-0 items-center justify-center rounded-full font-semibold">
                    {step}
                  </span>
                  <span>{t(`installStep${step}`)}</span>
                </li>
              ))}
            </ol>
          </div>
        ) : mode === "denied" ? (
          <p className="text-sm leading-6 text-gray-700">{t("denied")}</p>
        ) : (
          <p className="text-sm leading-6 text-gray-700">{t("intro")}</p>
        )}

        {error && <Alert type="error" message={error} />}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          {mode === "enable" && (
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={busy}
              onClick={remindLater}
            >
              {t("later")}
            </Button>
          )}
          {mode === "enable" ? (
            <Button
              type="button"
              size="md"
              disabled={busy}
              onClick={() => void enable()}
            >
              {busy && (
                <Loader2
                  className="mr-2 h-4 w-4 animate-spin"
                  aria-hidden="true"
                />
              )}
              {t("enable")}
            </Button>
          ) : (
            <Button type="button" size="md" onClick={finishInstallGuide}>
              {t("understood")}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  );
}
