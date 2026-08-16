import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { ParentMealPlanPage } from "./parent-meal-plan-page";
import type { Child, ChildFeatures, MealPlanEntry } from "~/lib/parent-api";

const baseFeatures = {
  sick_note_enabled: true,
  notes_enabled: true,
  excused_requires_approval: false,
  request_submit_enabled: true,
  pickup_change_enabled: true,
  guardian_contact_manage_allowed: true,
  related_accounts_invite_enabled: true,
  related_accounts_remove_enabled: true,
  master_data_edit_enabled: true,
  master_data_contact_edit_enabled: true,
  master_data_request_enabled: true,
  meal_plan_enabled: true,
  has_open_change_request: false,
  parent_news_enabled: true,
} satisfies ChildFeatures;

const childrenWithMealPlans: Child[] = [
  {
    student_id: "child-1",
    tenant_id: "school-1",
    first_name: "Mia",
    last_name: "Schmidt",
    school_class: "3b",
    status: "active",
    enrolled_from: "2025-08-01",
    school_name: "OGS Am Berg",
    school_slug: "am-berg",
  },
  {
    student_id: "child-2",
    tenant_id: "school-2",
    first_name: "Ben",
    last_name: "Schmidt",
    school_class: "1a",
    status: "active",
    enrolled_from: "2025-08-01",
    school_name: "OGS Sonnenhof",
    school_slug: "sonnenhof",
  },
];

function toISODate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function mondayISOFromOffset(weekOffset: number): string {
  const date = new Date();
  date.setHours(12, 0, 0, 0);
  date.setDate(date.getDate() + weekOffset * 7);
  const offset = (date.getDay() + 6) % 7;
  date.setDate(date.getDate() - offset);
  return toISODate(date);
}

function dateFromMonday(mondayISO: string, dayOffset: number): string {
  const parts = mondayISO.split("-");
  const year = Number(parts[0]);
  const month = Number(parts[1]);
  const day = Number(parts[2]);
  const date = new Date(year, month - 1, day, 12);
  date.setDate(date.getDate() + dayOffset);
  return toISODate(date);
}

function mealPlanForWeek(mondayISO: string): MealPlanEntry[] {
  return [
    {
      date: dateFromMonday(mondayISO, 0),
      position: 1,
      dish: "Gemüse-Lasagne",
      note: "mit Salat",
    },
    {
      date: dateFromMonday(mondayISO, 1),
      position: 1,
      dish: "Kartoffel-Gemüse-Eintopf",
    },
    {
      date: dateFromMonday(mondayISO, 2),
      position: 1,
      dish: "Hähnchen-Reis-Pfanne",
      note: "vegetarische Alternative verfügbar",
    },
    {
      date: dateFromMonday(mondayISO, 3),
      position: 1,
      dish: "Nudeln mit Tomatensoße",
    },
    {
      date: dateFromMonday(mondayISO, 4),
      position: 1,
      dish: "Fischstäbchen mit Kartoffelpüree",
      note: "Obst zum Nachtisch",
    },
  ];
}

function mockParentMealPlan({
  childrenData,
  enabledStudentIds = new Set(childrenData.map((child) => child.student_id)),
  entriesByStudent = {},
  shouldFail = false,
}: {
  childrenData: Child[];
  enabledStudentIds?: ReadonlySet<string>;
  entriesByStudent?: Record<string, MealPlanEntry[]>;
  shouldFail?: boolean;
}) {
  return installStorybookFetch(({ url }) => {
    const requestUrl = new URL(url, "http://storybook.test");
    const { pathname } = requestUrl;

    if (pathname === "/api/parent/me/children") {
      if (shouldFail) {
        return jsonResponse(
          { error: "Interner Serverfehler" },
          { status: 500 },
        );
      }
      return jsonResponse({ data: childrenData });
    }

    const childMatch = pathname.match(
      /^\/api\/parent\/me\/children\/([^/]+)\/(features|meal-plan)$/,
    );
    if (!childMatch) return undefined;

    const studentId = decodeURIComponent(childMatch[1] ?? "");
    const resource = childMatch[2];

    if (resource === "features") {
      return jsonResponse({
        data: {
          ...baseFeatures,
          meal_plan_enabled: enabledStudentIds.has(studentId),
        },
      });
    }

    return jsonResponse({ data: entriesByStudent[studentId] ?? [] });
  });
}

const meta = {
  title: "components/parent/ParentMealPlanPage",
  component: ParentMealPlanPage,
  parameters: {
    docs: {
      description: {
        component:
          "Loads the parent's children, per-child feature flags, and the selected school's weekly meal plan via mocked parent API responses.",
      },
    },
  },
} satisfies Meta<typeof ParentMealPlanPage>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Populated: Story = {
  beforeEach: () =>
    mockParentMealPlan({
      childrenData: childrenWithMealPlans,
      entriesByStudent: {
        "child-1": mealPlanForWeek(mondayISOFromOffset(0)),
        "child-2": mealPlanForWeek(mondayISOFromOffset(0)),
      },
    }),
};

export const EmptyWeek: Story = {
  beforeEach: () =>
    mockParentMealPlan({
      childrenData: [childrenWithMealPlans[0]!],
      entriesByStudent: { "child-1": [] },
    }),
};

export const NoEnabledSchools: Story = {
  beforeEach: () =>
    mockParentMealPlan({
      childrenData: [childrenWithMealPlans[0]!],
      enabledStudentIds: new Set(),
    }),
};

export const LoadError: Story = {
  beforeEach: () =>
    mockParentMealPlan({
      childrenData: [],
      shouldFail: true,
    }),
};
