"use client";

import { useCallback, useEffect, useState } from "react";
import {
  fetchOGKeyHistory,
  fetchOGSettingHistory,
  fetchSettingHistory,
  type HistoryFilters,
} from "~/lib/settings-api";
import { getScopeLabel, type SettingChange } from "~/lib/settings-helpers";

interface SettingHistoryProps {
  /** OG ID for OG-scoped history */
  readonly ogId?: string;
  /** Setting key for key-specific history */
  readonly settingKey?: string;
  /** Filters for general history query */
  readonly filters?: HistoryFilters;
  /** Maximum number of entries to show */
  readonly limit?: number;
  /** Title for the history section */
  readonly title?: string;
}

export function SettingHistory({
  ogId,
  settingKey,
  filters,
  limit = 20,
  title = "Änderungsverlauf",
}: SettingHistoryProps) {
  const [history, setHistory] = useState<SettingChange[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadHistory = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      let data: SettingChange[];

      if (ogId && settingKey) {
        data = await fetchOGKeyHistory(ogId, settingKey, limit);
      } else if (ogId) {
        data = await fetchOGSettingHistory(ogId, limit);
      } else {
        data = await fetchSettingHistory({ ...filters, limit });
      }

      setHistory(data);
    } catch (err) {
      console.error("Failed to load setting history:", err);
      setError("Fehler beim Laden des Verlaufs");
    } finally {
      setLoading(false);
    }
  }, [ogId, settingKey, filters, limit]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  const formatDate = (date: Date): string => {
    return date.toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const formatValue = (value: unknown): string => {
    if (value === null || value === undefined) {
      return "-";
    }
    if (typeof value === "boolean") {
      return value ? "Ja" : "Nein";
    }
    if (typeof value === "string" || typeof value === "number") {
      return String(value);
    }
    return JSON.stringify(value);
  };

  const getChangeTypeLabel = (
    changeType: SettingChange["changeType"],
  ): string => {
    switch (changeType) {
      case "create":
        return "Erstellt";
      case "update":
        return "Geändert";
      case "delete":
        return "Gelöscht";
      case "reset":
        return "Zurückgesetzt";
      default:
        return changeType;
    }
  };

  const getChangeTypeColor = (
    changeType: SettingChange["changeType"],
  ): string => {
    switch (changeType) {
      case "create":
        return "bg-green-100 text-green-800";
      case "update":
        return "bg-blue-100 text-blue-800";
      case "delete":
        return "bg-red-100 text-red-800";
      case "reset":
        return "bg-yellow-100 text-yellow-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  if (loading) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
        <h3 className="mb-4 text-base font-semibold text-gray-900">{title}</h3>
        <div className="flex items-center justify-center py-4">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-gray-900" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
        <h3 className="mb-4 text-base font-semibold text-gray-900">{title}</h3>
        <p className="text-sm text-red-600">{error}</p>
      </div>
    );
  }

  if (history.length === 0) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 backdrop-blur-sm">
        <h3 className="mb-4 text-base font-semibold text-gray-900">{title}</h3>
        <p className="text-sm text-gray-500">Keine Änderungen vorhanden.</p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
      <h3 className="mb-4 text-base font-semibold text-gray-900">{title}</h3>

      <div className="space-y-3">
        {history.map((change) => (
          <div
            key={change.id}
            className="rounded-lg border border-gray-100 bg-white p-3"
          >
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="flex items-center gap-2">
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${getChangeTypeColor(
                    change.changeType,
                  )}`}
                >
                  {getChangeTypeLabel(change.changeType)}
                </span>
                <span className="text-sm font-medium text-gray-800">
                  {change.settingKey}
                </span>
              </div>
              <span className="text-xs text-gray-500">
                {formatDate(change.createdAt)}
              </span>
            </div>

            <div className="mt-2 space-y-1">
              {change.changeType !== "delete" &&
                change.changeType !== "reset" && (
                  <div className="flex items-center gap-2 text-sm">
                    {change.oldValue !== undefined && (
                      <>
                        <span className="text-gray-500">
                          {formatValue(change.oldValue)}
                        </span>
                        <span className="text-gray-400">→</span>
                      </>
                    )}
                    <span className="font-medium text-gray-700">
                      {formatValue(change.newValue)}
                    </span>
                  </div>
                )}

              {change.reason && (
                <p className="text-xs text-gray-500 italic">
                  &quot;{change.reason}&quot;
                </p>
              )}

              <div className="flex items-center gap-3 text-xs text-gray-500">
                {change.accountEmail && <span>von {change.accountEmail}</span>}
                {change.scopeType && (
                  <span>
                    Bereich: {getScopeLabel(change.scopeType)}
                    {change.scopeId && ` #${change.scopeId}`}
                  </span>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {history.length >= limit && (
        <p className="mt-4 text-center text-xs text-gray-500">
          Zeige die letzten {limit} Änderungen
        </p>
      )}
    </div>
  );
}
