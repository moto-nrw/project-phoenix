"use client";

import { useState, useEffect, useRef, useCallback } from "react";

// Resource definitions with German labels
const RESOURCES = [
  { value: "users", label: "Benutzer" },
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
};

// Actions available per resource (matching Go backend constants)
const RESOURCE_ACTIONS: Record<string, string[]> = {
  users: ["create", "read", "update", "delete", "list", "manage"],
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
  feedback: ["create", "read", "delete", "list", "manage"], // no update
  config: ["read", "update", "manage"], // no create/delete/list
  iot: ["read", "update", "manage"], // no create/delete/list
  auth: ["manage"], // only manage
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
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

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
    [resource],
  );

  // Also notify when resource changes and action is already set
  useEffect(() => {
    if (resource && action && !isSyncingRef.current) {
      onChangeRef.current({ resource, action });
    }
  }, [resource, action]);

  const baseSelectClasses =
    "w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-all duration-200 focus:ring-2 focus:ring-pink-500 focus:outline-none appearance-none pr-10";

  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
      {/* Resource Dropdown */}
      <div>
        <label
          htmlFor="permission-resource"
          className="mb-1.5 block text-xs font-medium text-gray-700"
        >
          Ressource{required && "*"}
        </label>
        <div className="relative">
          <select
            id="permission-resource"
            value={resource}
            onChange={(e) => handleResourceChange(e.target.value)}
            className={baseSelectClasses}
            required={required}
          >
            <option value="">Ressource auswählen...</option>
            {RESOURCES.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </select>
          <svg
            className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </div>
      </div>

      {/* Action Dropdown */}
      <div>
        <label
          htmlFor="permission-action"
          className="mb-1.5 block text-xs font-medium text-gray-700"
        >
          Aktion{required && "*"}
        </label>
        <div className="relative">
          <select
            id="permission-action"
            value={action}
            onChange={(e) => handleActionChange(e.target.value)}
            className={baseSelectClasses}
            required={required}
            disabled={!resource}
          >
            <option value="">
              {resource ? "Aktion auswählen..." : "Zuerst Ressource wählen"}
            </option>
            {availableActions.map((a) => (
              <option key={a} value={a}>
                {ACTION_LABELS[a] ?? a}
              </option>
            ))}
          </select>
          <svg
            className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </div>
      </div>

      {/* Preview of generated permission name */}
      {resource && action && (
        <div className="md:col-span-2">
          <p className="text-xs text-gray-500">
            Permission-Name:{" "}
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
