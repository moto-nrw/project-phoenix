"use client";

import { useState, useCallback } from "react";
import type { SettingCategory, Scope } from "~/lib/settings-helpers";
import { SettingControl } from "./setting-control";
import { ActionControl } from "./action-control";
import { SettingInheritanceBadge } from "./setting-inheritance-badge";
import { clientSetSettingValue } from "~/lib/settings-api";
import { InfoTooltip } from "~/components/ui/info-tooltip";

// Threshold for showing description inline vs tooltip
const INLINE_DESCRIPTION_MAX_LENGTH = 80;

interface SettingsCategoryProps {
  category: SettingCategory;
  scope: Scope;
  scopeId?: string;
  deviceId?: string;
  onSettingChanged?: () => void;
}

export function SettingsCategory({
  category,
  scope,
  scopeId,
  deviceId,
  onSettingChanged,
}: SettingsCategoryProps) {
  const [savingKeys, setSavingKeys] = useState<Set<string>>(new Set());
  const [savedKeys, setSavedKeys] = useState<Set<string>>(new Set());
  const [errorKeys, setErrorKeys] = useState<Set<string>>(new Set());

  const handleSettingChange = useCallback(
    async (key: string, value: string) => {
      setSavingKeys((prev) => new Set(prev).add(key));
      setSavedKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
      setErrorKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });

      const success = await clientSetSettingValue(key, {
        value,
        scope,
        scopeId,
      });

      setSavingKeys((prev) => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });

      if (success) {
        setSavedKeys((prev) => new Set(prev).add(key));
        // Clear saved indicator after 2 seconds
        setTimeout(() => {
          setSavedKeys((prev) => {
            const next = new Set(prev);
            next.delete(key);
            return next;
          });
        }, 2000);
        onSettingChanged?.();
      } else {
        setErrorKeys((prev) => new Set(prev).add(key));
      }
    },
    [scope, scopeId, onSettingChanged],
  );

  return (
    <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
      <div className="border-b border-gray-200 bg-gray-50 px-4 py-3">
        <h3 className="text-lg font-medium text-gray-900">{category.name}</h3>
      </div>
      <div className="divide-y divide-gray-200">
        {category.settings.map((setting) => (
          <div
            key={setting.key}
            className="px-4 py-4 sm:grid sm:grid-cols-3 sm:gap-4"
          >
            <div className="mb-2 sm:mb-0">
              <div className="flex items-center gap-1.5">
                <label className="block text-sm font-medium text-gray-900">
                  {setting.definition.label ?? setting.key}
                </label>
                {/* Show tooltip for long descriptions */}
                {setting.definition.description &&
                  setting.definition.description.length >
                    INLINE_DESCRIPTION_MAX_LENGTH && (
                    <InfoTooltip content={setting.definition.description} />
                  )}
              </div>
              {/* Show short descriptions inline */}
              {setting.definition.description &&
                setting.definition.description.length <=
                  INLINE_DESCRIPTION_MAX_LENGTH && (
                  <p className="mt-1 text-sm text-gray-500">
                    {setting.definition.description}
                  </p>
                )}
              <div className="mt-1">
                <SettingInheritanceBadge
                  scope={setting.effectiveScope}
                  isOverridden={setting.isOverridden}
                />
              </div>
              {setting.definition.requiresRestart && (
                <p className="mt-1 text-xs text-amber-600">
                  Erfordert Neustart
                </p>
              )}
            </div>
            <div className="sm:col-span-2">
              {setting.definition.valueType === "action" ? (
                <ActionControl setting={setting} />
              ) : (
                <>
                  <div className="flex items-center gap-2">
                    <div className="flex-1">
                      <SettingControl
                        setting={setting}
                        onChange={(value) =>
                          handleSettingChange(setting.key, value)
                        }
                        deviceId={deviceId}
                      />
                    </div>
                    {savingKeys.has(setting.key) && (
                      <span className="text-sm text-gray-500">Speichern...</span>
                    )}
                    {savedKeys.has(setting.key) && (
                      <span className="text-sm text-green-600">Gespeichert</span>
                    )}
                    {errorKeys.has(setting.key) && (
                      <span className="text-sm text-red-600">Fehler</span>
                    )}
                  </div>
                  {!setting.canEdit && (
                    <p className="mt-1 text-xs text-gray-500">
                      Sie haben keine Berechtigung, diese Einstellung zu ändern.
                    </p>
                  )}
                </>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
