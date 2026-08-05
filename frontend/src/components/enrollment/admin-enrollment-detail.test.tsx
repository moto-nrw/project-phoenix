import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  type AdminRequestChild,
  type AdminRequestSchemaField,
} from "~/lib/enrollment-admin-api";
import type { CareOffering } from "~/lib/care-offering-api";
import { ChildExtraFields } from "./admin-enrollment-detail";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { useCareOfferingsEnabled } from "~/lib/tenant-context";

const mocks = vi.hoisted(() => ({
  getAdminRequest: vi.fn(),
  correctAdminChildData: vi.fn(),
  listCareOfferings: vi.fn(),
  listAdminChildOfferingAdjustments: vi.fn(),
  updateAdminChildOfferings: vi.fn(),
  restoreAdminRequest: vi.fn(),
  routerPush: vi.fn(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.routerPush }),
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    getAdminRequest: mocks.getAdminRequest,
    correctAdminChildData: mocks.correctAdminChildData,
    listAdminChildOfferingAdjustments: mocks.listAdminChildOfferingAdjustments,
    updateAdminChildOfferings: mocks.updateAdminChildOfferings,
    restoreAdminRequest: mocks.restoreAdminRequest,
  };
});

import {
  AdminEnrollmentDetail,
  ChildOfferingAdjustment,
  ChildOfferings,
  formatPlainDate,
} from "./admin-enrollment-detail";

describe("formatPlainDate", () => {
  it("preserves a date-only birthday east of Berlin", () => {
    const originalTZ = process.env.TZ;
    process.env.TZ = "Asia/Tokyo";
    try {
      expect(formatPlainDate("2015-06-15")).toBe("15.06.2015");
    } finally {
      process.env.TZ = originalTZ;
    }
  });
});

function field(
  type: string,
  overrides: Partial<AdminRequestSchemaField> = {},
): AdminRequestSchemaField {
  return {
    key: "student.pickup_status",
    label: "Abholregelung",
    type,
    applies_to_child: true,
    ...overrides,
  };
}

function catalogOffering(overrides: Partial<CareOffering> = {}): CareOffering {
  return {
    id: "offering-1",
    phase_id: "phase-1",
    name: "Ganztag",
    days_of_week_mode: "fixed",
    available_days: ["mon"],
    includes_holiday_care: false,
    includes_lunch: false,
    is_active: true,
    is_required: false,
    sort_order: 1,
    created_at: "2026-06-18T11:15:00Z",
    updated_at: "2026-06-18T11:15:00Z",
    ...overrides,
  };
}

function adjustmentChild(
  overrides: Partial<AdminRequestChild> = {},
): AdminRequestChild {
  return {
    id: "child-1",
    first_name: "Lina",
    last_name: "Kind",
    date_of_birth: "2018-01-01",
    status: "approved",
    activation_mode: "scheduled",
    ...overrides,
  };
}

function renderAdjustment(child: AdminRequestChild = adjustmentChild()) {
  return render(
    <ChildOfferingAdjustment
      requestId="request-1"
      phaseId="phase-1"
      onSaved={vi.fn()}
      child={child}
    />,
  );
}

beforeEach(() => {
  mocks.getAdminRequest.mockReset();
  mocks.correctAdminChildData.mockReset();
  mocks.listCareOfferings.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockReset();
  mocks.updateAdminChildOfferings.mockReset();
  mocks.restoreAdminRequest.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockResolvedValue([]);
  vi.mocked(useCareOfferingsEnabled).mockReturnValue(true);
});

describe("AdminEnrollmentDetail late-invite email warning", () => {
  const request = {
    id: "request-1",
    phase_id: "phase-1",
    phase_name: "2026/27",
    guardian_first_name: "Mara",
    guardian_last_name: "Beispiel",
    guardian_email: "submitted@example.test",
    submitted_at: "2026-08-05T10:00:00Z",
    status_token: "status-token",
    children: [
      {
        id: "child-1",
        first_name: "Lina",
        last_name: "Kind",
        date_of_birth: "2018-04-15",
        status: "submitted" as const,
        activation_mode: "scheduled",
      },
    ],
  };

  it("shows the warning when the submitted email differs from the invite", async () => {
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      late_invite_guardian_email: "invited@example.test",
      late_invite_email_mismatch: true,
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(
      await screen.findByText(
        "Der Nachzügler-Link wurde für invited@example.test erstellt. Im Antrag wurde submitted@example.test angegeben. Der Elternportal-Zugang bleibt mit der eingeladenen Adresse verknüpft.",
      ),
    ).toBeInTheDocument();
  });

  it("hides the warning when both emails match", async () => {
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      late_invite_guardian_email: "submitted@example.test",
      late_invite_email_mismatch: false,
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(await screen.findAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText(/Der Nachzügler-Link wurde für/),
    ).not.toBeInTheDocument();
  });
});

