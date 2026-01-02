"use client";

// Permission Entity Configuration

import { defineEntityConfig } from "../types";
import { databaseThemes } from "@/components/ui/database/themes";
import type { Permission, BackendPermission } from "@/lib/auth-helpers";
import { mapPermissionResponse } from "@/lib/auth-helpers";
import { formatPermissionDisplay } from "@/lib/permission-labels";
import {
  PermissionSelector,
  type PermissionSelectorValue,
  RESOURCES,
  ACTION_LABELS,
} from "@/components/permissions/permission-selector";

function displayName(p: Permission) {
  return formatPermissionDisplay(p.resource, p.action);
}

// Wrapper component for the custom field
function PermissionSelectorField({
  value,
  onChange,
  required,
}: Readonly<{
  value: unknown;
  onChange: (value: unknown) => void;
  label: string;
  required?: boolean;
}>) {
  // Extract resource and action from the form value
  const selectorValue: PermissionSelectorValue | undefined =
    value && typeof value === "object" && "resource" in value
      ? (value as PermissionSelectorValue)
      : undefined;

  return (
    <PermissionSelector
      value={selectorValue}
      onChange={(newValue) => onChange(newValue)}
      required={required}
    />
  );
}

export const permissionsConfig = defineEntityConfig<Permission>({
  name: {
    singular: "Berechtigung",
    plural: "Berechtigungen",
  },

  theme: databaseThemes.permissions,

  backUrl: "/database",

  api: {
    basePath: "/api/auth/permissions",
  },

  form: {
    sections: [
      {
        title: "Berechtigungsdetails",
        backgroundColor: "bg-pink-50/30",
        // Key icon - symbolizes access/permissions
        iconPath:
          "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z",
        columns: 2,
        fields: [
          {
            name: "permissionSelector",
            label: "Berechtigung",
            type: "custom",
            required: true,
            component: PermissionSelectorField,
            colSpan: 2,
          },
          {
            name: "name",
            label: "Anzeigename (optional)",
            type: "text",
            required: false,
            placeholder: "Wird automatisch generiert, falls leer",
            helperText:
              "Optionaler deutscher Anzeigename für diese Berechtigung",
            colSpan: 2,
          },
          {
            name: "description",
            label: "Beschreibung",
            type: "textarea",
            required: false,
            placeholder: "Kurze Beschreibung, was diese Berechtigung erlaubt",
            colSpan: 2,
          },
        ],
      },
    ],

    defaultValues: {},

    // Transform form data before submission
    // Form data includes custom fields like permissionSelector, so cast internally
    transformBeforeSubmit: (data) => {
      // Cast to access custom form fields (permissionSelector is not part of Permission type)
      const formData = data as unknown as Record<string, unknown>;
      const selector = formData.permissionSelector as
        | PermissionSelectorValue
        | undefined;
      const resource = selector?.resource ?? "";
      const action = selector?.action ?? "";

      // Auto-generate name if not provided (format: "ressource:aktion")
      let name = (formData.name as string | undefined) ?? "";
      if (!name && resource && action) {
        const resourceLabel =
          RESOURCES.find((r) => r.value === resource)?.label ?? resource;
        const actionLabel = ACTION_LABELS[action] ?? action;
        name = `${resourceLabel.toLowerCase()}:${actionLabel.toLowerCase()}`;
      }

      return {
        name: name || `${resource}:${action}`,
        description: (formData.description as string | undefined) ?? "",
        resource,
        action,
      };
    },
  },

  detail: {
    header: {
      title: (p: Permission) => displayName(p),
      subtitle: (p: Permission) => p.description || "Keine Beschreibung",
      avatar: {
        text: (p: Permission) => (p.resource?.[0] ?? "P").toUpperCase(),
        size: "lg",
      },
      badges: [
        {
          label: (p: Permission) => p.name || "Systemberechtigung",
          color: "bg-indigo-400/80",
          showWhen: () => true,
        },
      ],
    },
    sections: [
      {
        title: "Technische Daten",
        titleColor: "text-indigo-800",
        items: [
          { label: "Ressource", value: (p: Permission) => p.resource },
          { label: "Aktion", value: (p: Permission) => p.action },
          { label: "Anzeigename", value: (p: Permission) => p.name },
          {
            label: "Beschreibung",
            value: (p: Permission) => p.description || "Keine Beschreibung",
          },
          { label: "ID", value: (p: Permission) => p.id },
          {
            label: "Erstellt am",
            value: (p: Permission) =>
              new Date(p.createdAt).toLocaleDateString("de-DE"),
          },
          {
            label: "Aktualisiert am",
            value: (p: Permission) =>
              new Date(p.updatedAt).toLocaleDateString("de-DE"),
          },
        ],
      },
    ],
  },

  list: {
    title: "Berechtigungen verwalten",
    description: "Systemweite Berechtigungen definieren und prüfen",
    searchPlaceholder: "Berechtigungen durchsuchen...",

    searchStrategy: "frontend",
    searchableFields: ["name", "description", "resource", "action"],
    minSearchLength: 0,

    item: {
      title: (p: Permission) => displayName(p),
      subtitle: (p: Permission) => p.name,
      description: (p: Permission) => p.description || "",
      avatar: {
        text: (p: Permission) => (p.resource?.[0] ?? "P").toUpperCase(),
      },
      badges: [],
    },
  },

  service: {
    mapResponse: (data: unknown): Permission => {
      let actual = data;
      if (
        data &&
        typeof data === "object" &&
        "status" in data &&
        "data" in data
      ) {
        actual = (data as { status: string; data: unknown }).data;
      }
      return mapPermissionResponse(actual as BackendPermission);
    },
  },

  labels: {
    createButton: "Neue Berechtigung erstellen",
    createModalTitle: "Neue Berechtigung",
    editModalTitle: "Berechtigung bearbeiten",
    detailModalTitle: "Berechtigungsdetails",
    deleteConfirmation:
      "Diese Berechtigung wirklich löschen? Dies kann bestehende Rollen betreffen.",
  },
});
