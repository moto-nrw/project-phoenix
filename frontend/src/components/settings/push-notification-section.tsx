"use client";

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { createLogger } from "~/lib/logger";
import { sendTestNotification } from "~/lib/notification-api";
import {
  isPushConfigurationMissing,
  isPushSupported,
  needsIOSInstall,
  subscribePush,
  syncExistingPushSubscription,
  unsubscribePush,
  type PushPortal,
} from "~/lib/push-api";

const logger = createLogger({ component: "PushNotificationSection" });

interface PushNotificationSectionProps {
  readonly portal?: PushPortal;
}

type PushState =
  | "loading"
  | "unsupported"
  | "needs-install"
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
  const [state, setState] = useState<PushState>("loading");
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    if (needsIOSInstall()) {
      setState("needs-install");
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
  }, [portal]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const headerAction =
    state === "unsubscribed" ? (
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={busy}
        onClick={() => void enable()}
      >
        Einschalten
      </Button>
    ) : state === "subscribed" ? (
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={busy}
        onClick={() => void disable()}
      >
        Ausschalten
      </Button>
    ) : null;

  const enable = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await subscribePush(portal);
      setMessage("Benachrichtigungen sind auf diesem Gerät eingeschaltet.");
      await refresh();
    } catch (err) {
      logger.error("push_subscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
      await refresh();
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
      setMessage("Benachrichtigungen sind auf diesem Gerät ausgeschaltet.");
      await refresh();
    } catch (err) {
      logger.error("push_unsubscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
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
      setMessage("Testbenachrichtigung wurde gesendet.");
    } catch (err) {
      logger.error("test_notification_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Das Senden hat nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setTesting(false);
      setBusy(false);
    }
  };

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <ConceptSectionHeader
        className="mb-4"
        // Geschwisterkarten auf /profile und /parents/settings sind h3.
        level={3}
        title="Push-Benachrichtigungen"
        concept="notifications"
        actions={headerAction}
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

      {state === "loading" && <p className="text-sm text-gray-500">Laden...</p>}

      {state === "needs-install" && (
        <p className="text-sm text-gray-600">
          Auf iPhone und iPad geht das nur, wenn moto auf dem Home-Bildschirm
          liegt. So geht es: In Safari unten auf das Teilen-Symbol tippen, dann
          {" „"}Zum Home-Bildschirm{"“"} wählen. Danach moto von dort öffnen und
          die Benachrichtigungen hier einschalten.
        </p>
      )}

      {state === "unsupported" && (
        <p className="text-sm text-gray-600">
          Dieser Browser kann leider keine Benachrichtigungen anzeigen.
        </p>
      )}

      {state === "disabled" && (
        <p className="text-sm text-gray-600">
          Benachrichtigungen sind hier zurzeit nicht verfügbar.
        </p>
      )}

      {state === "denied" && (
        <p className="text-sm text-gray-600">
          Benachrichtigungen sind für moto blockiert. Sie können sie in den
          Einstellungen Ihres Browsers wieder erlauben.
        </p>
      )}

      {state === "unsubscribed" && (
        <p className="text-sm text-gray-600">
          Bekommen Sie Erinnerungen direkt auf dieses Gerät, auch wenn moto
          gerade geschlossen ist.
        </p>
      )}

      {state === "subscribed" && (
        <div>
          <p className="text-sm text-gray-600">
            Benachrichtigungen sind auf diesem Gerät eingeschaltet. Erinnerungen
            kommen auch an, wenn moto geschlossen ist.
          </p>
          {portal === "tenant" && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="-ms-2.5 mt-2"
              isLoading={testing}
              loadingText="Wird gesendet…"
              disabled={busy}
              onClick={() => void sendTest()}
            >
              Testbenachrichtigung senden
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