describe("AdminEnrollmentDetail data correction", () => {
  it("keeps the corrected child in local state when the follow-up reload fails", async () => {
    const child: AdminRequestChild = {
      id: "child-1",
      first_name: "Lina",
      last_name: "Falsch",
      date_of_birth: "2018-04-15",
      target_grade_level: 2,
      target_school_class: "2a",
      status: "approved",
      activation_mode: "scheduled",
      created_student_id: "student-1",
    };
    const request = {
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.com",
      submitted_at: "2026-07-14T10:00:00Z",
      status_token: "status-token",
      children: [child],
    };
    const correctedChild = { ...child, last_name: "Richtig" };
    mocks.getAdminRequest
      .mockResolvedValueOnce(request)
      .mockRejectedValueOnce(new Error("Refetch fehlgeschlagen"));
    mocks.correctAdminChildData.mockResolvedValue({
      request: { ...request, children: [correctedChild] },
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldedaten korrigieren" }),
    );
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Richtig" },
    });
    fireEvent.change(screen.getByLabelText("Grund der Korrektur"), {
      target: { value: "Nach Rücksprache berichtigt" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    expect(
      await screen.findAllByRole("heading", { name: "Lina Richtig" }),
    ).not.toHaveLength(0);
    expect(screen.getByText("Refetch fehlgeschlagen")).toBeInTheDocument();
  });
});

describe("AdminEnrollmentDetail restore (#2157)", () => {
  const withdrawnChild: AdminRequestChild = {
    id: "child-1",
    first_name: "Lina",
    last_name: "Kind",
    date_of_birth: "2018-04-15",
    status: "withdrawn",
    activation_mode: "scheduled",
  };
  const withdrawnRequest = {
    id: "request-1",
    phase_id: "phase-1",
    phase_name: "2026/27",
    guardian_first_name: "Mara",
    guardian_last_name: "Beispiel",
    guardian_email: "mara@example.com",
    submitted_at: "2026-07-14T10:00:00Z",
    withdrawn_at: "2026-08-04T09:00:00Z",
    status_token: "status-token",
    children: [withdrawnChild],
  };

  it("restores a withdrawn request after confirmation", async () => {
    mocks.getAdminRequest
      .mockResolvedValueOnce(withdrawnRequest)
      .mockResolvedValueOnce({
        ...withdrawnRequest,
        withdrawn_at: null,
        children: [{ ...withdrawnChild, status: "submitted" }],
      });
    mocks.restoreAdminRequest.mockResolvedValue({
      restored_children: 1,
      waitlisted_children: 0,
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));

    await waitFor(() =>
      expect(mocks.restoreAdminRequest).toHaveBeenCalledWith("request-1"),
    );
    expect(
      await screen.findByText(/Die Anmeldung wurde wiederhergestellt/),
    ).toBeInTheDocument();
    // After the refetch the child is active again, so the action disappears.
    expect(
      screen.queryByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).not.toBeInTheDocument();
  });

  it("reports waitlisted children when the capacity gate demoted them", async () => {
    mocks.getAdminRequest
      .mockResolvedValueOnce(withdrawnRequest)
      .mockResolvedValueOnce({
        ...withdrawnRequest,
        withdrawn_at: null,
        children: [{ ...withdrawnChild, status: "waitlisted" }],
      });
    mocks.restoreAdminRequest.mockResolvedValue({
      restored_children: 1,
      waitlisted_children: 1,
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));

    expect(
      await screen.findByText(
        /1 Kind steht auf der Warteliste, weil ein Betreuungsangebot inzwischen voll ist/,
      ),
    ).toBeInTheDocument();
  });

  it("hides the restore action on a request that is not withdrawn", async () => {
    mocks.getAdminRequest.mockResolvedValueOnce({
      ...withdrawnRequest,
      withdrawn_at: null,
      children: [{ ...withdrawnChild, status: "submitted" }],
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    await screen.findByRole("button", { name: "Gesamte Anmeldung löschen" });
    expect(
      screen.queryByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).not.toBeInTheDocument();
  });

  it("shows the restore action when every child was withdrawn individually", async () => {
    // One-by-one withdraws never stamp withdrawn_at on the request.
    mocks.getAdminRequest.mockResolvedValueOnce({
      ...withdrawnRequest,
      withdrawn_at: null,
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    expect(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).toBeInTheDocument();
  });

  it("shows the restore action when only one of several children is withdrawn", async () => {
    // A partial withdraw leaves siblings active; the restore targets
    // exactly the withdrawn subset, so the action must stay visible.
    mocks.getAdminRequest.mockResolvedValueOnce({
      ...withdrawnRequest,
      withdrawn_at: null,
      children: [
        withdrawnChild,
        {
          ...withdrawnChild,
          id: "child-2",
          first_name: "Ben",
          status: "submitted",
        },
      ],
    });
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    expect(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).toBeInTheDocument();
  });

  it("surfaces the error message when the restore fails", async () => {
    mocks.getAdminRequest.mockResolvedValueOnce(withdrawnRequest);
    mocks.restoreAdminRequest.mockRejectedValue(
      new Error(
        "Für mindestens ein Kind dieser Anmeldung existiert bereits eine andere aktive Anmeldung in dieser Phase. Bitte prüfe die vorhandenen Anmeldungen, bevor du wiederherstellst.",
      ),
    );
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));

    expect(
      await screen.findByText(/bereits eine andere aktive Anmeldung/),
    ).toBeInTheDocument();
    // The action stays available for a retry once the conflict is resolved.
    expect(
      screen.getByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).toBeInTheDocument();
  });
});

describe("formatCustomValue", () => {
  // Regression: weekday_boolean values ({mon: true, tue: false}) used to be
  // filtered out entirely (the object branch only kept non-empty strings), so
  // Abholregelung and Buskind vanished from the admin review page. They must
  // now render the selected days, like the backend export (formatWeekdayBoolean).
  it("renders selected days for a weekday_boolean value", () => {
    const value = {
      mon: true,
      tue: false,
      wed: true,
      thu: false,
      fri: true,
    };
    expect(formatCustomValue(value, field("weekday_boolean"))).toBe(
      "Mo, Mi, Fr",
    );
  });

  it("renders weekday_boolean even when no field metadata is passed", () => {
    const value = { mon: true, tue: true };
    expect(formatCustomValue(value)).toBe("Mo, Di");
  });

  it("returns null when no weekday_boolean day is selected", () => {
    const value = { mon: false, tue: false, wed: false };
    expect(formatCustomValue(value, field("weekday_boolean"))).toBeNull();
  });

  // The reserved student.pickup_status target treats an empty map as a real
  // answer ("Geht alleine nach Hause"), not a missing field. The admin must be
  // able to tell "goes alone every day" apart from an absent field, so the row
  // is rendered with an explicit label instead of being dropped.
  it("renders 'Geht alleine nach Hause' for an all-false pickup_status", () => {
    const value = {
      mon: false,
      tue: false,
      wed: false,
      thu: false,
      fri: false,
    };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Geht alleine nach Hause");
  });

  it("renders 'Geht alleine nach Hause' for an explicit empty pickup_status map", () => {
    expect(
      formatCustomValue(
        {},
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Geht alleine nach Hause");
  });

  it("still renders selected days for pickup_status when days are chosen", () => {
    const value = { mon: true, fri: true };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", { target: "student.pickup_status" }),
      ),
    ).toBe("Mo, Fr");
  });

  // Buskind is also weekday_boolean but has no special empty-map meaning: no
  // bus days means the child simply is not a bus kid, so the row drops out.
  it("returns null for an all-false non-pickup weekday_boolean (Buskind)", () => {
    const value = { mon: false, tue: false };
    expect(
      formatCustomValue(
        value,
        field("weekday_boolean", {
          key: "student.bus",
          target: "student.bus",
          label: "Buskind",
        }),
      ),
    ).toBeNull();
  });

  // #1694: the accompanied ("Mit anderem Kind") mode was added to the parent
  // form but the admin formatter only knew alone/bus/pickup, so a multi_mode
  // answer rendered the raw "accompanied" token to staff.
  it("renders the accompanied mode label for a weekday_multi_mode value", () => {
    const value = { mon: ["accompanied"], wed: ["bus", "accompanied"] };
    expect(formatCustomValue(value, field("weekday_multi_mode"))).toBe(
      "Mo: geht mit anderem Kind; Mi: fährt Bus, geht mit anderem Kind",
    );
  });

  // #1694 regression: a weekday_mode answer of all-accompanied days used to be
  // swallowed by the bus/pickup-only filter and rendered as the "Geht immer
  // alleine" fallback — actively WRONG info (the child never goes alone).
  it("renders accompanied days for a weekday_mode value, not 'Geht immer alleine'", () => {
    const value = { mon: "accompanied", tue: "accompanied", wed: "bus" };
    expect(formatCustomValue(value, field("weekday_mode"))).toBe(
      "Mo: geht mit anderem Kind, Di: geht mit anderem Kind, Mi: fährt Bus",
    );
  });

  it("still renders weekday_schedule time values per day", () => {
    const value = { mon: "07:30", wed: "08:00" };
    const { container } = render(
      <>{formatCustomValue(value, field("weekday_schedule"))}</>,
    );
    expect(container.textContent).toContain("Mo:");
    expect(container.textContent).toContain("07:30");
    expect(container.textContent).toContain("Mi:");
    expect(container.textContent).toContain("08:00");
    // Days without a time must not appear.
    expect(container.textContent).not.toContain("Di:");
  });
});

describe("ChildOfferings", () => {
  it("renders mixed automatic and manual day provenance", () => {
    render(
      <ChildOfferings
        offerings={[
          {
            offering_id: "22",
            offering_name: "Randstunde",
            days_of_week_mode: "parent_choice",
            selected_days: ["mon", "tue", "wed", "thu", "fri"],
            manual_selected_days: ["fri"],
            automatic_selected_days: ["mon", "tue", "wed", "thu"],
            available_days: ["mon", "tue", "wed", "thu", "fri"],
          },
        ]}
      />,
    );

    expect(screen.getByText("Randstunde")).toBeVisible();
    expect(
      screen.getByText(
        "Mo, Di, Mi, Do automatisch mitgebucht; Fr von Eltern gewählt",
      ),
    ).toBeVisible();
  });
});

describe("ChildOfferingAdjustment", () => {
  it("hides mutation controls while retaining adjustment history when disabled", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.listAdminChildOfferingAdjustments.mockResolvedValue([
      {
        id: "adjustment-1",
        request_id: "request-1",
        request_child_id: "child-1",
        student_id: "student-1",
        actor_account_id: "account-1",
        actor_role: "admin",
        actor_name_snapshot: "Ada Admin",
        reason: "Historische Korrektur",
        before: [],
        after: [],
        changed_at: "2026-01-02T00:00:00Z",
      },
    ]);

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
        }}
      />,
    );

    expect(
      await screen.findByText(/Historische Korrektur/, { selector: "li" }),
    ).toBeVisible();
    expect(screen.getByText("Änderungshistorie")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(mocks.listCareOfferings).not.toHaveBeenCalled();
  });

  it("preserves selected source-phase offerings that are absent from the current catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      {
        id: "current-1",
        phase_id: "phase-1",
        name: "Aktuelles Angebot",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue"],
        includes_holiday_care: false,
        includes_lunch: false,
        is_active: true,
        is_required: false,
        sort_order: 1,
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "source-1",
              offering_name: "Quellphase",
              days_of_week_mode: "parent_choice",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon", "tue"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await screen.findByText("Aktuelles Angebot");
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Rollover unverändert gespeichert" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({
          offerings: [{ offering_id: "source-1", selected_days: ["mon"] }],
        }),
      );
    });
  });

  it("removes a selected offering that is unavailable for the child's grade", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      {
        id: "grade-1-only",
        phase_id: "phase-1",
        name: "Randstunde",
        days_of_week_mode: "fixed",
        available_days: ["mon"],
        includes_holiday_care: false,
        includes_lunch: false,
        is_active: true,
        is_required: false,
        sort_order: 1,
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
        created_at: "2026-06-18T11:15:00Z",
        updated_at: "2026-06-18T11:15:00Z",
      },
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          target_grade_level: 3,
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "grade-1-only",
              offering_name: "Randstunde",
              days_of_week_mode: "fixed",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalled());
    expect(screen.queryByText("Randstunde")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Nicht für Klassenstufe 3 verfügbar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({ offerings: [] }),
      );
    });
  });

  it("keeps a selected unavailable offering removed after reopening the cached catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        id: "grade-1-only",
        name: "Randstunde",
        availability_rule: {
          match: "all",
          conditions: [{ source: "grade_level", operator: "in", value: [1] }],
        },
      }),
    ]);
    mocks.updateAdminChildOfferings.mockResolvedValue({});
    renderAdjustment(
      adjustmentChild({
        target_grade_level: 3,
        offerings: [
          {
            offering_id: "grade-1-only",
            offering_name: "Randstunde",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Nicht mehr verfügbar" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        expect.objectContaining({ offerings: [] }),
      );
    });
    expect(mocks.listCareOfferings).toHaveBeenCalledOnce();
  });

  it("reseeds an active required offering after reopening the cached catalog", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({ name: "Verpflichtender Ganztag", is_required: true }),
    ]);
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Verpflichtender Ganztag/,
    });
    expect(checkbox).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(
      screen.getByRole("checkbox", { name: /Verpflichtender Ganztag/ }),
    ).toBeChecked();
    expect(mocks.listCareOfferings).toHaveBeenCalledOnce();
  });

  it("does not newly select an inactive required offering", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        name: "Inaktiver Pflichtplatz",
        is_active: false,
        is_required: true,
      }),
    ]);
    renderAdjustment();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Inaktiver Pflichtplatz/,
    });

    expect(checkbox).toBeDisabled();
    expect(checkbox).not.toBeChecked();
  });

  it("retains an inactive required offering that was already selected", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      catalogOffering({
        name: "Bestehender Pflichtplatz",
        is_active: false,
        is_required: true,
      }),
    ]);
    renderAdjustment(
      adjustmentChild({
        offerings: [
          {
            offering_id: "offering-1",
            offering_name: "Bestehender Pflichtplatz",
            days_of_week_mode: "fixed",
            selected_days: ["mon"],
            manual_selected_days: ["mon"],
            available_days: ["mon"],
          },
        ],
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const checkbox = await screen.findByRole("checkbox", {
      name: /Bestehender Pflichtplatz/,
    });

    expect(checkbox).toBeDisabled();
    expect(checkbox).toBeChecked();
  });

  it("blocks saving when the care-offering catalog failed to load", async () => {
    mocks.listCareOfferings.mockRejectedValue(new Error("Katalog kaputt"));

    render(
      <ChildOfferingAdjustment
        requestId="request-1"
        phaseId="phase-1"
        onSaved={vi.fn()}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-01-01",
          status: "approved",
          activation_mode: "scheduled",
          offerings: [
            {
              offering_id: "offering-1",
              offering_name: "Ganztag",
              days_of_week_mode: "parent_choice",
              selected_days: ["mon"],
              manual_selected_days: ["mon"],
              available_days: ["mon", "tue"],
            },
          ],
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Katalog kaputt",
    );
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(save);
    await waitFor(() => {
      expect(mocks.updateAdminChildOfferings).not.toHaveBeenCalled();
    });
  });
});

