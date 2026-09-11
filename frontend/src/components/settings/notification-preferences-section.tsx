"use client";

import { useCallback, useEffect, useState } from "react";
import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { createLogger } from "~/lib/logger";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  disableAllNotificationPreferences,
  fetchNotificationPreferences,
  setNotificationPreference,
  type NotificationPreferenceType,
  type PreferencePortal,
} from "~/lib/notification-preferences-api";

const logger = createLogger({ component: "NotificationPreferencesSection" });

/**
 * German headings for the registry groups (services/notifications/types.go).
 *
 * Only the wording lives here — the order and the set of groups come from the
 * backend response, which is already sorted. An unknown key falls back to the
 * raw group name, so a new backend group renders with an ugly heading rather
 * than disappearing: a missing group would take its types with it, and since
 * "no row means off", nobody could ever switch them on.
 */
const GROUP_LABELS: Record<string, string> = {
  abholung: "Abholungen",
  aktivitaeten: "Aktivitäten",
  kinder: "Kinder",
  mitteilungen: "Mitteilungen",
  termine: "Termine",
};

const PARENT_GROUPS = new Set(["kinder", "mitteilungen", "termine"]);
const PARENT_TYPES = new Set([
  "parent_announcement",
  "parent_message",
  "parent_appointment",
  "parent_appointment_reminder",
  "parent_request_decided",
  "parent_care_cancelled",
]);

interface NotificationPreferencesSectionProps {
  readonly portal?: PreferencePortal;
}

/**
 * Per-account notification consent. Sits above the per-device push card: this
 * one answers "what do I want to hear about", the other "on which device".
 *
 * Nothing is delivered without an explicit switch here, so every type starts
 * off.
 */
export function NotificationPreferencesSection({
  portal = "tenant",
}: NotificationPreferencesSectionProps) {
  return portal === "parent" ? (
    <ParentNotificationPreferencesSection />
  ) : (
    // Staff-catalogue portals (OGS and, since #2208, the school portal) share
    // the German copy; only the proxy base path differs.
    <NotificationPreferencesSectionContent portal={portal} />
  );
}

function ParentNotificationPreferencesSection() {
  const t = useTranslations("notificationPreferences");
  return (
    <NotificationPreferencesSectionContent
      portal="parent"
      parentText={(key) => t(key as never)}
    />
  );
}

