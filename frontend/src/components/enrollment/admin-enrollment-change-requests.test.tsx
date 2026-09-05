import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  approveEnrollmentChangeRequest: vi.fn(),
  askEnrollmentChangeRequestQuestion: vi.fn(),
  getAdminEnrollmentChangeRequest: vi.fn(),
  listAdminEnrollmentChangeRequests: vi.fn(),
  rejectEnrollmentChangeRequest: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    approveEnrollmentChangeRequest: mocks.approveEnrollmentChangeRequest,
    askEnrollmentChangeRequestQuestion:
      mocks.askEnrollmentChangeRequestQuestion,
    getAdminEnrollmentChangeRequest: mocks.getAdminEnrollmentChangeRequest,
    listAdminEnrollmentChangeRequests: mocks.listAdminEnrollmentChangeRequests,
    rejectEnrollmentChangeRequest: mocks.rejectEnrollmentChangeRequest,
  };
});

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => null,
}));

import { AdminEnrollmentChangeRequestDetail } from "./admin-enrollment-change-requests";
import type { AdminEnrollmentChangeRequest } from "~/lib/enrollment-admin-api";

const baseChangeRequest: AdminEnrollmentChangeRequest = {
  id: "42",
  request_id: "7",
  origin: "parent",
  status: "pending_review",
  parent_note: "Bitte korrigieren.",
  admin_decision_note: null,
  base_snapshot: {
    guardian_phone: "+49 221 555 990",
    additional_guardians: [
      {
        first_name: "Coco",
        last_name: "Sommer",
        email: "coco@example.test",
      },
    ],
    custom_data: {
      note: "Alter Hinweis",
      seed_source: "legacy-seed",
    },
    children: [
      {
        id: "99",
        status: "approved",
        first_name: "Lea",
        last_name: "Muster",
        date_of_birth: "2018-04-15",
        target_grade_level: 1,
        custom_data: { allergies: "keine" },
      },
    ],
  },
  proposed_snapshot: {
    guardian_phone: "+49 221 555 991",
    additional_guardians: [
      {
        first_name: "Cocoa",
        last_name: "Sommer",
        email: "cocoa@example.test",
      },
    ],
    custom_data: {
      note: "Neuer Hinweis",
      seed_source: "legacy-seed",
    },
    children: [
      {
        first_name: "Lea-Marie",
        last_name: "Muster",
        date_of_birth: "2018-04-15",
        target_grade_level: 1,
        custom_data: { allergies: "keine" },
      },
    ],
  },
  diff: {
    changed: [
      "additional_guardians",
      "children",
      "custom_data",
      "guardian_phone",
    ],
  },
  created_at: "2026-06-27T10:00:00.000Z",
  updated_at: "2026-06-27T10:00:00.000Z",
  reviewed_at: null,
  reviewed_by_account_id: null,
  request: {
    id: "7",
    phase_id: "5",
    phase_name: "Schuljahr 2026/27",
    care_offering_selection_mode: "optional",
    guardian_first_name: "Mara",
    guardian_last_name: "Muster",
    guardian_email: "mara@example.test",
    guardian_phone: "+49 221 555 990",
    consent_flags: {},
    custom_data: {},
    submitted_at: "2026-06-01T10:00:00.000Z",
    children: [
      {
        id: "99",
        first_name: "Lea",
        last_name: "Muster",
        date_of_birth: "2018-04-15",
        status: "approved",
        activation_mode: "scheduled",
      },
    ],
  },
  messages: [],
};

describe("AdminEnrollmentChangeRequestDetail", () => {
  beforeEach(() => {
    mocks.approveEnrollmentChangeRequest.mockReset();
    mocks.askEnrollmentChangeRequestQuestion.mockReset();
    mocks.getAdminEnrollmentChangeRequest.mockReset();
    mocks.listAdminEnrollmentChangeRequests.mockReset();
    mocks.rejectEnrollmentChangeRequest.mockReset();
  });

  it("renders nested child and custom-data diffs explicitly", async () => {
    mocks.getAdminEnrollmentChangeRequest.mockResolvedValueOnce(
      baseChangeRequest,
    );

    render(<AdminEnrollmentChangeRequestDetail changeRequestId="42" />);

    expect(
      await screen.findByText("Änderungsanfrage prüfen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Kind 1 · Vorname")).toBeInTheDocument();
    expect(screen.getByText("Lea")).toBeInTheDocument();
    expect(screen.getByText("Lea-Marie")).toBeInTheDocument();
    expect(screen.getByText("note")).toBeInTheDocument();
    expect(screen.getByText("Alter Hinweis")).toBeInTheDocument();
    expect(screen.getByText("Neuer Hinweis")).toBeInTheDocument();
    expect(screen.getAllByText("Telefon").length).toBeGreaterThan(0);
    expect(screen.getByText("+49 221 555 991")).toBeInTheDocument();
    expect(screen.getByText("Weitere Person 1 · Vorname")).toBeInTheDocument();
    expect(screen.getByText("Coco")).toBeInTheDocument();
    expect(screen.getByText("Cocoa")).toBeInTheDocument();
    expect(screen.queryByText("1 Eintrag")).not.toBeInTheDocument();
  });

  it("renders admin corrections as read-only audit entries", async () => {
    mocks.getAdminEnrollmentChangeRequest.mockResolvedValueOnce({
      ...baseChangeRequest,
      origin: "admin",
      status: "approved",
      parent_note: null,
      admin_decision_note: "Klassenstufe nach Rücksprache korrigiert",
    });

    render(<AdminEnrollmentChangeRequestDetail changeRequestId="42" />);

    expect(await screen.findByText("OGS-Korrektur")).toBeInTheDocument();
    expect(
      screen.getByText("Klassenstufe nach Rücksprache korrigiert"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Freigeben" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Nachricht an Eltern")).not.toBeInTheDocument();
  });

  it("refreshes request badges after a decision", async () => {
    mocks.getAdminEnrollmentChangeRequest.mockResolvedValueOnce(
      baseChangeRequest,
    );
    mocks.approveEnrollmentChangeRequest.mockResolvedValueOnce({
      ...baseChangeRequest,
      status: "approved",
    });
    const refreshListener = vi.fn();
    window.addEventListener("change-requests-refresh", refreshListener);

    render(<AdminEnrollmentChangeRequestDetail changeRequestId="42" />);
    await screen.findByText("Änderungsanfrage prüfen");
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Passt." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await waitFor(() =>
      expect(mocks.approveEnrollmentChangeRequest).toHaveBeenCalledWith(
        "42",
        "Passt.",
      ),
    );
    expect(refreshListener).toHaveBeenCalledTimes(1);
    window.removeEventListener("change-requests-refresh", refreshListener);
  });
});
