"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
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
import { hasSessionAccess } from "~/lib/session-access";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import {
  listTrustedDevices,
  revokeTrustedDevice,
  type LoginScope,
  type TrustedDeviceDTO,
} from "~/lib/mfa-api";

const logger = createLogger({ component: "TrustedDevicesSection" });

function formatDeviceDate(iso: string, locale: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleString(locale, {
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

/** Every text of the card; the German default is the staff portals'. */
export interface TrustedDevicesCopy {
  readonly title: string;
  readonly description: string;
  readonly emptyTitle: string;
  readonly emptyDescription: string;
  readonly added: string;
  readonly validUntil: string;
  readonly lastUsed: string;
  readonly actionsFor: (device: string) => string;
  readonly remove: string;
  readonly removeDescription: string;
  readonly confirmTitle: string;
  readonly confirmText: (device: ReactNode) => ReactNode;
  readonly confirmFirstStep: string;
  readonly confirmFinal: string;
  readonly removing: string;
  readonly removedToast: string;
  readonly loadObject: string;
  readonly reloadObject: string;
  readonly removeObject: string;
  /** BCP 47 tag the dates are written in. */
  readonly dateLocale: string;
}

const GERMAN_TRUSTED_DEVICES_COPY: TrustedDevicesCopy = {
  title: "Meine vertrauten Geräte",
  description:
    "Gemerkte Geräte überspringen den 2FA-Code. Entfernen Sie Geräte, die Sie nicht mehr nutzen.",
  emptyTitle: "Sie haben aktuell keine vertrauten Geräte gespeichert.",
  emptyDescription:
    "Wenn moto beim Anmelden nach einem Code fragt, können Sie Ihr Gerät dort merken.",
  added: "Hinzugefügt",
  validUntil: "Gültig bis",
  lastUsed: "Zuletzt benutzt",
  actionsFor: (device) => `Aktionen für ${device}`,
  remove: "Entfernen",
  removeDescription:
    "moto fragt vorher nach, danach braucht das Gerät wieder den 2FA-Code.",
  confirmTitle: "Gerät entfernen?",
  confirmText: (device) => (
    <>
      Das Gerät {device} wird entfernt. Beim nächsten Anmelden dort wird der
      2FA-Code wieder abgefragt.
    </>
  ),
  confirmFirstStep: "Ja, entfernen",
  confirmFinal: "Endgültig entfernen",
  removing: "Wird entfernt…",
  removedToast: "Das Gerät ist entfernt.",
  loadObject: "die Liste der vertrauten Geräte",
  reloadObject: "die Anzeige der vertrauten Geräte",
  removeObject: "das Entfernen des Geräts",
  dateLocale: "de-DE",
};

interface TrustedDevicesSectionProps {
  readonly scope?: LoginScope;
  readonly copy?: TrustedDevicesCopy;
  readonly headingLevel?: 2 | 3;
}

export function TrustedDevicesSection({
  scope = "tenant",
  copy = GERMAN_TRUSTED_DEVICES_COPY,
  headingLevel = 3,
}: TrustedDevicesSectionProps = {}) {
  const { data: session, status: sessionStatus } = useSession();
  const hasAccess = hasSessionAccess(session);
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

  // The routes read the session cookie (#5005); a stripped session sends no
  // request. Rejects on failure: after a write the caller shows it as a
  // reload of a saved change and keeps the shown list (#6652).
  const readDevices = useCallback(async () => {
    if (!hasAccess) return;
    const list = await listTrustedDevices(scope);
    setDevices(list);
  }, [hasAccess, scope]);
  // Never rejects: a failed load shows its error in place of the list.
  const load = useCallback(async () => {
    if (!hasAccess) return;
    setLoading(true);
    setLoadFailed(false);
    clear();
    try {
      await readDevices();
    } catch (err) {
      logger.error("list_trusted_devices_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: copy.loadObject,
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [hasAccess, clear, readDevices, showLoadError, copy.loadObject]);
  const { error: reloadError, refresh: refreshDevices } = useRefreshAfterWrite(
    copy.reloadObject,
    readDevices,
  );

  useEffect(() => {
    if (sessionStatus === "authenticated") {
      // Fire-and-forget pattern is the project convention for async work in
      // useEffect; errors are surfaced via the function's own try/catch.
      void load();
    }
  }, [sessionStatus, load]);

  const handleRevoke = useCallback(
    async (deviceId: number) => {
      if (!hasAccess) return;
      setRevokingId(deviceId);
      revokeErrors.clear();
      try {
        await revokeTrustedDevice(scope, deviceId);
      } catch (err) {
        logger.error("revoke_trusted_device_failed", {
          device_id: deviceId,
          error: err instanceof Error ? err.message : String(err),
        });
        void revokeErrors.show(err, {
          object: copy.removeObject,
          retry: () => latestRevokeRef.current(deviceId),
        });
        setRevokingId(null);
        return;
      }
      setRevokingId(null);
      setRevokeTarget(null);
      toast.success(copy.removedToast);
      await refreshDevices();
    },
    [hasAccess, scope, toast, revokeErrors, refreshDevices, copy],
  );

  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
    latestRevokeRef.current = (deviceId) => void handleRevoke(deviceId);
  });

  return (
    <SectionCard
      headingLevel={headingLevel}
      title={copy.title}
      description={copy.description}
    >
      {/* Bis der Text eines Ladefehlers da ist, bleibt das Skelett stehen. */}
      {(loading || (loadFailed && !loadError)) && (
        <div className="space-y-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      )}
      {!loading && loadFailed && loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : null}
      <LoadErrorAlert error={reloadError} className="mb-4" />
      {!loading && !loadFailed && (!devices || devices.length === 0) && (
        <EmptyState
          variant="compact"
          title={copy.emptyTitle}
          description={copy.emptyDescription}
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
                  {copy.added}:{" "}
                  {formatDeviceDate(d.created_at, copy.dateLocale)} ·{" "}
                  {copy.validUntil}:{" "}
                  {formatDeviceDate(d.expires_at, copy.dateLocale)}
                  {d.last_used_at && (
                    <>
                      {" "}
                      · {copy.lastUsed}:{" "}
                      {formatDeviceDate(d.last_used_at, copy.dateLocale)}
                    </>
                  )}
                  {d.ip_address && <> · IP: {d.ip_address}</>}
                </div>
              </div>
              {/* Zeilenaktionen nur im Kebab der Zeile (BAUARTEN-SPEC
                  Bauart 1 Regel 4); Entfernen fragt weiter im
                  ConfirmDeleteModal nach. */}
              <div className="flex shrink-0 justify-end">
                <OverflowMenu
                  ariaLabel={copy.actionsFor(
                    formatDeviceLabelFromUserAgent(d.user_agent),
                  )}
                  items={[
                    {
                      label: copy.remove,
                      description: copy.removeDescription,
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
        title={copy.confirmTitle}
        description={
          revokeTarget ? (
            <p ref={revokeRef}>
              {copy.confirmText(
                <span className="font-medium text-gray-900">
                  {formatDeviceLabelFromUserAgent(revokeTarget.user_agent)}
                </span>,
              )}
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: copy.confirmFirstStep }}
        confirmLabel={copy.confirmFinal}
        loadingLabel={copy.removing}
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
