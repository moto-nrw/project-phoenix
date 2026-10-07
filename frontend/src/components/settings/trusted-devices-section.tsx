"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { Trash2 } from "lucide-react";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { Skeleton } from "~/components/ui/skeleton";
import { SectionCard } from "~/components/ui/section-card";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
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
  const [loadFailed, setLoadFailed] = useState(false);
  const [revokingId, setRevokingId] = useState<number | null>(null);
  // Entfernen läuft erst nach der Rückfrage (Bauart 2 Regel 6, #3109); ein
  // Fehler bleibt im Dialog stehen. Ladefehler stehen vor Ort mit
  // Wiederholen, nie als leere Liste (#2517).
  const [revokeTarget, setRevokeTarget] = useState<TrustedDeviceDTO | null>(
    null,
  );
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const revokeRef = useRef<HTMLParagraphElement>(null);
  const revokeErrors = useApiFormError(revokeRef);
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestRevokeRef = useRef<(deviceId: number) => void>(() => undefined);

  const load = useCallback(async () => {
    if (!bearerToken) return;
    setLoading(true);
    setLoadFailed(false);
    clear();
    try {
      const list = await listTrustedDevices(scope, bearerToken);
      setDevices(list);
    } catch (err) {
      logger.error("list_trusted_devices_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: "die Liste der vertrauten Geräte",
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [bearerToken, clear, scope, showLoadError]);

  useEffect(() => {
    if (sessionStatus === "authenticated") {
      // Fire-and-forget pattern is the project convention for async work in
      // useEffect; errors are surfaced via the function's own try/catch.
      void load();
    }
  }, [sessionStatus, load]);

  const handleRevoke = useCallback(
    async (deviceId: number) => {
      if (!bearerToken) return;
      setRevokingId(deviceId);
      revokeErrors.clear();
      try {
        await revokeTrustedDevice(scope, bearerToken, deviceId);
        setRevokeTarget(null);
        toast.success("Das Gerät ist entfernt.");
        await load();
      } catch (err) {
        logger.error("revoke_trusted_device_failed", {
          device_id: deviceId,
          error: err instanceof Error ? err.message : String(err),
        });
        void revokeErrors.show(err, {
          object: "das Entfernen des Geräts",
          retry: () => latestRevokeRef.current(deviceId),
        });
      } finally {
        setRevokingId(null);
      }
    },
    [bearerToken, scope, toast, load, revokeErrors],
  );

  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
    latestRevokeRef.current = (deviceId) => void handleRevoke(deviceId);
  });

  return (
    <SectionCard
      headingLevel={3}
      title="Meine vertrauten Geräte"
      description="Gemerkte Geräte überspringen den 2FA-Code. Entfernen Sie Geräte, die Sie nicht mehr nutzen."
    >
      {/* Bis der Text eines Ladefehlers da ist, bleibt das Skelett stehen. */}
      {(loading || (loadFailed && !loadError)) && (
        <div className="space-y-2">
          <Skeleton className="h-16 w-full rounded" />
          <Skeleton className="h-16 w-full rounded" />
        </div>
      )}
      {!loading && loadFailed && loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : null}
      {!loading && !loadFailed && (!devices || devices.length === 0) && (
        <EmptyState
          variant="compact"
          title="Sie haben aktuell keine vertrauten Geräte gespeichert."
          description="Beim nächsten Login können Sie ein Gerät merken lassen, um den 2FA-Code dort zu überspringen."
        />
      )}
      {!loading && !loadFailed && devices && devices.length > 0 && (
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
              {/* Zeilenaktionen nur im Kebab der Zeile (BAUARTEN-SPEC
                  Bauart 1 Regel 4); Entfernen fragt weiter im
                  ConfirmDeleteModal nach. */}
              <div className="flex shrink-0 justify-end">
                <OverflowMenu
                  ariaLabel={`Aktionen für ${formatDeviceLabelFromUserAgent(d.user_agent)}`}
                  items={[
                    {
                      label: "Entfernen",
                      icon: <Trash2 className="h-4 w-4" aria-hidden />,
                      destructive: true,
                      disabled: revokingId === d.id,
                      onClick: () => {
                        revokeErrors.clear();
                        setRevokeTarget(d);
                      },
                    },
                  ]}
                />
              </div>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDeleteModal
        isOpen={revokeTarget !== null}
        title="Gerät entfernen?"
        description={
          revokeTarget ? (
            <p ref={revokeRef}>
              Das Gerät{" "}
              <span className="font-medium text-gray-900">
                {formatDeviceLabelFromUserAgent(revokeTarget.user_agent)}
              </span>{" "}
              wird entfernt. Beim nächsten Anmelden dort wird der 2FA-Code
              wieder abgefragt.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Ja, entfernen" }}
        confirmLabel="Endgültig entfernen"
        loadingLabel="Wird entfernt…"
        loading={revokingId !== null}
        error={revokeErrors.error}
        onConfirm={() => {
          if (revokeTarget) void handleRevoke(revokeTarget.id);
        }}
        onClose={() => {
          setRevokeTarget(null);
          revokeErrors.clear();
        }}
      />
    </SectionCard>
  );
}
