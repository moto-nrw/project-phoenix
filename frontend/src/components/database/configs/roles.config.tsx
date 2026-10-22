"use client";

// Role Entity Configuration

import { defineEntityConfig, type SectionConfig } from "@/lib/database/types";
import type { Role, BackendRole } from "@/lib/auth-helpers";
import {
  mapRoleResponse,
  getRoleDisplayName,
  getRoleDisplayDescription,
  getBaseRoleLabel,
  BASE_ROLE_LABELS,
} from "@/lib/auth-helpers";
import { ROLE_TEMPLATES } from "@/lib/role-labels";

const nameField = {
  name: "name",
  label: "Name",
  type: "text",
  required: true,
  placeholder: "z. B. OGS-Leitung, Springkraft",
} as const;

const descriptionField = {
  name: "description",
  label: "Beschreibung",
  type: "textarea",
  required: false,
  placeholder: "Wofür ist diese Rolle da?",
} as const;

/**
 * Stammdaten einer bestehenden Rolle (#4574). „Gilt als" ist base_role: es
 * steuert Mitteilungen und Einladungen, nicht die Rechte. Die Vorlage gibt
 * es nur beim Anlegen.
 */
export const roleEditSections: SectionConfig[] = [
  {
    title: "Rolleninformationen",
    concept: "roles",
    columns: 1,
    fields: [
      nameField,
      descriptionField,
      {
        name: "baseRole",
        label: "Gilt als",
        type: "select",
        required: true,
        placeholder: "Bitte wählen",
        helperText:
          "Bestimmt, welche Mitteilungen die Rolle bekommt. Ändert keine Rechte.",
        options: Object.entries(BASE_ROLE_LABELS).map(([value, label]) => ({
          value,
          label,
        })),
      },
    ],
  },
];

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
    // Anlegen (#4574): die neue Rolle startet mit den Rechten der Vorlage.
    sections: [
      {
        title: "Rolleninformationen",
        concept: "roles",
        columns: 1,
        fields: [
          nameField,
          descriptionField,
          {
            name: "template",
            label: "Vorlage",
            type: "select",
            required: true,
            placeholder: "Vorlage wählen",
            helperText:
              "Die neue Rolle startet mit den Rechten der Vorlage. Danach können Sie die Rechte frei ändern.",
            options: ROLE_TEMPLATES.map(({ value, label }) => ({
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
    },

    sections: [
      {
        title: "Rolleninformationen",
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
            label: "Gilt als",
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
    description: "Legen Sie fest, wer in moto was darf.",
    searchPlaceholder: "Rollen durchsuchen…",

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
    },
  },

  service: {
    mapRequest: (
      data: Partial<Role> & { template?: string },
    ): Record<string, unknown> => ({
      name: data.name,
      description: data.description,
      // Beim Anlegen setzt die Vorlage auch base_role (#4574).
      ...(data.template
        ? { template: data.template }
        : !data.isSystem && { base_role: data.baseRole }),
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
