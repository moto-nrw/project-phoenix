// Group Entity Configuration

import { defineEntityConfig } from "../types";
import { databaseThemes } from "@/components/ui/database/themes";
import type { Group, BackendGroup } from "@/lib/group-helpers";
import { mapGroupResponse } from "@/lib/group-helpers";

export const groupsConfig = defineEntityConfig<Group>({
  name: {
    singular: "Gruppe",
    plural: "Gruppen",
  },

  theme: databaseThemes.groups,

  backUrl: "/database",

  api: {
    basePath: "/api/groups",
  },

  form: {
    sections: [
      {
        title: "Gruppendetails",
        backgroundColor: "bg-green-50/30",
        iconPath:
          "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0",
        columns: 2,
        fields: [
          {
            name: "name",
            label: "Gruppenname",
            type: "text",
            required: true,
            placeholder: "z.B. Gruppe Blau",
          },
          {
            name: "room_id",
            label: "Raum",
            type: "select",
            required: false,
            options: async () => {
              try {
                // Fetch rooms from API
                const response = await fetch("/api/rooms");
                const result = (await response.json()) as {
                  data?: Array<{ id: number; name: string }>;
                };
                const rooms = result.data ?? [];
                return [
                  { value: "", label: "Kein Raum" },
                  ...rooms.map((room) => ({
                    value: room.id.toString(),
                    label: room.name,
                  })),
                ];
              } catch (error) {
                console.error("Failed to fetch rooms:", error);
                return [{ value: "", label: "Kein Raum" }];
              }
            },
          },
          {
            name: "teacher_ids",
            label: "Aufsichtspersonen",
            type: "multiselect",
            required: false,
            colSpan: 2,
            options: async () => {
              try {
                // Fetch teachers from staff API (filtered for teachers only)
                const response = await fetch("/api/staff?teachers_only=true");
                const result = (await response.json()) as
                  | {
                      data?: Array<{
                        id: string;
                        name: string;
                        specialization?: string;
                      }>;
                    }
                  | Array<{
                      id: string;
                      name: string;
                      specialization?: string;
                    }>;

                // Handle both array and wrapped response formats
                const teachers = Array.isArray(result)
                  ? result
                  : (result.data ?? []);

                return teachers.map((teacher) => ({
                  value: teacher.id.toString(),
                  label: teacher.specialization
                    ? `${teacher.name} (${teacher.specialization})`
                    : teacher.name,
                }));
              } catch (error) {
                console.error("Failed to fetch teachers:", error);
                return [];
              }
            },
            placeholder: "Aufsichtspersonen auswählen...",
            helperText:
              "Wählen Sie eine oder mehrere Aufsichtspersonen für diese Gruppe aus",
          },
        ],
      },
    ],

    defaultValues: {},

    transformBeforeSubmit: (data) => ({
      ...data,
      room_id: data.room_id ?? undefined,
    }),
  },

  detail: {
    header: {
      title: (group: Group) => group.name,
      subtitle: (group: Group) => group.room_name ?? "Kein Raum zugewiesen",
      avatar: {
        text: (group: Group) => group.name?.[0] ?? "G",
        size: "lg",
      },
      badges: [
        {
          label: (group: Group) => `${group.student_count ?? 0} Schüler`,
          color: "bg-green-400/80",
          showWhen: () => true,
        },
        {
          label: "Raum zugewiesen",
          color: "bg-blue-400/80",
          showWhen: (group: Group) => !!group.room_name,
        },
        {
          label: (group: Group) =>
            `${group.supervisors?.length ?? 0} Aufsichtsperson${(group.supervisors?.length ?? 0) === 1 ? "" : "en"}`,
          color: "bg-indigo-400/80",
          showWhen: (group: Group) => (group.supervisors?.length ?? 0) > 0,
        },
      ],
    },

    sections: [
      {
        title: "Gruppendetails",
        titleColor: "text-green-800",
        items: [
          {
            label: "Gruppenname",
            value: (group: Group) => group.name,
          },
          {
            label: "Raum",
            value: (group: Group) => group.room_name ?? "Kein Raum zugewiesen",
          },
          {
            label: "Aufsichtspersonen",
            value: (group: Group) => {
              if (!group.supervisors || group.supervisors.length === 0) {
                return "Keine Aufsichtspersonen zugewiesen";
              }
              // Return supervisor names as a formatted string
              return group.supervisors
                .map((supervisor) => supervisor.name)
                .join(", ");
            },
          },
          {
            label: "Anzahl Schüler",
            value: (group: Group) => group.student_count?.toString() ?? "0",
          },
          {
            label: "IDs",
            value: (group: Group) => (
              <div className="flex flex-col text-xs text-gray-600">
                <span>Gruppe: {group.id}</span>
                {group.room_id && <span>Raum: {group.room_id}</span>}
                {group.supervisors && group.supervisors.length > 0 && (
                  <span>
                    Aufsichtspersonen:{" "}
                    {group.supervisors.map((s) => s.id).join(", ")}
                  </span>
                )}
              </div>
            ),
          },
        ],
      },
    ],
  },

  list: {
    title: "Gruppe auswählen",
    description: "Verwalte Gruppen und deren Zuordnungen",
    searchPlaceholder: "Gruppe suchen...",

    // Frontend search configuration (loads all data at once)
    searchStrategy: "frontend",
    searchableFields: ["name", "room_name"],
    minSearchLength: 0, // Start searching immediately

    filters: [
      {
        id: "room_id",
        label: "Raum",
        type: "select",
        options: "dynamic", // Will extract from data
      },
    ],

    item: {
      title: (group: Group) => group.name,
      subtitle: (group: Group) => {
        const supervisorCount = group.supervisors?.length ?? 0;
        return supervisorCount > 0
          ? `${supervisorCount} Aufsichtsperson${supervisorCount === 1 ? "" : "en"}`
          : "Keine Aufsichtspersonen";
      },
      description: (group: Group) => {
        const parts = [];
        if (group.room_name) parts.push(`Raum: ${group.room_name}`);
        if (group.student_count !== undefined)
          parts.push(`${group.student_count} Schüler`);
        return parts.join(" • ");
      },
      avatar: {
        text: (group: Group) => group.name?.[0] ?? "G",
      },
      badges: [
        {
          label: (group: Group) => group.room_name ?? "Kein Raum",
          color: "bg-blue-100 text-blue-700",
          showWhen: (group: Group) => !!group.room_name,
        },
        {
          label: (group: Group) => `${group.student_count ?? 0} Schüler`,
          color: "bg-green-100 text-green-700",
          showWhen: () => true,
        },
      ],
    },
  },

  service: {
    mapResponse: (data: unknown): Group => {
      // Handle wrapped response format (for consistency)
      let actualData = data;
      if (
        data &&
        typeof data === "object" &&
        "status" in data &&
        "data" in data
      ) {
        actualData = (data as { status: string; data: unknown }).data;
      }

      return mapGroupResponse(actualData as BackendGroup);
    },

    mapRequest: (data: Partial<Group>) => {
      const mapped: Record<string, unknown> = {
        ...data,
        // Backend expects these as numbers, frontend stores as strings
        room_id: data.room_id ? parseInt(data.room_id) : undefined,
        teacher_ids: data.teacher_ids?.map((id) => parseInt(id)) ?? undefined,
      };

      return mapped;
    },
  },

  labels: {
    createButton: "Neue Gruppe erstellen",
    createModalTitle: "Neue Gruppe",
    editModalTitle: "Gruppe bearbeiten",
    detailModalTitle: "Gruppendetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie diese Gruppe löschen möchten?",
  },
});
