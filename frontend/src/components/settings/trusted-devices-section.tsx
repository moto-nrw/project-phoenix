"use client";

import { useCallback, useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { SectionCard } from "~/components/ui/section-card";
import { EmptyState } from "~/components/ui/empty-state";
import { useToast } from "~/contexts/ToastContext";
import { formatDeviceLabelFromUserAgent } from "~/lib/device-label";
import { createLogger } from "~/lib/logger";
import {
  listTrustedDevices,
  revokeTrustedDevice,
  type LoginScope,
  type TrustedDeviceDTO,
} from "~/lib/mfa-api";

const logger = createLogger({ component: "TrustedDevicesSection" });

function formatGermanDate(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

interface TrustedDevicesSectionProps {
  readonly scope?: LoginScope;
}

export function TrustedDevicesSection({
  scope = "tenant",
}: TrustedDevicesSectionProps = {}) {
  const { data: session, status: sessionStatus } = useSession();
  const bearerToken = session?.user?.token;
  const toast = useToast();
  const [devices, setDevices] = useState<TrustedDeviceDTO[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (!bearerToken) return;
    setLoading(true);
    setError(null);
    try {
      const list = await listTrustedDevices(scope, bearerToken);
      setDevices(list);
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : "Geräte konnten nicht geladen werden.";
      logger.error("list_trusted_devices_failed", { error: msg });
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [bearerToken, scope]);

  useEffect(() => {
    if (sessionStatus === "authenticated") {
      // Fire-and-forget pattern is the project convention for async work in
      // useEffect; errors are surfaced via the function's own try/catch.
      void load(); // NOSONAR typescript:S3735 fire-and-forget pattern matches project convention (10+ existing sites)
    }
  }, [sessionStatus, load]);

  const handleRevoke = useCallback(
    async (deviceId: number) => {
      if (!bearerToken) return;
      setRevokingId(deviceId);
      try {
        await revokeTrustedDevice(scope, bearerToken, deviceId);
        toast.success("Gerät erfolgreich entfernt.");
        await load();
      } catch (err) {
        const msg =
          err instanceof Error
            ? err.message
            : "Gerät konnte nicht entfernt werden.";
        logger.error("revoke_trusted_device_failed", {
          device_id: deviceId,
          error: msg,
        });
        toast.error(msg);
      } finally {
        setRevokingId(null);
      }
    },
    [bearerToken, scope, toast, load],
  );

  return (
    <SectionCard
      headingLevel={3}
      title="Meine vertrauten Geräte"
      description="Gemerkte Geräte überspringen den 2FA-Code. Entfernen Sie Geräte, die Sie nicht mehr nutzen."
    >
      {error && (
        <div className="mb-4">
          <Alert type="error" message={error} />
        </div>
      )}

      {loading && (
        <div className="space-y-2">
          <Skeleton className="h-16 w-full rounded" />
          <Skeleton className="h-16 w-full rounded" />
        </div>
      )}
      {!loading && (!devices || devices.length === 0) && (
        <EmptyState
          variant="compact"
          title="Sie haben aktuell keine vertrauten Geräte gespeichert."
          description="Beim nächsten Login können Sie ein Gerät merken lassen, um den 2FA-Code dort zu überspringen."
        />
      )}
      {!loading && devices && devices.length > 0 && (
        <ul className="divide-y divide-gray-100">
          {devices.map((d) => (
            <li
              key={d.id}
              className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-gray-900">
                  {formatDeviceLabelFromUserAgent(d.user_agent)}
                </div>
                <div className="mt-1 text-xs text-gray-500">
                  Hinzugefügt: {formatGermanDate(d.created_at)} · Gültig bis:{" "}
                  {formatGermanDate(d.expires_at)}
                  {d.last_used_at && (
                    <> · Zuletzt benutzt: {formatGermanDate(d.last_used_at)}</>
                  )}
                  {d.ip_address && <> · IP: {d.ip_address}</>}
                </div>
              </div>
              <Button
                type="button"
                variant="outline_danger"
                size="sm"
                disabled={revokingId === d.id}
                // void discards the promise from the async handler so the
                // onClick signature returns void as expected.
                onClick={() => void handleRevoke(d.id)} // NOSONAR typescript:S3735 fire-and-forget pattern matches project convention (10+ existing sites)
              >
                {revokingId === d.id ? "Entferne..." : "Entfernen"}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