function NotificationPreferencesSectionContent({
  portal,
  parentText,
}: Readonly<{
  portal: PreferencePortal;
  parentText?: (key: string) => string;
}>) {
  const isParentPortal = portal === "parent";
  const t = useCallback(
    (key: string) => parentText?.(key) ?? key,
    [parentText],
  );
  const [types, setTypes] = useState<NotificationPreferenceType[]>([]);
  const [tenantEnabled, setTenantEnabled] = useState(true);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [bulkEnabling, setBulkEnabling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await fetchNotificationPreferences(portal);
      setTypes(data.types);
      setTenantEnabled(data.tenant_enabled);
      setError(null);
    } catch (err) {
      logger.error("notification_preferences_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        isParentPortal
          ? t("loadError")
          : "Die Einstellungen konnten nicht geladen werden.",
      );
    } finally {
      setLoading(false);
    }
  }, [isParentPortal, portal, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const toggle = async (type: NotificationPreferenceType, enabled: boolean) => {
    // Optimistic: the switch answers immediately and rolls back on failure.
    setBusy(true);
    setTypes((current) =>
      current.map((t) => (t.key === type.key ? { ...t, enabled } : t)),
    );
    setError(null);
    try {
      await setNotificationPreference(type.key, enabled, portal);
    } catch (err) {
      logger.error("notification_preference_save_failed", {
        type: type.key,
        error: err instanceof Error ? err.message : String(err),
      });
      setTypes((current) =>
        current.map((t) =>
          t.key === type.key ? { ...t, enabled: !enabled } : t,
        ),
      );
      setError(
        isParentPortal
          ? t("saveError")
          : "Die Einstellung konnte nicht gespeichert werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const disableAll = async () => {
    setBusy(true);
    setError(null);
    try {
      await disableAllNotificationPreferences(portal);
      setTypes((current) => current.map((t) => ({ ...t, enabled: false })));
    } catch (err) {
      logger.error("notification_preferences_disable_all_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        isParentPortal
          ? t("bulkError")
          : "Die Einstellungen konnten nicht geändert werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const enableAll = async () => {
    const disabledTypes = types.filter((type) => !type.enabled);
    if (disabledTypes.length === 0) return;

    setBusy(true);
    setBulkEnabling(true);
    setError(null);
    try {
      const results = await Promise.allSettled(
        disabledTypes.map((type) =>
          setNotificationPreference(type.key, true, portal),
        ),
      );
      const failedCount = results.filter(
        (result) => result.status === "rejected",
      ).length;

      if (failedCount > 0) {
        logger.error("notification_preferences_enable_all_failed", {
          failedCount,
        });
        await load();
        setError(
          isParentPortal
            ? t("bulkError")
            : "Die Einstellungen konnten nicht geändert werden.",
        );
        return;
      }

      setTypes((current) =>
        current.map((type) => ({ ...type, enabled: true })),
      );
    } finally {
      setBulkEnabling(false);
      setBusy(false);
    }
  };

  const anyEnabled = types.some((t) => t.enabled);
  const allEnabled = types.length > 0 && types.every((type) => type.enabled);
  // Groups come from the response, in the order the backend sent them — the
  // catalogue is already sorted by group and then by rank. Deriving them here
  // rather than from a local list means a new backend group shows up instead of
  // silently dropping its types out of the card.
  const groups = types.reduce<
    { group: string; label: string; items: typeof types }[]
  >((acc, type) => {
    const existing = acc.find((g) => g.group === type.group);
    if (existing) {
      existing.items.push(type);
      return acc;
    }
    acc.push({
      group: type.group,
      label:
        isParentPortal && PARENT_GROUPS.has(type.group)
          ? t(`groups.${type.group}`)
          : (GROUP_LABELS[type.group] ?? type.group),
      items: [type],
    });
    return acc;
  }, []);

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <ConceptSectionHeader
        className="mb-4"
        // Die Kanalgruppen darunter sind h4, und auf /parents/settings ist
        // dieser Abschnitt die oberste Ueberschrift der Seite.
        level={3}
        title={isParentPortal ? t("title") : "Benachrichtigungen"}
        concept="notifications"
        subtitle={
          isParentPortal
            ? t("description")
            : "Wählen Sie, worüber moto Sie informieren soll."
        }
        actionsClassName={isParentPortal ? "ms-auto" : undefined}
        actions={
          isParentPortal && loading ? (
            <Skeleton className="h-9 w-28 rounded-lg" />
          ) : isParentPortal && types.length > 0 ? (
            allEnabled ? (
              <span
                role="status"
                className="flex h-9 items-center justify-center gap-2 text-sm font-medium text-gray-600"
              >
                <Check
                  className="h-4 w-4"
                  style={{ color: LOCATION_COLORS.GROUP_ROOM }}
                  aria-hidden="true"
                />
                {t("allEnabled")}
              </span>
            ) : (
              <Button
                type="button"
                size="md"
                disabled={busy}
                isLoading={bulkEnabling}
                loadingText={t("enablingAll")}
                onClick={() => void enableAll()}
              >
                {t("enableAll")}
              </Button>
            )
          ) : anyEnabled ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void disableAll()}
            >
              {isParentPortal ? t("disableAll") : "Alle deaktivieren"}
            </Button>
          ) : null
        }
      />

      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}

      {!loading && !tenantEnabled && (
        <div className="mb-3">
          <Alert
            type="info"
            message={
              isParentPortal
                ? t("tenantDisabled")
                : "Ihre Schule hat Benachrichtigungen derzeit deaktiviert. Ihre Auswahl wird gespeichert und gilt, sobald die Schule sie wieder einschaltet."
            }
          />
        </div>
      )}

      {loading ? (
        <div
          data-testid="notification-preferences-skeleton"
          className="space-y-5"
          aria-hidden="true"
        >
          {[2, 3].map((rowCount) => (
            <div key={rowCount}>
              <Skeleton className="mb-2 h-3 w-28" />
              <div className="divide-y divide-gray-100">
                {Array.from({ length: rowCount }, (_, index) => (
                  <div
                    key={index}
                    className="flex min-h-16 items-center justify-between gap-4 py-3"
                  >
                    <div className="min-w-0 flex-1 space-y-2">
                      <Skeleton className="h-4 w-40 max-w-2/3" />
                      <Skeleton className="h-3 w-64 max-w-full" />
                    </div>
                    <Skeleton className="h-6 w-11 shrink-0 rounded-full" />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-5">
          {groups.map(({ group, label, items }) => (
            <div key={group}>
              <h4 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {label}
              </h4>
              <div className="divide-y divide-gray-100">
                {items.map((type) => (
                  <div
                    key={type.key}
                    className="flex items-start justify-between gap-4 py-3"
                  >
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-gray-900">
                        {isParentPortal && PARENT_TYPES.has(type.key)
                          ? t(`types.${type.key}.label`)
                          : type.label}
                      </p>
                      <p className="text-sm text-gray-600">
                        {isParentPortal && PARENT_TYPES.has(type.key)
                          ? t(`types.${type.key}.description`)
                          : type.description}
                      </p>
                      {!type.available && (
                        // The switch stays usable on purpose: the choice
                        // belongs to the person and must survive the school
                        // toggling its own setting off and on again.
                        <p className="mt-1 text-xs text-gray-500">
                          {isParentPortal
                            ? t("typeUnavailable")
                            : "Von Ihrer Schule derzeit deaktiviert."}
                        </p>
                      )}
                    </div>
                    <BooleanField
                      value={type.enabled}
                      onChange={(next) => void toggle(type, next)}
                      disabled={busy}
                      ariaLabel={
                        isParentPortal && PARENT_TYPES.has(type.key)
                          ? t(`types.${type.key}.label`)
                          : type.label
                      }
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
