"use client";

import { useCallback, useEffect, useState } from "react";
import { BellRing } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
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

  const enable = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await subscribePush(portal);
      setMessage("Benachrichtigungen sind auf diesem Gerät aktiviert.");
      await refresh();
    } catch (err) {
      logger.error("push_subscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Benachrichtigungen konnten nicht aktiviert werden.",
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
      setMessage("Benachrichtigungen sind auf diesem Gerät deaktiviert.");
      await refresh();
    } catch (err) {
      logger.error("push_unsubscribe_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Benachrichtigungen konnten nicht deaktiviert werden.",
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
          : "Testbenachrichtigung konnte nicht gesendet werden. Prüfen Sie die Verbindung und versuchen Sie es erneut.",
      );
    } finally {
      setTesting(false);
      setBusy(false);
    }
  };

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <BellRing className="h-5 w-5 text-gray-800" aria-hidden="true" />
          <h3 className="text-base font-semibold text-gray-900">
            Push-Benachrichtigungen
          </h3>
        </div>
        {state === "unsubscribed" && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => void enable()}
          >
            Aktivieren
          </Button>
        )}
        {state === "subscribed" && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => void disable()}
          >
            Deaktivieren
          </Button>
        )}
      </div>

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
          Auf iPhone und iPad funktionieren Benachrichtigungen nur, wenn die App
          zuerst zum Home-Bildschirm hinzugefügt wurde: In Safari das
          Teilen-Symbol antippen und {"„"}Zum Home-Bildschirm{"“"} wählen.
          Danach die App vom Home-Bildschirm aus öffnen und Benachrichtigungen
          hier aktivieren.
        </p>
      )}

      {state === "unsupported" && (
        <p className="text-sm text-gray-600">
          Dieser Browser unterstützt keine Push-Benachrichtigungen.
        </p>
      )}

      {state === "disabled" && (
        <p className="text-sm text-gray-600">
          Push-Benachrichtigungen sind auf diesem Server nicht eingerichtet.
        </p>
      )}

      {state === "denied" && (
        <p className="text-sm text-gray-600">
          Benachrichtigungen wurden für diese Seite blockiert. Bitte in den
          Browser-Einstellungen wieder erlauben, um sie zu aktivieren.
        </p>
      )}

      {state === "unsubscribed" && (
        <p className="text-sm text-gray-600">
          Erhalten Sie Erinnerungen als Systembenachrichtigung auf diesem Gerät,
          auch wenn die App geschlossen ist.
        </p>
      )}

      {state === "subscribed" && (
        <div>
          <p className="text-sm text-gray-600">
            Benachrichtigungen sind auf diesem Gerät aktiv. Erinnerungen kommen
            auch bei geschlossener App an.
          </p>
          {portal === "tenant" && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="mt-4"
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
