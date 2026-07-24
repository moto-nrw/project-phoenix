"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { useLatest } from "~/lib/hooks/use-latest";

// Resource definitions with German labels
const RESOURCES = [
  { value: "users", label: "Benutzer" },
  { value: "roles", label: "Rollen" },
  { value: "permissions", label: "Berechtigungen" },
  { value: "activities", label: "Aktivitäten" },
  { value: "rooms", label: "Räume" },
  { value: "groups", label: "Gruppen" },
  { value: "substitutions", label: "Vertretungen" },
  { value: "schedules", label: "Stundenpläne" },
  { value: "visits", label: "Besuche" },
  { value: "feedback", label: "Feedback" },
  { value: "config", label: "Konfiguration" },
  { value: "iot", label: "IoT-Geräte" },
  { value: "auth", label: "Authentifizierung" },
  { value: "suggestions", label: "Vorschläge" },
  { value: "time_tracking", label: "Zeiterfassung" },
  { value: "grade_transitions", label: "Klassenwechsel" },
  { value: "system", label: "System" },
  { value: "admin", label: "Administration" },
] as const;

// Action labels in German
const ACTION_LABELS: Record<string, string> = {
  create: "Erstellen",
  read: "Lesen",
  update: "Bearbeiten",
  delete: "Löschen",
  list: "Auflisten",
  manage: "Verwalten",
  enroll: "Einschreiben",
  assign: "Zuweisen",
  own: "Eigene",
  apply: "Anwenden",
};

// Actions available per resource (matching Go backend constants)
const RESOURCE_ACTIONS: Record<string, string[]> = {
  users: ["create", "read", "update", "delete", "list", "manage"],
  roles: ["create", "read", "update", "delete"],
  permissions: ["create", "read", "update", "delete"],
  activities: [
    "create",
    "read",
    "update",
    "delete",
    "list",
    "manage",
    "enroll",
    "assign",
  ],
  rooms: ["create", "read", "update", "delete", "list", "manage"],
  groups: ["create", "read", "update", "delete", "list", "manage", "assign"],
  substitutions: ["create", "read", "update", "delete", "list", "manage"],
  schedules: ["create", "read", "update", "delete", "list", "manage"],
  visits: ["create", "read", "update", "delete", "list", "manage"],
  feedback: ["create", "read", "delete", "list", "manage"],
  config: ["read", "update", "manage"],
  iot: ["read", "update", "manage"],
  auth: ["manage"],
  suggestions: ["create", "read", "update", "delete", "list", "manage"],
  time_tracking: ["own"],
  grade_transitions: ["read", "create", "update", "delete", "apply"],
  system: ["manage"],
  admin: ["*"],
};

export interface PermissionSelectorValue {
  resource: string;
  action: string;
}

interface PermissionSelectorProps {
  readonly value: PermissionSelectorValue | undefined;
  readonly onChange: (value: PermissionSelectorValue) => void;
  readonly required?: boolean;
}

export function PermissionSelector({
  value,
  onChange,
  required,
}: PermissionSelectorProps) {
  const [resource, setResource] = useState(value?.resource ?? "");
  const [action, setAction] = useState(value?.action ?? "");

  // Use ref to store onChange to avoid dependency issues
  const onChangeRef = useLatest(onChange);

  // Track if we're syncing from external value to prevent loops
  const isSyncingRef = useRef(false);

  // Sync with external value changes (e.g., when editing existing permission)
  useEffect(() => {
    if (value?.resource !== undefined && value.resource !== resource) {
      isSyncingRef.current = true;
      setResource(value.resource);
    }
    if (value?.action !== undefined && value.action !== action) {
      isSyncingRef.current = true;
      setAction(value.action);
    }
    // Reset sync flag after state updates
    const timer = setTimeout(() => {
      isSyncingRef.current = false;
    }, 0);
    return () => clearTimeout(timer);
  }, [value?.resource, value?.action]); // eslint-disable-line react-hooks/exhaustive-deps

  // Reset action when resource changes if current action is not available
  useEffect(() => {
    if (resource && !isSyncingRef.current) {
      const availableActions = RESOURCE_ACTIONS[resource] ?? [];
      if (action && !availableActions.includes(action)) {
        setAction("");
      }
    }
  }, [resource, action]);

  const availableActions = resource ? (RESOURCE_ACTIONS[resource] ?? []) : [];

  // Handlers that notify parent of changes
  const handleResourceChange = useCallback(
    (newResource: string) => {
      setResource(newResource);
      // Reset action if it's not valid for the new resource
      const availableActions = RESOURCE_ACTIONS[newResource] ?? [];
      if (action && !availableActions.includes(action)) {
        setAction("");
      }
    },
    [action],
  );

  const handleActionChange = useCallback(
    (newAction: string) => {
      setAction(newAction);
      // Notify parent immediately when both values are set
      if (resource && newAction) {
        onChangeRef.current({ resource, action: newAction });
      }
    },
    [onChangeRef, resource],
  );

  // Also notify when resource changes and action is already set
  useEffect(() => {
    if (resource && action && !isSyncingRef.current) {
      onChangeRef.current({ resource, action });
    }
  }, [resource, action, onChangeRef]);

  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
      {/* Resource Dropdown */}
      <div>
        <label
          id="permission-resource-label"
          htmlFor="permission-resource"
          className="mb-1.5 block text-xs font-medium text-gray-700"
        >
          Ressource{required && "*"}
        </label>
        <CustomSelect
          id="permission-resource"
          ariaLabelledBy="permission-resource-label"
          value={resource}
          onChange={handleResourceChange}
          options={RESOURCES.map((r) => ({ value: r.value, label: r.label }))}
          placeholder="Ressource auswählen..."
          required={required}
        />
      </div>

      {/* Action Dropdown */}
      <div>
        <label
          id="permission-action-label"
          htmlFor="permission-action"
          className="mb-1.5 block text-xs font-medium text-gray-700"
        >
          Aktion{required && "*"}
        </label>
        <CustomSelect
          id="permission-action"
          ariaLabelledBy="permission-action-label"
          value={action}
          onChange={handleActionChange}
          options={availableActions.map((a) => ({
            value: a,
            label: ACTION_LABELS[a] ?? a,
          }))}
          placeholder={
            resource ? "Aktion auswählen..." : "Zuerst Ressource wählen"
          }
          required={required}
          disabled={!resource}
        />
      </div>

      {/* Preview of generated permission name */}
      {resource && action && (
        <div className="md:col-span-2">
          <p className="text-xs text-gray-500">
            Berechtigungsname:{" "}
            <code className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-pink-600">
              {resource}:{action}
            </code>
          </p>
        </div>
      )}
    </div>
  );
}

// Export constants used by other components
export { RESOURCES, ACTION_LABELS };
