"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { useTranslations } from "next-intl";
import { PushInstallSteps } from "~/components/settings/push-install-steps";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { Skeleton } from "~/components/ui/skeleton";
import { createLogger } from "~/lib/logger";
import { sendTestNotification } from "~/lib/notification-api";
import {
  isPushConfigurationMissing,
  isPushSupported,
  isStandaloneApp,
  needsIOSInstall,
  subscribePush,
  syncExistingPushSubscription,
  unsubscribePush,
  verifyPushConfiguration,
  type PushPortal,
} from "~/lib/push-api";
import {
  canPromptInstall,
  isAndroidDevice,
  isInstallationCompleted,
  subscribeInstallPrompt,
  triggerInstallPrompt,
} from "~/lib/pwa-install-prompt";

const logger = createLogger({ component: "PushNotificationSection" });

interface PushNotificationSectionProps {
  readonly portal?: PushPortal;
}

type PushState =
  | "loading"
  | "unsupported"
  | "needs-install-ios"
  | "needs-install-android"
  | "disabled"
  | "denied"
  | "subscribed"
  | "unsubscribed";

/**
 * Per-device Web Push opt-in (#2003). Permission is only ever requested from
 * the button click (iOS requirement); on iOS Safari outside an installed
 * home-screen app it explains the install prerequisite instead.
 */
