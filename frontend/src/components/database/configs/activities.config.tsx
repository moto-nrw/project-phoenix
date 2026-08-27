// Activity Entity Configuration

import { defineEntityConfig } from "@/lib/database/types";
import type { Activity, ActivitySupervisor } from "@/lib/activity-helpers";
import { getSupervisors } from "@/lib/activity-api";
import { getCachedSession } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ActivitiesConfig" });

// Emoji mapping rules: [emoji, ...keywords]
// Rules are checked in order; first match wins
type EmojiRule = readonly [emoji: string, ...keywords: string[]];

const NAME_EMOJI_RULES: readonly EmojiRule[] = [
  // Sports
  ["⚽", "fußball", "fussball"],
  ["🏀", "basketball"],
  ["🏐", "volleyball"],
  ["🎾", "tennis"],
  ["🏊", "schwimm"],
  ["🏃", "lauf", "athletik", "sport"],
  ["🤸", "turnen", "gym"],
  // Creative
  ["🎨", "kunst", "mal", "zeich"],
  ["🎵", "musik", "chor", "band"],
  ["🎭", "theater", "drama"],
  ["💃", "tanz", "dance"],
  ["📸", "foto", "photo"],
  ["🎬", "film", "video"],
  // Academic
  ["🔢", "mathematik", "mathe"],
  ["🔬", "physik", "chemie", "labor"],
  ["🌿", "biologie", "natur"],
  ["💻", "computer", "informatik", "coding"],
  ["🤖", "robotik", "technik"],
  ["🗣️", "sprach", "english", "französisch"],
  ["📚", "lesen", "buch", "literatur"],
  ["✍️", "schreib", "journal"],
  // Practical
  ["🍳", "koch", "küche", "back"],
  ["🌱", "garten", "pflanzen"],
  ["🔨", "werk", "holz", "handwerk"],
  ["🧵", "näh", "textil", "schneid"],
  // Games
  ["♟️", "schach"],
  ["🎲", "spiel", "game"],
  ["🧩", "puzzle", "rätsel"],
  // Other
  ["🧘", "meditation", "yoga", "entspann"],
  ["🚑", "erste hilfe", "sanitäter"],
  ["♻️", "umwelt", "recycl", "nachhaltig"],
  ["🔥", "feuer", "pfadfinder"],
  // Meals
  ["🍽️", "mensa", "essen", "mittag"],
] as const;

const CATEGORY_EMOJI_RULES: readonly EmojiRule[] = [
  ["🏃", "sport"],
  ["🍽️", "mensa"],
  ["🌳", "draußen"],
  ["🏠", "gruppenraum"],
  ["📖", "lernen"],
  ["🎨", "kreativ"],
  ["📝", "hausaufgaben"],
] as const;

function matchEmojiRule(
  text: string | undefined,
  rules: readonly EmojiRule[],
): string | null {
  if (!text) return null;
  for (const [emoji, ...keywords] of rules) {
    if (keywords.some((keyword) => text.includes(keyword))) {
      return emoji;
    }
  }
  return null;
}

function getActivityEmoji(activity: Activity): string {
  const name = activity.name?.toLowerCase();
  const category = activity.category_name?.toLowerCase();

  return (
    matchEmojiRule(name, NAME_EMOJI_RULES) ??
    matchEmojiRule(category, CATEGORY_EMOJI_RULES) ??
    (activity.name ? activity.name.substring(0, 2).toUpperCase() : "AG")
  );
}

export const activitiesConfig = defineEntityConfig<Activity>({
  name: {
    singular: "Aktivität",
    plural: "Aktivitäten",
  },

  concept: "activities",

  backUrl: "/database",

  api: {
    basePath: "/api/activities",
    // Admin view keeps showing system activities (Schulhof Freispiel, WC) —
    // they are rendered greyed-out via isSystemActivity instead of hidden.
    listParams: { include_system: "true" },
  },

  form: {
    sections: [
      {
        title: "Grundinformationen",
        concept: "activities",
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
                  Authorization: `Bearer ${(await getCachedSession())?.user?.token}`,
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
            label: "Maximale Teilnehmer (optional)",
            type: "number",
            required: false,
            min: 1,
            placeholder: "Keine Begrenzung",
          },
        ],
      },
      {
        title: "Betreuer",
        concept: "staff",
        fields: [
          {
            name: "supervisor_id",
            label: "Hauptbetreuer",
            type: "select",
            required: false,
            placeholder: "Kein Hauptbetreuer",
            options: async () => {
              const supervisors = await getSupervisors();
              return supervisors.map((sup) => ({
                value: sup.id,
                label: sup.name,
              }));
            },
          },
        ],
      },
    ],

    defaultValues: {
      max_participant: 20,
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
            activity.max_participant == null
              ? `${activity.participant_count ?? 0} Teilnehmende`
              : `${activity.participant_count ?? 0}/${activity.max_participant}`,
          color: "bg-moto-blue-soft text-moto-blue-strong",
          showWhen: () => true,
        },
        {
          label: "Voll",
          color: "bg-moto-red-soft text-moto-red-strong",
          showWhen: (activity: Activity) =>
            activity.max_participant != null &&
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
            value: (activity: Activity) =>
              activity.max_participant?.toString() ?? "Unbegrenzt",
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
    },
  },

  list: {
    title: "Aktivität auswählen",
    description: "Verwalte Aktivitäten und deren Teilnehmer",
    searchPlaceholder: "Aktivität suchen…",

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
            const session = await getCachedSession();
            const response = await fetch("/api/activities/supervisors", {
              headers: {
                Authorization: `Bearer ${session?.user?.token}`,
              },
            });

            if (!response.ok) {
              logger.error("failed to fetch supervisors", {
                status: response.status,
              });
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
            logger.error("error loading supervisors", { error: String(error) });
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
          color: "bg-moto-purple-soft text-moto-purple-strong",
          showWhen: () => true,
        },
        {
          label: (activity: Activity) =>
            activity.max_participant == null
              ? `${activity.participant_count ?? 0} Teilnehmende`
              : `${activity.participant_count ?? 0}/${activity.max_participant}`,
          color: "bg-moto-blue-soft text-moto-blue-strong",
          showWhen: () => true,
        },
        {
          label: "Voll",
          color: "bg-moto-red-soft text-moto-red-strong",
          showWhen: (activity: Activity) =>
            activity.max_participant != null &&
            (activity.participant_count ?? 0) >= activity.max_participant,
        },
      ],
    },
  },

  service: {
    mapRequest: (data: Partial<Activity>): Record<string, unknown> => {
      const maxParticipants = data.max_participant as unknown;

      // Convert frontend Activity to backend format
      const request: Record<string, unknown> = {
        name: data.name,
        max_participants:
          maxParticipants === "" || maxParticipants == null
            ? null
            : maxParticipants,
        category_id: data.ag_category_id
          ? Number.parseInt(data.ag_category_id, 10)
          : undefined,
      };

      // Backend expects supervisor_ids as an array even for single primary
      if (data.supervisor_id) {
        request.supervisor_ids = [Number.parseInt(data.supervisor_id, 10)];
      }

      return request;
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
    detailModalTitle: "Aktivitätsdetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie diese Aktivität löschen möchten?",
  },
});
