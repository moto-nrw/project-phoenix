/**
 * Tests for Activities Configuration
 * Tests emoji matching, config structure, and form sections
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { activitiesConfig } from "./activities.config";
import type { Activity, ActivitySupervisor } from "@/lib/activity-helpers";

// Mock next-auth
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() =>
    Promise.resolve({
      user: { token: "mock-token" },
    }),
  ),
}));

// Mock global fetch
global.fetch = vi.fn();

describe("activitiesConfig", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("exports a valid entity config", () => {
    expect(activitiesConfig).toBeDefined();
    expect(activitiesConfig.name).toEqual({
      singular: "Aktivität",
      plural: "Aktivitäten",
    });
  });

  it("has correct API configuration", () => {
    expect(activitiesConfig.api.basePath).toBe("/api/activities");
  });

  it("has form sections configured", () => {
    expect(activitiesConfig.form.sections).toHaveLength(2);
    expect(activitiesConfig.form.sections[0]?.title).toBe("Grundinformationen");
    expect(activitiesConfig.form.sections[1]?.title).toBe("Betreuer");
  });

  it("has required form fields", () => {
    const fields = activitiesConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("ag_category_id");
    expect(fieldNames).toContain("max_participant");
  });

  it("has supervisor field in Betreuer section", () => {
    const fields = activitiesConfig.form.sections[1]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("supervisor_id");
  });

  it("has default max_participant value", () => {
    expect(activitiesConfig.form.defaultValues?.max_participant).toBe(20);
  });

  it("has no transformBeforeSubmit", () => {
    expect(activitiesConfig.form.transformBeforeSubmit).toBeUndefined();
  });

  it("has detail header configuration", () => {
    const mockActivity = {
      id: "1",
      name: "Fußball AG",
      category_name: "Sport",
      max_participant: 20,
      participant_count: 15,
    } as unknown as Activity;

    expect(activitiesConfig.detail.header?.title(mockActivity)).toBe(
      "Fußball AG",
    );
    expect(activitiesConfig.detail.header?.subtitle?.(mockActivity)).toBe(
      "Sport",
    );
  });

  it("calculates activity emoji for sports", () => {
    const mockActivity = {
      id: "1",
      name: "Fußball AG",
      category_name: "Sport",
      max_participant: 20,
    } as unknown as Activity;

    const emoji = activitiesConfig.detail.header?.avatar?.text(mockActivity);
    expect(emoji).toBe("⚽");
  });

  it("calculates activity emoji for creative activities", () => {
    const mockActivity = {
      id: "1",
      name: "Kunst AG",
      category_name: "Kreativ",
      max_participant: 20,
    } as unknown as Activity;

    const emoji = activitiesConfig.detail.header?.avatar?.text(mockActivity);
    expect(emoji).toBe("🎨");
  });

  it("uses initials as fallback emoji", () => {
    const mockActivity = {
      id: "1",
      name: "Unknown Activity",
      category_name: "Other",
      max_participant: 20,
    } as unknown as Activity;

    const emoji = activitiesConfig.detail.header?.avatar?.text(mockActivity);
    expect(emoji).toBe("UN");
  });

  it("shows participant count badge", () => {
    const mockActivity = {
      id: "1",
      name: "Test AG",
      max_participant: 20,
      participant_count: 15,
    } as unknown as Activity;

    const badges = activitiesConfig.detail.header?.badges ?? [];
    const countBadge = badges.find((b) => typeof b.label === "function");
    expect(countBadge).toBeDefined();
    expect(
      (countBadge!.label as (item: Activity) => string)(mockActivity),
    ).toBe("15/20");
  });

  it("shows full badge when activity is at capacity", () => {
    const mockActivity = {
      id: "1",
      name: "Test AG",
      max_participant: 20,
      participant_count: 20,
    } as unknown as Activity;

    const badges = activitiesConfig.detail.header?.badges ?? [];
    const fullBadge = badges.find((b) => b.label === "Voll");
    expect(fullBadge?.showWhen(mockActivity)).toBe(true);
  });

  it("maps request data correctly", () => {
    const data: Partial<Activity> = {
      name: "Test Activity",
      max_participant: 25,
      ag_category_id: "3",
    };

    const mapped = activitiesConfig.service?.mapRequest?.(data);
    expect(mapped).toEqual({
      name: "Test Activity",
      max_participants: 25,
      category_id: 3,
    });
  });

  it("maps request data with supervisor", () => {
    const data: Partial<Activity> = {
      name: "Test Activity",
      max_participant: 25,
      ag_category_id: "3",
      supervisor_id: "42",
    };

    const mapped = activitiesConfig.service?.mapRequest?.(data);
    expect(mapped).toEqual({
      name: "Test Activity",
      max_participants: 25,
      category_id: 3,
      supervisor_ids: [42],
    });
  });

  it("maps response data correctly", () => {
    const responseData = {
      id: "1",
      name: "Test Activity",
      max_participant: 20,
    };

    const mapped = activitiesConfig.service?.mapResponse?.(responseData);
    expect(mapped).toEqual(responseData);
  });

  it("has list configuration", () => {
    expect(activitiesConfig.list.title).toBe("Aktivität auswählen");
    expect(activitiesConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays supervisor in list item subtitle", () => {
    const mockActivity = {
      id: "1",
      name: "Test AG",
      max_participant: 20,
      supervisors: [
        {
          id: "1",
          staff_id: "1",
          activity_id: "1",
          is_primary: true,
          first_name: "Max",
          last_name: "Mustermann",
        } as ActivitySupervisor,
      ],
    } as unknown as Activity;

    const subtitle = activitiesConfig.list.item.subtitle?.(mockActivity);
    expect(subtitle).toBe("Max Mustermann");
  });

  it("displays no supervisor message when none assigned", () => {
    const mockActivity = {
      id: "1",
      name: "Test AG",
      max_participant: 20,
      supervisors: [],
    } as unknown as Activity;

    const subtitle = activitiesConfig.list.item.subtitle?.(mockActivity);
    expect(subtitle).toBe("Kein Hauptbetreuer");
  });

  it("has custom labels", () => {
    expect(activitiesConfig.labels?.createButton).toBe(
      "Neue Aktivität erstellen",
    );
    expect(activitiesConfig.labels?.deleteConfirmation).toContain("löschen");
  });

  it("does not expose custom detail actions for removed activity subpages", () => {
    expect(activitiesConfig.detail.actions?.custom).toBeUndefined();
  });
});
