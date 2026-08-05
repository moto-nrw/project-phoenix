"use client";

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { createLogger } from "~/lib/logger";
import {
  disableAllNotificationPreferences,
  fetchNotificationPreferences,
  setNotificationPreference,
  type NotificationPreferenceType,
  type PreferencePortal,
} from "~/lib/notification-preferences-api";

const logger = createLogger({ component: "NotificationPreferencesSection" });

/** German headings for the registry groups, in display order. */
const GROUP_LABELS: Record<string, string> = {
  abholung: "Abholungen",
  aktivitaeten: "Aktivitäten",
  kinder: "Kinder",
  mitteilungen: "Mitteilungen",
};

const GROUP_ORDER = ["abholung", "aktivitaeten", "kinder", "mitteilungen"];

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
  const [types, setTypes] = useState<NotificationPreferenceType[]>([]);
  const [tenantEnabled, setTenantEnabled] = useState(true);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
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
      setError("Die Einstellungen konnten nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [portal]);

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
      setError("Die Einstellung konnte nicht gespeichert werden.");
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
      setError("Die Einstellungen konnten nicht geändert werden.");
    } finally {
      setBusy(false);
    }
  };

  const anyEnabled = types.some((t) => t.enabled);
  const groups = GROUP_ORDER.map((group) => ({
    group,
    label: GROUP_LABELS[group] ?? group,
    items: types.filter((t) => t.group === group),
  })).filter((g) => g.items.length > 0);

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <ConceptSectionHeader
        className="mb-4"
        title="Benachrichtigungen"
        concept="notifications"
        actions={
          anyEnabled && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void disableAll()}
            >
              Alle deaktivieren
            </Button>
          )
        }
      />

      <p className="mb-4 text-sm text-gray-600">
        Wählen Sie, worüber moto Sie informieren soll. Es wird nur verschickt,
        was Sie hier einschalten.
      </p>

      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}

      {!loading && !tenantEnabled && (
        <div className="mb-3">
          <Alert
            type="info"
            message="Ihre Schule hat Benachrichtigungen derzeit deaktiviert. Ihre Auswahl wird gespeichert und gilt, sobald die Schule sie wieder einschaltet."
          />
        </div>
      )}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
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
                        {type.label}
                      </p>
                      <p className="text-sm text-gray-600">
                        {type.description}
                      </p>
                      {!type.available && (
                        // The switch stays usable on purpose: the choice
                        // belongs to the person and must survive the school
                        // toggling its own setting off and on again.
                        <p className="mt-1 text-xs text-gray-500">
                          Von Ihrer Schule derzeit deaktiviert.
                        </p>
                      )}
                    </div>
                    <BooleanField
                      value={type.enabled}
                      onChange={(next) => void toggle(type, next)}
                      disabled={busy}
                      ariaLabel={type.label}
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