describe("ChildExtraFields companion note (#1694)", () => {
  function child(custom: Record<string, unknown>): AdminRequestChild {
    return {
      id: "1",
      first_name: "Max",
      last_name: "Muster",
      date_of_birth: "2018-05-01",
      status: "submitted",
      activation_mode: "immediate",
      custom_data: custom,
    };
  }

  const departureField: AdminRequestSchemaField = {
    key: "student.allowed_departure_modes",
    label: "Heimweg",
    type: "weekday_multi_mode",
    applies_to_child: true,
    target: "student.allowed_departure_modes",
  };

  // The companion note rides on a reserved key (not a schema field), so the
  // schema-field loop never emits it. Staff must still see it before approving,
  // because the backend persists it onto the student on approval.
  it("renders the reserved companion note even without a matching schema field", () => {
    const { container } = render(
      <ChildExtraFields
        child={child({ "student.departure_companion_note": "Geschwisterkind" })}
        schemaFields={[departureField]}
      />,
    );
    expect(container.textContent).toContain("Mit welchem Kind?");
    expect(container.textContent).toContain("Geschwisterkind");
  });

  it("renders nothing when neither schema fields nor a companion note are present", () => {
    const { container } = render(
      <ChildExtraFields child={child({})} schemaFields={[departureField]} />,
    );
    expect(container.firstChild).toBeNull();
  });
});