export function PushNotificationSection({
  portal = "tenant",
}: PushNotificationSectionProps) {
  const t = useTranslations("pushNotifications");
  const setupT = useTranslations("parentNotificationSetup");
  const [state, setState] = useState<PushState>("loading");
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
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

  const refresh = useCallback(async () => {
    if (needsIOSInstall()) {
      try {
        await verifyPushConfiguration(portal);
        setState("needs-install-ios");
      } catch (err) {
        if (!isPushConfigurationMissing(err)) {
          logger.error("push_configuration_check_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        setState("disabled");
      }
      return;
    }
    if (
      portal === "parent" &&
      isAndroidDevice(window.navigator) &&
      !isStandaloneApp() &&
      !installationCompleted &&
      !installAccepted
    ) {
      try {
        await verifyPushConfiguration(portal);
        setState("needs-install-android");
      } catch (err) {
        if (!isPushConfigurationMissing(err)) {
          logger.error("push_configuration_check_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        setState("disabled");
      }
      return;
    }
    if (!isPushSupported()) {
      setState("unsupported");
      return;
    }
    if (Notification.permission === "denied") {
      setState("denied");
      return;
    }
    try {
      const subscription = await syncExistingPushSubscription(portal);
      setState(subscription ? "subscribed" : "unsubscribed");
    } catch (err) {
      if (isPushConfigurationMissing(err)) {
        setState("disabled");
        return;
      }
      logger.error("push_state_check_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setState("unsubscribed");
    }
  }, [installAccepted, installationCompleted, portal]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const enable = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await subscribePush(portal);
      setMessage(t("enabledMessage"));
      await refresh();
    } catch (err) {
      logger.error("push_subscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(t("enableError"));
      await refresh();
    } finally {
      setBusy(false);
    }
  };

  const installAndroid = async () => {
    setBusy(true);
    setError(null);
    try {
      const outcome = await triggerInstallPrompt();
      if (outcome === "accepted") {
        setInstallAccepted(true);
      }
    } catch (err) {
      logger.warn("parent_app_install_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(setupT("installError"));
    } finally {
      setBusy(false);
    }
  };

  const disable = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await unsubscribePush(portal);
      setMessage(t("disabledMessage"));
      await refresh();
    } catch (err) {
      logger.error("push_unsubscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(t("disableError"));
    } finally {
      setBusy(false);
    }
  };

  const sendTest = async () => {
    setBusy(true);
    setTesting(true);
    setError(null);
    setMessage(null);
    try {
      await sendTestNotification();
      setMessage(t("testSent"));
    } catch (err) {
      logger.error("test_notification_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : t("testError"));
    } finally {
      setTesting(false);
      setBusy(false);
    }
  };

  if (state === "loading") return <PushNotificationSkeleton />;
  if (state === "disabled") return null;

  // Der Zustandstext gehört als Erklärung in den Kartenkopf, nicht als
  // freier Absatz darunter. Hinweise, die einen Schritt außerhalb der App
  // verlangen (Installation, blockierte Browser-Berechtigung), bleiben ein
  // Alert mit der zugehörigen Aktion.
  const headerSubtitle =
    state === "unsupported" ? (
      <p className="max-w-2xl text-sm leading-6 text-pretty">
        {t("unsupportedBody")}
      </p>
    ) : state === "unsubscribed" ? (
      <>
        <p className="max-w-2xl text-sm leading-6 text-pretty">
          {t("offBody")}
        </p>
        <p className="mt-1 max-w-2xl text-sm leading-6 text-pretty">
          {t("permissionHint")}
        </p>
      </>
    ) : state === "subscribed" ? (
      <p className="max-w-2xl text-sm leading-6 text-pretty">{t("onBody")}</p>
    ) : undefined;

  const primaryAction =
    state === "unsubscribed" ? (
      <Button
        type="button"
        size="md"
        aria-label={t("enable")}
        isLoading={busy}
        loadingText={t("enabling")}
        onClick={() => void enable()}
      >
        {t("enableShort")}
      </Button>
    ) : state === "subscribed" ? (
      <Button
        type="button"
        variant="surface"
        size="md"
        aria-label={t("disable")}
        disabled={busy}
        onClick={() => void disable()}
      >
        {t("disableShort")}
      </Button>
    ) : state === "needs-install-android" && installPromptReady ? (
      <Button
        type="button"
        size="md"
        isLoading={busy}
        loadingText={setupT("installing")}
        onClick={() => void installAndroid()}
      >
        {setupT("installApp")}
      </Button>
    ) : null;

  const testAction =
    state === "subscribed" && portal === "tenant" ? (
      <Button
        type="button"
        variant="surface"
        size="md"
        isLoading={testing}
        loadingText={t("testing")}
        disabled={busy}
        onClick={() => void sendTest()}
      >
        {t("sendTest")}
      </Button>
    ) : null;

  const headerActions =
    primaryAction != null || testAction != null ? (
      <>
        {testAction}
        {primaryAction}
      </>
    ) : null;

  // Ohne Karteninhalt darf der Kopf keinen Abstand nach unten aufspannen.
  const hasBody =
    error != null ||
    message != null ||
    state === "needs-install-ios" ||
    state === "needs-install-android" ||
    state === "denied";

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <ConceptSectionHeader
        className={hasBody ? "mb-4" : undefined}
        // Geschwisterkarten auf /profile und /parents/settings sind h3.
        level={3}
        title={t("title")}
        concept="notifications"
        subtitle={headerSubtitle}
        actions={headerActions}
        actionsClassName="ms-auto flex flex-wrap items-center gap-2"
      />

      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}
      {message && (
        <div className="mb-3">
          <Alert type="success" message={message} />
        </div>
      )}

      {state === "needs-install-ios" && <PushInstallSteps compact />}

      {state === "needs-install-android" && (
        <Alert
          type="info"
          title={setupT("installAndroidTitle")}
          message={
            installPromptReady
              ? setupT("installAndroidIntro")
              : setupT("installAndroidManual")
          }
        />
      )}

      {state === "denied" && (
        <Alert
          type="warning"
          title={t("blockedTitle")}
          message={t("blockedBody")}
          action={
            <Button
              type="button"
              variant="surface"
              size="md"
              disabled={busy}
              onClick={() => void refresh()}
            >
              {t("checkAgain")}
            </Button>
          }
        />
      )}
    </div>
  );
}

function PushNotificationSkeleton() {
  return (
    <div
      data-testid="push-notification-skeleton"
      className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6"
      aria-hidden="true"
    >
      <div className="flex items-center gap-3">
        <Skeleton className="size-10 shrink-0 rounded-xl" />
        <Skeleton className="h-5 w-44" />
        <Skeleton className="ms-auto h-10 w-28 rounded-lg" />
      </div>
      <Skeleton className="mt-4 h-4 w-full max-w-2xl" />
      <Skeleton className="mt-2 h-4 w-3/4 max-w-xl" />
    </div>
  );
}
