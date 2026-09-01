"use client";

// Role Entity Configuration

import { defineEntityConfig } from "@/lib/database/types";
import type { Role, BackendRole } from "@/lib/auth-helpers";
import {
  mapRoleResponse,
  getRoleDisplayName,
  getRoleDisplayDescription,
  getBaseRoleLabel,
  BASE_ROLE_LABELS,
} from "@/lib/auth-helpers";

export const rolesConfig = defineEntityConfig<Role>({
  name: {
    singular: "Rolle",
    plural: "Rollen",
  },

  concept: "roles",

  backUrl: "/database",

  api: {
    basePath: "/api/auth/roles",
  },

  form: {
    sections: [
      {
        title: "Rolleninformationen",
        concept: "roles",
        columns: 1,
        fields: [
          {
            name: "name",
            label: "Name",
            type: "text",
            required: true,
            placeholder: "z.B. Pädagogische Fachkraft, Administrator",
          },
          {
            name: "description",
            label: "Beschreibung",
            type: "textarea",
            required: false,
            placeholder:
              "Beschreiben Sie die Aufgaben und Verantwortlichkeiten dieser Rolle",
          },
          {
            name: "baseRole",
            label: "Systemrolle",
            type: "select",
            required: true,
            placeholder: "Systemrolle auswählen",
            helperText:
              "Ordnet diese Rolle einer Systemrolle zu, damit Ankündigungen korrekt zugestellt werden",
            options: Object.entries(BASE_ROLE_LABELS).map(([value, label]) => ({
              value,
              label,
            })),
          },
        ],
      },
    ],

    defaultValues: {},
  },

  detail: {
    header: {
      title: (role: Role) => getRoleDisplayName(role.name),
      subtitle: (role: Role) =>
        getRoleDisplayDescription(role.name, role.description) ||
        "Keine Beschreibung",
      avatar: {
        text: (role: Role) => getRoleDisplayName(role.name)?.[0] ?? "R",
        size: "lg",
      },
      badges: [
        {
          label: (role: Role) =>
            `${role.permissions?.length ?? 0} Berechtigungen`,
          color: "bg-moto-blue-light/80",
          showWhen: () => true,
        },
      ],
    },

    sections: [
      {
        title: "Rolleninformationen",
        titleColor: "text-gray-800",
        items: [
          {
            label: "Name",
            value: (role: Role) => getRoleDisplayName(role.name),
          },
          {
            label: "Beschreibung",
            value: (role: Role) =>
              getRoleDisplayDescription(role.name, role.description) ||
              "Keine Beschreibung",
          },
          {
            label: "Systemrolle",
            value: (role: Role) => getBaseRoleLabel(role.baseRole),
          },
          {
            label: "Berechtigungen",
            value: (role: Role) =>
              `${role.permissions?.length ?? 0} Berechtigungen zugewiesen`,
          },
          {
            label: "Erstellt am",
            value: (role: Role) =>
              new Date(role.createdAt).toLocaleDateString("de-DE", {
                day: "2-digit",
                month: "2-digit",
                year: "numeric",
              }),
          },
          {
            label: "Aktualisiert am",
            value: (role: Role) =>
              new Date(role.updatedAt).toLocaleDateString("de-DE", {
                day: "2-digit",
                month: "2-digit",
                year: "numeric",
              }),
          },
        ],
      },
    ],
  },

  list: {
    title: "Rollen verwalten",
    description: "Verwalten Sie Systemrollen und deren Berechtigungen",
    searchPlaceholder: "Rollen durchsuchen...",

    // Frontend search configuration
    searchStrategy: "frontend",
    searchableFields: ["name", "description"],
    minSearchLength: 0,

    item: {
      title: (role: Role) => getRoleDisplayName(role.name),
      subtitle: (role: Role) =>
        getRoleDisplayDescription(role.name, role.description) ||
        "Keine Beschreibung",
      avatar: {
        text: (role: Role) => getRoleDisplayName(role.name)?.[0] ?? "R",
      },
      badges: [],
    },
  },

  service: {
    mapRequest: (data: Partial<Role>): Record<string, unknown> => ({
      name: data.name,
      description: data.description,
      ...(!data.isSystem && { base_role: data.baseRole }),
    }),
    mapResponse: (data: unknown): Role => {
      // Handle wrapped response format
      let actualData = data;
      if (
        data &&
        typeof data === "object" &&
        "status" in data &&
        "data" in data
      ) {
        actualData = (data as { status: string; data: unknown }).data;
      }

      return mapRoleResponse(actualData as BackendRole);
    },
  },

  labels: {
    createButton: "Neue Rolle erstellen",
    createModalTitle: "Neue Rolle",
    editModalTitle: "Rolle bearbeiten",
    detailModalTitle: "Rollendetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie diese Rolle löschen möchten? Alle Benutzer mit dieser Rolle verlieren ihre Berechtigungen.",
  },
});
