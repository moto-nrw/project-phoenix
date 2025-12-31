// Room Entity Configuration

import { defineEntityConfig } from "../types";
import { databaseThemes } from "@/components/ui/database/themes";
import { mapRoomResponse, prepareRoomForBackend } from "@/lib/room-helpers";
import type { Room, BackendRoom } from "@/lib/room-helpers";

export const roomsConfig = defineEntityConfig<Room>({
  name: {
    singular: "Raum",
    plural: "Räume",
  },

  theme: databaseThemes.rooms,

  backUrl: "/database",

  api: {
    basePath: "/api/rooms",
  },

  form: {
    sections: [
      {
        title: "Raumdetails",
        backgroundColor: "bg-indigo-50/30",
        columns: 2,
        fields: [
          {
            name: "name",
            label: "Raumname",
            type: "text",
            required: true,
            placeholder: "z.B. Klassenraum 101",
          },
          {
            name: "category",
            label: "Kategorie",
            type: "select",
            required: true,
            options: [
              { value: "Normaler Raum", label: "Normaler Raum" },
              { value: "Gruppenraum", label: "Gruppenraum" },
              { value: "Themenraum", label: "Themenraum" },
              { value: "Sport", label: "Sport" },
            ],
          },
          {
            name: "building",
            label: "Gebäude",
            type: "text",
            placeholder: "z.B. Gebäude A",
          },
          {
            name: "floor",
            label: "Etage",
            type: "text",
            required: false, // Now optional
            placeholder: "z.B. 0, 1, 2",
            validation: (value) => {
              if (value && isNaN(parseInt(value as string))) {
                return "Bitte geben Sie eine gültige Etage ein";
              }
              return null;
            },
          },
        ],
      },
    ],

    defaultValues: {
      color: "#4F46E5", // Default color (matches original implementation)
    },

    transformBeforeSubmit: (data) => {
      // Match original implementation:
      // - Trim name (String(form.name).trim())
      // - Convert floor to number (Number(form.floor))
      // - Ensure color has default (form.color ?? "#4F46E5")
      return {
        ...data,
        name: typeof data.name === "string" ? data.name.trim() : data.name,
        floor:
          typeof data.floor === "string"
            ? parseInt(data.floor, 10)
            : data.floor,
        color: data.color ?? "#4F46E5", // Ensure color is always set
      };
    },
  },

  detail: {
    header: {
      title: (room) => room.name,
      subtitle: (room) => {
        if (room.building && room.floor !== undefined) {
          return `${room.building}, Etage ${room.floor}`;
        }
        if (room.floor !== undefined) {
          return `Etage ${room.floor}`;
        }
        if (room.building) {
          return room.building;
        }
        return "";
      },
      avatar: {
        text: (room) => room.name?.[0] ?? "R",
        size: "md",
      },
    },

    sections: [
      {
        title: "Raumdetails",
        titleColor: "text-green-800",
        items: [
          {
            label: "Raumname",
            value: (room: Room) => room.name,
          },
          {
            label: "Kategorie",
            value: (room: Room) => room.category ?? "Nicht angegeben",
          },
          {
            label: "Gebäude",
            value: (room: Room) => room.building ?? "Nicht angegeben",
          },
          {
            label: "Etage",
            value: (room: Room) =>
              room.floor !== undefined
                ? `Etage ${room.floor}`
                : "Nicht angegeben",
          },
          {
            label: "Status",
            value: (room: Room) => (room.isOccupied ? "Belegt" : "Frei"),
            colSpan: 2,
          },
        ],
      },
    ],
  },

  list: {
    title: "Raum auswählen",
    description: "Verwalte Räume und deren Eigenschaften",
    searchPlaceholder: "Raum suchen...",

    // No filters needed for ~20 rooms - search is sufficient

    // Frontend search configuration
    searchStrategy: "frontend",
    searchableFields: ["name", "category", "building"],
    minSearchLength: 0,

    item: {
      title: (room: Room) => room.name,
      subtitle: (room: Room) => {
        // Show occupancy status as subtitle
        if (room.isOccupied) {
          const parts = ["Belegt"];
          if (room.groupName) parts.push(`Gruppe: ${room.groupName}`);
          if (room.activityName) parts.push(room.activityName);
          return parts.join(" • ");
        }
        return "Frei";
      },
      description: (room: Room) => {
        const parts: string[] = [];
        if (room.building) parts.push(`Gebäude ${room.building}`);
        if (room.floor !== undefined) parts.push(`Etage ${room.floor}`);
        return parts.join(" • ");
      },
      avatar: {
        text: (room: Room) => {
          // Use icon based on category
          switch (room.category) {
            case "Normaler Raum":
              return "📚";
            case "Gruppenraum":
              return "👥";
            case "Themenraum":
              return "🎨";
            case "Sport":
              return "🏃";
            default:
              return room.name?.[0] ?? "R";
          }
        },
        backgroundColor: databaseThemes.rooms.primary,
      },
      badges: [
        // Category badge
        {
          label: (room: Room) => room.category ?? "Keine Kategorie",
          color: "bg-indigo-100 text-indigo-800",
          showWhen: (room: Room) => !!room.category,
        },
        // Building and floor badge
        {
          label: (room: Room) => {
            if (room.building && room.floor !== undefined) {
              return `${room.building} - Etage ${room.floor}`;
            }
            if (room.floor !== undefined) {
              return `Etage ${room.floor}`;
            }
            if (room.building) {
              return room.building;
            }
            return "";
          },
          color: "bg-gray-100 text-gray-800",
          showWhen: (room: Room) => room.floor !== undefined || !!room.building,
        },
        // Occupancy status badge
        {
          label: "Belegt",
          color: "bg-red-100 text-red-800",
          showWhen: (room: Room) => room.isOccupied,
        },
        {
          label: "Frei",
          color: "bg-green-100 text-green-800",
          showWhen: (room: Room) => !room.isOccupied,
        },
      ],
    },
  },

  service: {
    mapResponse: (data: unknown) => mapRoomResponse(data as BackendRoom),
    mapRequest: prepareRoomForBackend,
  },

  labels: {
    createButton: "Neuen Raum erstellen",
    createModalTitle: "Neuer Raum",
    editModalTitle: "Raum bearbeiten",
    detailModalTitle: "Raumdetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie diesen Raum löschen möchten?",
  },
});
