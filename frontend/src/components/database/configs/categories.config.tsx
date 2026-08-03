// Activity Category Entity Configuration (#2131)

import { defineEntityConfig } from "@/lib/database/types";
import { databaseThemes } from "@/lib/database/themes";
import { CategoryColorField } from "@/components/ui/database/category-color-field";
import {
  prepareCategoryForBackend,
  type ActivityCategory,
} from "@/lib/activity-helpers";

/** Longest name the backend accepts — mirrors maxCategoryNameLength in Go. */
export const CATEGORY_NAME_MAX_LENGTH = 60;
const CATEGORY_DESCRIPTION_MAX_LENGTH = 255;

export const categoriesConfig = defineEntityConfig<ActivityCategory>({
  name: {
    singular: "Kategorie",
    plural: "Kategorien",
  },

  // Categories are activity Stammdaten; they share the activities theme rather
  // than introducing a second palette.
  theme: databaseThemes.activities,

  backUrl: "/database",

  api: {
    basePath: "/api/activities/categories",
    // The Stammdaten screen is the one place that shows retired categories —
    // it is where they are restored. System categories (Schulhof, WC) stay
    // hidden: they are backend infrastructure and every write on them is
    // refused.
    listParams: { include_archived: "true" },
  },

  form: {
    sections: [
      {
        title: "Kategoriedetails",
        columns: 1,
        fields: [
          {
            name: "name",
            label: "Name",
            type: "text",
            required: true,
            placeholder: "z.B. Essen",
            maxLength: CATEGORY_NAME_MAX_LENGTH,
            validation: (value) => {
              const name = typeof value === "string" ? value.trim() : "";
              if (name.length === 0) return "Name ist erforderlich";
              if (name.length > CATEGORY_NAME_MAX_LENGTH) {
                return `Name darf höchstens ${CATEGORY_NAME_MAX_LENGTH} Zeichen lang sein`;
              }
              return null;
            },
          },
          {
            name: "description",
            label: "Beschreibung",
            type: "textarea",
            placeholder: "Wofür wird diese Kategorie verwendet?",
            maxLength: CATEGORY_DESCRIPTION_MAX_LENGTH,
          },
          {
            name: "color",
            label: "Farbe",
            type: "custom",
            component: CategoryColorField,
          },
        ],
      },
    ],

    transformBeforeSubmit: (data) => ({
      ...data,
      name: typeof data.name === "string" ? data.name.trim() : data.name,
    }),
  },

  detail: {
    header: {
      title: (category) => category.name,
      subtitle: (category) =>
        category.archivedAt ? "Archiviert" : "Aktive Kategorie",
      avatar: {
        text: (category) => category.name?.[0]?.toUpperCase() ?? "K",
        size: "md",
      },
    },
    sections: [
      {
        title: "Kategoriedetails",
        items: [
          { label: "Name", value: (category) => category.name },
          {
            label: "Beschreibung",
            value: (category) => category.description ?? "Nicht angegeben",
          },
          {
            label: "Verwendet in",
            value: (category) =>
              `${category.usageCount ?? 0} Aktivitäten/Terminen`,
          },
          {
            label: "Status",
            value: (category) => (category.archivedAt ? "Archiviert" : "Aktiv"),
          },
        ],
      },
    ],
  },

  list: {
    title: "Kategorie auswählen",
    description: "Aktivitätskategorien der Schule verwalten",
    searchPlaceholder: "Kategorie suchen...",
    searchStrategy: "frontend",
    searchableFields: ["name", "description"],
    minSearchLength: 0,

    item: {
      title: (category: ActivityCategory) => category.name,
      subtitle: (category: ActivityCategory) =>
        `${category.usageCount ?? 0} Aktivitäten/Termine`,
      avatar: {
        text: (category: ActivityCategory) =>
          category.name?.[0]?.toUpperCase() ?? "K",
        backgroundColor: databaseThemes.activities.primary,
      },
    },
  },

  service: {
    mapResponse: (data: unknown) => data as ActivityCategory,
    mapRequest: prepareCategoryForBackend,
  },

  labels: {
    createButton: "Neue Kategorie erstellen",
    createModalTitle: "Neue Kategorie",
    detailModalTitle: "Kategoriedetails",
    deleteConfirmation:
      "Möchtest du diese Kategorie archivieren? Bestehende Termine und Aktivitäten bleiben gültig.",
  },
});
