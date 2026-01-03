// Activity Entity Configuration

import { defineEntityConfig } from "../types";
import { databaseThemes } from "@/components/ui/database/themes";
import type { Activity, ActivitySupervisor } from "@/lib/activity-helpers";
import { getSession } from "next-auth/react";

// Helper functions for emoji selection

function getSportsEmoji(name?: string, category?: string): string | null {
  if (name?.includes("fußball") || name?.includes("fussball")) return "⚽";
  if (name?.includes("basketball")) return "🏀";
  if (name?.includes("volleyball")) return "🏐";
  if (name?.includes("tennis")) return "🎾";
  if (name?.includes("schwimm")) return "🏊";
  if (name?.includes("lauf") || name?.includes("athletik")) return "🏃";
  if (name?.includes("turnen") || name?.includes("gym")) return "🤸";
  if (name?.includes("sport") || category?.includes("sport")) return "🏃";
  return null;
}

function getCreativeEmoji(name?: string): string | null {
  if (name?.includes("kunst") || name?.includes("mal") || name?.includes("zeich"))
    return "🎨";
  if (name?.includes("musik") || name?.includes("chor") || name?.includes("band"))
    return "🎵";
  if (name?.includes("theater") || name?.includes("drama")) return "🎭";
  if (name?.includes("tanz") || name?.includes("dance")) return "💃";
  if (name?.includes("foto") || name?.includes("photo")) return "📸";
  if (name?.includes("film") || name?.includes("video")) return "🎬";
  return null;
}

function getAcademicEmoji(name?: string): string | null {
  if (name?.includes("mathematik") || name?.includes("mathe")) return "🔢";
  if (
    name?.includes("physik") ||
    name?.includes("chemie") ||
    name?.includes("labor")
  )
    return "🔬";
  if (name?.includes("biologie") || name?.includes("natur")) return "🌿";
  if (
    name?.includes("computer") ||
    name?.includes("informatik") ||
    name?.includes("coding")
  )
    return "💻";
  if (name?.includes("robotik") || name?.includes("technik")) return "🤖";
  if (
    name?.includes("sprach") ||
    name?.includes("english") ||
    name?.includes("französisch")
  )
    return "🗣️";
  if (
    name?.includes("lesen") ||
    name?.includes("buch") ||
    name?.includes("literatur")
  )
    return "📚";
  if (name?.includes("schreib") || name?.includes("journal")) return "✍️";
  return null;
}

function getPracticalEmoji(name?: string): string | null {
  if (
    name?.includes("koch") ||
    name?.includes("küche") ||
    name?.includes("back")
  )
    return "🍳";
  if (name?.includes("garten") || name?.includes("pflanzen")) return "🌱";
  if (
    name?.includes("werk") ||
    name?.includes("holz") ||
    name?.includes("handwerk")
  )
    return "🔨";
  if (
    name?.includes("näh") ||
    name?.includes("textil") ||
    name?.includes("schneid")
  )
    return "🧵";
  return null;
}

function getGamesAndOtherEmoji(name?: string, category?: string): string | null {
  // Games
  if (name?.includes("schach")) return "♟️";
  if (name?.includes("spiel") || name?.includes("game")) return "🎲";
  if (name?.includes("puzzle") || name?.includes("rätsel")) return "🧩";

  // Other activities
  if (
    name?.includes("meditation") ||
    name?.includes("yoga") ||
    name?.includes("entspann")
  )
    return "🧘";
  if (name?.includes("erste hilfe") || name?.includes("sanitäter")) return "🚑";
  if (
    name?.includes("umwelt") ||
    name?.includes("recycl") ||
    name?.includes("nachhaltig")
  )
    return "♻️";
  if (name?.includes("feuer") || name?.includes("pfadfinder")) return "🔥";

  // Meal-related
  if (
    name?.includes("mensa") ||
    name?.includes("essen") ||
    name?.includes("mittag") ||
    category?.includes("mensa")
  )
    return "🍽️";

  return null;
}

function getCategoryEmoji(category?: string): string | null {
  if (category?.includes("draußen")) return "🌳";
  if (category?.includes("gruppenraum")) return "🏠";
  if (category?.includes("lernen")) return "📖";
  if (category?.includes("kreativ")) return "🎨";
  if (category?.includes("hausaufgaben")) return "📝";
  return null;
}

