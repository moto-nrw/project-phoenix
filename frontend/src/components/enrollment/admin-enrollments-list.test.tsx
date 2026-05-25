import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchSettingsSchema: vi.fn(),
  listAdminRequests: vi.fn(),
  listCareOfferings: vi.fn(),
  listPhases: vi.fn(),
  listSchemas: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", () => ({
  listAdminRequests: mocks.listAdminRequests,
}));

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mocks.listPhases,
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    listSchemas: mocks.listSchemas,
  };
});

vi.mock("~/lib/settings-api", () => ({
  fetchSettingsSchema: mocks.fetchSettingsSchema,
}));

import { AdminEnrollmentsList } from "./admin-enrollments-list";
import type { Phase } from "~/lib/enrollment-phase-api";

function phase(overrides: Partial<Phase> = {}): Phase {
  return {
    id: "10",
    name: "Schuljahr 2026/27",
    kind: "school_year",
    service_start_date: "2026-09-01",
    service_end_date: "2027-07-31",
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: null,
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    is_active: true,
    created_at: "2026-01-01T00:00:00.000Z",
    updated_at: "2026-01-01T00:00:00.000Z",
    ...overrides,
  };
}

function settingsSchema(enrollmentEnabled: boolean) {
  return {
    tabs: [
      {
        key: "enrollment",
        label: "Anmeldung",
        categories: [
          {
            key: "general",
            label: "Allgemein",
            items: [
              {
                key: "enrollment.enabled",
                value: enrollmentEnabled,
              },
            ],
          },
        ],
      },
    ],
  };
}

beforeEach(() => {
  mocks.fetchSettingsSchema.mockReset();
  mocks.listAdminRequests.mockReset();
  mocks.listCareOfferings.mockReset();
  mocks.listPhases.mockReset();
  mocks.listSchemas.mockReset();

  mocks.fetchSettingsSchema.mockResolvedValue(settingsSchema(false));
  mocks.listAdminRequests.mockResolvedValue([]);
  mocks.listCareOfferings.mockResolvedValue([]);
  mocks.listPhases.mockResolvedValue([]);
  mocks.listSchemas.mockResolvedValue([]);
});

describe("AdminEnrollmentsList setup guide", () => {
  it("does not count the form step as complete before a phase exists", async () => {
    render(<AdminEnrollmentsList />);

    await waitFor(() => {
      expect(screen.getByText("Online-Anmeldung vorbereiten")).toBeVisible();
    });

    expect(screen.getByText("0 von 5 Schritten erledigt")).toBeVisible();
    expect(screen.getAllByText("Anmeldeformular festlegen")).toHaveLength(2);
    expect(screen.getAllByText("Wartet auf Phase")).toHaveLength(2);
  });

  it("counts the form step once an active phase uses the base form", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);

    render(<AdminEnrollmentsList />);

    await waitFor(() => {
      expect(screen.getByText("Online-Anmeldung vorbereiten")).toBeVisible();
    });

    expect(screen.getByText("2 von 5 Schritten erledigt")).toBeVisible();
    expect(screen.getByText("Basisformular")).toBeVisible();
  });
});