function getActivityEmoji(activity: Activity): string {
  const name = activity.name?.toLowerCase();
  const category = activity.category_name?.toLowerCase();

  return (
    getSportsEmoji(name, category) ??
    getCreativeEmoji(name) ??
    getAcademicEmoji(name) ??
    getPracticalEmoji(name) ??
    getGamesAndOtherEmoji(name, category) ??
    getCategoryEmoji(category) ??
    (activity.name ? activity.name.substring(0, 2).toUpperCase() : "AG")
  );
}

export const activitiesConfig = defineEntityConfig<Activity>({
  name: {
    singular: "Aktivität",
    plural: "Aktivitäten",
  },

  theme: databaseThemes.activities,

  backUrl: "/database",

  api: {
    basePath: "/api/activities",
  },

  form: {
    sections: [
      {
        title: "Grundinformationen",
        backgroundColor: "bg-red-50/30",
        iconPath:
          "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
        columns: 2,
        fields: [
          {
            name: "name",
            label: "Name",
            type: "text",
            required: true,
            placeholder: "z.B. Fußball AG",
            colSpan: 2,
          },
          {
            name: "ag_category_id",
            label: "Kategorie",
            type: "select",
            required: true,
            options: async () => {
              // Fetch categories from API
              const response = await fetch("/api/activities/categories", {
                headers: {
                  Authorization: `Bearer ${(await getSession())?.user?.token}`,
                },
              });
              const result = (await response.json()) as
                | { data?: Array<{ id: number; name: string }> }
                | Array<{ id: number; name: string }>;

              // Handle both wrapped and direct response formats
              let categories: Array<{ id: number; name: string }>;
              if (Array.isArray(result)) {
                categories = result;
              } else if (
                result &&
                typeof result === "object" &&
                "data" in result
              ) {
                categories = result.data ?? [];
              } else {
                categories = [];
              }
              return categories.map((cat) => ({
                value: cat.id.toString(),
                label: cat.name,
              }));
            },
          },
          {
            name: "max_participant",
            label: "Maximale Teilnehmer",
            type: "number",
            required: true,
            min: 1,
            placeholder: "20",
          },
        ],
      },
    ],

    defaultValues: {
      max_participant: 20,
    },

    transformBeforeSubmit: (data: Partial<Activity>) => {
      // Return the data as is - the service.mapRequest will handle the transformation
      return data;
    },
  },

  detail: {
    header: {
      title: (activity) => activity.name,
      subtitle: (activity) => activity.category_name ?? "Keine Kategorie",
      avatar: {
        text: getActivityEmoji,
        size: "lg",
      },
      badges: [
        {
          label: (activity) =>
            `${activity.participant_count ?? 0}/${activity.max_participant}`,
          color: "bg-blue-100 text-blue-700",
          showWhen: () => true,
        },
        {
          label: "Voll",
          color: "bg-red-100 text-red-700",
          showWhen: (activity: Activity) =>
            (activity.participant_count ?? 0) >= activity.max_participant,
        },
      ],
    },

    sections: [
      {
        title: "Grundinformationen",
        titleColor: "text-red-800",
        items: [
          {
            label: "Name",
            value: (activity) => activity.name,
          },
          {
            label: "Kategorie",
            value: (activity) => activity.category_name ?? "Keine Kategorie",
          },
          {
            label: "Beschreibung",
            value: (_activity) => "Keine Beschreibung",
          },
          {
            label: "Maximale Teilnehmer",
            value: (activity: Activity) => activity.max_participant.toString(),
          },
          {
            label: "Aktuelle Teilnehmer",
            value: (activity: Activity) =>
              String(activity.participant_count ?? 0),
          },
        ],
      },
      {
        title: "Betreuer",
        titleColor: "text-purple-800",
        items: [
          {
            label: "Hauptbetreuer",
            value: (activity: Activity) => {
              const primary = activity.supervisors?.find(
                (s: ActivitySupervisor) => s.is_primary,
              );
              return primary
                ? `${primary.first_name ?? ""} ${primary.last_name ?? ""}`.trim()
                : "Kein Hauptbetreuer zugewiesen";
            },
          },
          {
            label: "Weitere Betreuer",
            value: (activity: Activity) => {
              const secondary =
                activity.supervisors?.filter(
                  (s: ActivitySupervisor) => !s.is_primary,
                ) ?? [];
              return secondary.length > 0
                ? secondary
                    .map((s: ActivitySupervisor) =>
                      `${s.first_name ?? ""} ${s.last_name ?? ""}`.trim(),
                    )
                    .join(", ")
                : "Keine weiteren Betreuer";
            },
          },
        ],
      },
    ],

    actions: {
      edit: true,
      delete: true,
      custom: [
        {
          label: "Schüler verwalten",
          onClick: (activity) => {
            globalThis.location.href = `/database/activities/${activity.id}/students`;
          },
          color: "blue",
        },
        {
          label: "Zeiten verwalten",
          onClick: (activity) => {
            globalThis.location.href = `/database/activities/${activity.id}/times`;
          },
          color: "green",
        },
      ],
    },
  },

  list: {
    title: "Aktivität auswählen",
    description: "Verwalte Aktivitäten und deren Teilnehmer",
    searchPlaceholder: "Aktivität suchen...",

    // Frontend search for better UX
    searchStrategy: "frontend",
    searchableFields: ["name", "category_name", "description"],
    minSearchLength: 0,

    filters: [
      {
        id: "ag_category_id",
        label: "Kategorie",
        type: "select",
        options: "dynamic", // Will be extracted from the loaded data
      },
      {
        id: "supervisor_id",
        label: "Betreuer",
        type: "select",
        loadOptions: async () => {
          try {
            // Fetch supervisors from API
            const session = await getSession();
            const response = await fetch("/api/activities/supervisors", {
              headers: {
                Authorization: `Bearer ${session?.user?.token}`,
              },
            });

            if (!response.ok) {
              console.error("Failed to fetch supervisors:", response.status);
              return [];
            }

            const result = (await response.json()) as
              | { data?: Array<{ id: string; name: string }> }
              | Array<{ id: string; name: string }>;

            // Handle wrapped response from route wrapper
            let supervisors: Array<{ id: string; name: string }> = [];
            if (Array.isArray(result)) {
              // Direct array response
              supervisors = result;
            } else if (
              result &&
              typeof result === "object" &&
              "data" in result
            ) {
              // Response is wrapped in ApiResponse format
              supervisors = result.data ?? [];
            }

            return supervisors.map((sup) => ({
              value: sup.id,
              label: sup.name,
            }));
          } catch (error) {
            console.error("Error loading supervisors:", error);
            return [];
          }
        },
      },
    ],

    item: {
      title: (activity: Activity) => activity.name,
      subtitle: (activity: Activity) => {
        const primary = activity.supervisors?.find(
          (s: ActivitySupervisor) => s.is_primary,
        );
        return primary
          ? `${primary.first_name ?? ""} ${primary.last_name ?? ""}`.trim()
          : "Kein Hauptbetreuer";
      },
      description: (activity: Activity) => {
        if (activity.is_open_ags) {
          return "Anmeldung offen";
        }
        return "Anmeldung geschlossen";
      },
      avatar: {
        text: getActivityEmoji,
      },
      badges: [
        {
          label: (activity: Activity) =>
            activity.category_name ?? "Keine Kategorie",
          color: "bg-purple-100 text-purple-700",
          showWhen: () => true,
        },
        {
          label: (activity: Activity) =>
            `${activity.participant_count ?? 0}/${activity.max_participant}`,
          color: "bg-blue-100 text-blue-700",
          showWhen: () => true,
        },
        {
          label: "Voll",
          color: "bg-red-100 text-red-700",
          showWhen: (activity: Activity) =>
            (activity.participant_count ?? 0) >= activity.max_participant,
        },
      ],
    },
  },

  service: {
    mapRequest: (data: Partial<Activity>): Record<string, unknown> => {
      // Convert frontend Activity to backend format
      // Only send name, category_id, and max_participants
      return {
        name: data.name,
        max_participants: data.max_participant,
        category_id: data.ag_category_id
          ? Number.parseInt(data.ag_category_id, 10)
          : undefined,
      };
    },

    mapResponse: (responseData: unknown): Activity => {
      // Simply cast the response data to Activity
      // Supervisor data is preserved for display in detail view but not used for editing
      return responseData as Activity;
    },
  },

  labels: {
    createButton: "Neue Aktivität erstellen",
    createModalTitle: "Neue Aktivität",
    editModalTitle: "Aktivität bearbeiten",
    detailModalTitle: "Aktivitätsdetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie diese Aktivität löschen möchten?",
  },
});
