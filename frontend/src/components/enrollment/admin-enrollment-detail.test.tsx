import {
  act,
  fireEvent,
  render as renderComponent,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { releaseFakeTimers, setTestClock } from "~/test/clock";

// Errors and success messages run through the real display path (#2515).
const render = (ui: ReactElement) =>
  renderComponent(ui, { wrapper: ToastProvider });

import type { AdminRequestChild } from "~/lib/enrollment-admin-api";
import type { CareOffering } from "~/lib/care-offering-api";
import { useCareOfferingsEnabled } from "~/lib/tenant-context";

const mocks = vi.hoisted(() => ({
  getAdminRequest: vi.fn(),
  getAdminRequestMails: vi.fn(),
  decideAdminChild: vi.fn(),
  correctAdminChildData: vi.fn(),
  listAdminChildOfferingAdjustments: vi.fn(),
  restoreAdminRequest: vi.fn(),
  routerPush: vi.fn(),
  listCareOfferings: vi.fn(),
  fetchCareOfferingBookingStats: vi.fn(),
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/care-offering-booking-stats")
  >()),
  fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.routerPush }),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    getAdminRequest: mocks.getAdminRequest,
    getAdminRequestMails: mocks.getAdminRequestMails,
    decideAdminChild: mocks.decideAdminChild,
    correctAdminChildData: mocks.correctAdminChildData,
    listAdminChildOfferingAdjustments: mocks.listAdminChildOfferingAdjustments,
    restoreAdminRequest: mocks.restoreAdminRequest,
  };
});

import { AdminEnrollmentDetail } from "./admin-enrollment-detail";

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

beforeEach(() => {
  mocks.getAdminRequest.mockReset();
  mocks.getAdminRequestMails.mockReset().mockResolvedValue([]);
  mocks.decideAdminChild.mockReset();
  mocks.correctAdminChildData.mockReset();
  mocks.restoreAdminRequest.mockReset();
  mocks.listAdminChildOfferingAdjustments.mockReset().mockResolvedValue([]);
  mocks.listCareOfferings.mockReset().mockResolvedValue([]);
  mocks.fetchCareOfferingBookingStats.mockReset().mockResolvedValue({});
  vi.mocked(useCareOfferingsEnabled).mockReturnValue(true);
});

describe("AdminEnrollmentDetail mail updates", () => {
  it("reloads decision mails and polls subsequent delivery outcomes", async () => {
    mocks.getAdminRequest.mockResolvedValue({
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.test",
      submitted_at: "2026-08-05T10:00:00Z",
      status_token: "status-token",
      children: [adjustmentChild({ status: "submitted" })],
    });
    mocks.decideAdminChild.mockResolvedValue({});
    mocks.getAdminRequestMails
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([
        { kind: "enrollment_waitlisted", outcome: "queued" },
      ])
      .mockResolvedValue([
        { kind: "enrollment_waitlisted", outcome: "delivered" },
      ]);
    const interval = vi.spyOn(globalThis, "setInterval");
    const { unmount } = render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Warteliste" }));
    expect(await screen.findByText("Wird gesendet")).toBeVisible();
    expect(mocks.getAdminRequestMails).toHaveBeenCalledTimes(2);

    const poll = interval.mock.calls.find((call) => call[1] === 15_000)?.[0];
    expect(poll).toBeTypeOf("function");
    const visibility = vi
      .spyOn(document, "visibilityState", "get")
      .mockReturnValue("visible");
    await act(async () => {
      if (typeof poll === "function") poll();
    });
    expect(await screen.findByText("Zugestellt")).toBeVisible();
    unmount();
    interval.mockRestore();
    visibility.mockRestore();
  });
});

describe("AdminEnrollmentDetail approval without an offering", () => {
  it("requires confirmation when offerings are optional and none is booked", async () => {
    mocks.getAdminRequest.mockResolvedValue({
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      care_offering_selection_mode: "optional",
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.test",
      submitted_at: "2026-08-05T10:00:00Z",
      status_token: "status-token",
      children: [
        {
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-04-15",
          status: "submitted",
          activation_mode: "scheduled",
          offerings: [],
        },
      ],
    });
    mocks.decideAdminChild.mockResolvedValue({
      id: "child-1",
      status: "approved",
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Bestätigen" }));

    expect(mocks.decideAdminChild).not.toHaveBeenCalled();
    expect(
      screen.getByText(
        "Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen.",
      ),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem bestätigen" }),
    );

    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        "approved",
        undefined,
      );
    });
  });

  it("approves directly when care offerings are disabled", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      care_offering_selection_mode: "optional",
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.test",
      submitted_at: "2026-08-05T10:00:00Z",
      status_token: "status-token",
      children: [
        {
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-04-15",
          status: "submitted",
          activation_mode: "scheduled",
          offerings: [],
        },
      ],
    });
    mocks.decideAdminChild.mockResolvedValue({
      id: "child-1",
      status: "approved",
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        "approved",
        undefined,
      );
    });
    expect(
      screen.queryByRole("button", { name: "Trotzdem bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it.each([
    {
      name: "the phase requires an offering",
      mode: "at_least_one",
      offerings: [],
    },
    {
      name: "an optional offering is booked",
      mode: "optional",
      offerings: [
        {
          offering_id: "offering-1",
          offering_name: "Ganztag",
          days_of_week_mode: "fixed",
        },
      ],
    },
  ])("approves directly when $name", async ({ mode, offerings }) => {
    mocks.getAdminRequest.mockResolvedValue({
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      care_offering_selection_mode: mode,
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.test",
      submitted_at: "2026-08-05T10:00:00Z",
      status_token: "status-token",
      children: [
        {
          id: "child-1",
          first_name: "Lina",
          last_name: "Kind",
          date_of_birth: "2018-04-15",
          status: "submitted",
          activation_mode: "scheduled",
          offerings,
        },
      ],
    });
    mocks.decideAdminChild.mockResolvedValue({
      id: "child-1",
      status: "approved",
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        "approved",
        undefined,
      );
    });
    expect(
      screen.queryByText(/Das Kind wird trotzdem in die OGS aufgenommen/),
    ).not.toBeInTheDocument();
  });
});

describe("AdminEnrollmentDetail Kinderkontingent (#3570)", () => {
  const request = {
    id: "request-1",
    phase_id: "phase-1",
    phase_name: "2027/28",
    guardian_first_name: "Mara",
    guardian_last_name: "Beispiel",
    guardian_email: "mara@example.test",
    submitted_at: "2027-08-05T10:00:00Z",
    status_token: "status-token",
  };
  const heldChild = {
    id: "child-1",
    first_name: "Lina",
    last_name: "Kind",
    date_of_birth: "2018-04-15",
    status: "submitted" as const,
    activation_mode: "scheduled",
    review_reason: "child_quota_reached",
  };

  it("marks a renewal the Kinderkontingent held back", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [heldChild],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(
      (await screen.findAllByText("Wegen Kinderkontingent offen")).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByText(/Darum wurde dieses Kind nicht automatisch verlängert/),
    ).toBeVisible();
  });

  it("drops the mark once the enrollment is decided", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...heldChild, status: "waitlisted" as const }],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(await screen.findAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText("Wegen Kinderkontingent offen"),
    ).not.toBeInTheDocument();
  });

  it("shows the Kinderkontingent message when an approval is refused", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...heldChild, review_reason: null }],
    });
    mocks.decideAdminChild.mockRejectedValue(
      new ApiError("child quota reached", 409, {
        code: "students.child_quota_reached",
        details: { booked_places: 50, occupied_places: 50 },
      }),
    );

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Bestätigen" }));

    // Der Kasten steht an der Entscheidung des Kindes, mit den Zahlen aus
    // den Details.
    expect(
      await screen.findByText(
        /Kinderkontingent Ihrer Schule ist voll \(50 von 50 Kindern\)/,
      ),
    ).toBeVisible();
  });
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
  it("resets an open offering editor when the child's grade is corrected", async () => {
    const child = adjustmentChild({
      target_grade_level: 2,
      created_student_id: "student-1",
    });
    const request = {
      id: "request-1",
      phase_id: "phase-1",
      phase_name: "2026/27",
      guardian_first_name: "Mara",
      guardian_last_name: "Beispiel",
      guardian_email: "mara@example.test",
      submitted_at: "2026-08-05T10:00:00Z",
      status_token: "status-token",
      children: [child],
    };
    const offerings: CareOffering[] = [2, 3].map((grade) => ({
      id: `grade-${grade}`,
      phase_id: "phase-1",
      name: `Angebot für Klasse ${grade}`,
      days_of_week_mode: "fixed",
      available_days: ["mon"],
      includes_holiday_care: false,
      includes_lunch: false,
      is_active: true,
      is_required: false,
      sort_order: grade,
      created_at: "2026-06-18T11:15:00Z",
      updated_at: "2026-06-18T11:15:00Z",
      availability_rule: {
        match: "all",
        conditions: [{ source: "grade_level", operator: "in", value: [grade] }],
      },
    }));
    mocks.listCareOfferings.mockResolvedValue(offerings);
    mocks.getAdminRequest
      .mockResolvedValueOnce(request)
      .mockReturnValueOnce(new Promise(() => undefined));
    mocks.correctAdminChildData.mockResolvedValue({
      request: {
        ...request,
        children: [{ ...child, target_grade_level: 3 }],
      },
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    const previousOffering = await screen.findByRole("checkbox", {
      name: "Angebot für Klasse 2",
    });
    fireEvent.click(previousOffering);
    expect(previousOffering).toBeChecked();
    expect(
      screen.getByRole("checkbox", {
        name: "Angebot für Klasse 3 bleibt gebucht",
      }),
    ).toBeDisabled();

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldedaten korrigieren" }),
    );
    fireEvent.change(screen.getByLabelText("Ziel-Klassenstufe"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Grund der Korrektur"), {
      target: { value: "Klassenstufe berichtigt" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));
    expect(
      await screen.findByRole("checkbox", { name: "Angebot für Klasse 3" }),
    ).toBeEnabled();
    const blockedOffering = screen.getByRole("checkbox", {
      name: "Angebot für Klasse 2 bleibt gebucht",
    });
    expect(blockedOffering).toBeDisabled();
    expect(blockedOffering).not.toBeChecked();
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2);
  });

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
      .mockRejectedValueOnce(
        new ApiError("refetch failed", 503, { code: "general.unavailable" }),
      );
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
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
  });
  // The follow-up reload after a correction runs quietly, like the one after
  // an offering change (#5102): the page must not fall back to its skeleton,
  // which would unmount every card while the reload is pending.
  it("keeps the page on screen while reloading after a correction", async () => {
    const child: AdminRequestChild = {
      id: "child-1",
      first_name: "Lina",
      last_name: "Falsch",
      date_of_birth: "2018-04-15",
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
      .mockReturnValueOnce(new Promise(() => undefined));
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

    await waitFor(() => expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2));
    expect(
      screen.getAllByRole("heading", { name: "Lina Richtig" }),
    ).not.toHaveLength(0);
    expect(
      screen.getByRole("button", { name: "Gesamte Anmeldung löschen" }),
    ).toBeVisible();
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
      new ApiError("restore duplicate", 409, {
        code: "enrollment.restore_duplicate",
      }),
    );
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);

    render(<AdminEnrollmentDetail requestId="request-1" />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Anmeldung wiederherstellen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederherstellen" }));

    // Der Fehler steht im noch offenen Dialog.
    expect(
      await screen.findByText(
        catalogText("enrollment.restore_duplicate", "die Anmeldung"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Anmeldung wiederherstellen" }),
    ).toBeInTheDocument();
    // The action stays available for a retry once the conflict is resolved.
    expect(
      screen.getByRole("button", { name: "Anmeldung wiederherstellen" }),
    ).toBeInTheDocument();
  });
});

describe("AdminEnrollmentDetail future renewal (#6043)", () => {
  const request = {
    id: "request-1",
    phase_id: "phase-1",
    phase_name: "Ab Februar",
    guardian_first_name: "Mara",
    guardian_last_name: "Beispiel",
    guardian_email: "mara@example.test",
    submitted_at: "2026-12-01T10:00:00Z",
    status_token: "status-token",
  };
  const approvedChild = {
    id: "child-1",
    first_name: "Lina",
    last_name: "Kind",
    date_of_birth: "2018-04-15",
    status: "approved" as const,
    activation_mode: "scheduled",
    created_student_id: "42",
  };

  it("names the day the approved weekly plan applies", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(
      await screen.findByText(
        "Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung gelten ab 03.02.2027. Bis dahin bleiben die bisherigen.",
      ),
    ).toBeVisible();
  });

  it("says on the start day that the weekly plan applies today", async () => {
    setTestClock("2027-02-03T09:00:00+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(
      await screen.findByText(
        "Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung gelten ab heute. moto übernimmt sie im Lauf des Tages.",
      ),
    ).toBeVisible();
  });

  // A plan the hourly run could not apply stays set after its day; the
  // previous values still apply and the office learns what to do (#6043).
  it("warns that an overdue weekly plan has not applied yet", async () => {
    setTestClock("2027-02-10T09:00:00+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(
      await screen.findByText(
        "Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung sollten ab 03.02.2027 gelten. moto konnte sie noch nicht übernehmen. Darum gelten noch die bisherigen. moto versucht es jede Stunde neu. Bleibt dieser Hinweis, melden Sie sich bitte beim moto-Team.",
      ),
    ).toBeVisible();
    expect(
      screen.queryByText(/gelten ab 03\.02\.2027/),
    ).not.toBeInTheDocument();
  });

  it("shows no hint once the weekly plan applies", async () => {
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [approvedChild],
    });

    render(<AdminEnrollmentDetail requestId="request-1" />);

    expect(await screen.findAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText(/aus dieser Anmeldung gelten ab/),
    ).not.toBeInTheDocument();
  });

  it("refreshes an applied weekly plan when the tab becomes visible", async () => {
    setTestClock("2027-02-03T09:00:00+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest
      .mockResolvedValueOnce({
        ...request,
        children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
      })
      .mockResolvedValue({ ...request, children: [approvedChild] });
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    render(<AdminEnrollmentDetail requestId="request-1" />);
    const notice = await screen.findByText(
      /aus dieser Anmeldung gelten ab heute/,
    );

    fireEvent(document, new Event("visibilitychange"));
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(1);
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    fireEvent(document, new Event("visibilitychange"));

    await waitFor(() => expect(notice).not.toBeInTheDocument());
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2);
  });

  it("checks the pending plan again before showing a warning after midnight", async () => {
    vi.useFakeTimers();
    setTestClock("2027-02-03T23:59:30+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValueOnce({
      ...request,
      children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
    });
    let finishRefresh!: (value: unknown) => void;
    mocks.getAdminRequest.mockImplementation(
      () =>
        new Promise((resolve) => {
          finishRefresh = resolve;
        }),
    );
    await act(async () => {
      render(<AdminEnrollmentDetail requestId="request-1" />);
    });
    expect(
      screen.getByText(/aus dieser Anmeldung gelten ab heute/),
    ).toBeVisible();

    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });

    // Anker: Die Anmeldung bleibt während des erneuten Abrufs geladen.
    expect(screen.getAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText(/moto konnte sie noch nicht übernehmen/),
    ).not.toBeInTheDocument();
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2);
    await act(async () => {
      finishRefresh({ ...request, children: [approvedChild] });
    });
    releaseFakeTimers();

    // Anker: Die Anmeldung bleibt nach der Übernahme geladen.
    expect(screen.getAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText(
        /Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung/,
      ),
    ).not.toBeInTheDocument();
  });

  it("does not turn stale data into a warning when the midnight refresh fails", async () => {
    vi.useFakeTimers();
    setTestClock("2027-02-03T23:59:30+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest
      .mockResolvedValueOnce({
        ...request,
        children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
      })
      .mockRejectedValue(new Error("offline"));
    await act(async () => {
      render(<AdminEnrollmentDetail requestId="request-1" />);
    });
    expect(
      screen.getByText(/aus dieser Anmeldung gelten ab heute/),
    ).toBeVisible();

    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });
    releaseFakeTimers();

    // Anker: Die Anmeldung bleibt trotz des fehlgeschlagenen Abrufs geladen.
    expect(screen.getAllByText("Mara Beispiel")).not.toHaveLength(0);
    expect(
      screen.queryByText(/moto konnte sie noch nicht übernehmen/),
    ).not.toBeInTheDocument();
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2);
  });

  it("warns after midnight when a fresh response still has the pending plan", async () => {
    vi.useFakeTimers();
    setTestClock("2027-02-03T23:59:30+01:00");
    vi.mocked(useCareOfferingsEnabled).mockReturnValue(false);
    mocks.getAdminRequest.mockResolvedValue({
      ...request,
      children: [{ ...approvedChild, weekly_plan_on: "2027-02-03" }],
    });
    await act(async () => {
      render(<AdminEnrollmentDetail requestId="request-1" />);
    });
    expect(
      screen.getByText(/aus dieser Anmeldung gelten ab heute/),
    ).toBeVisible();

    await act(async () => {
      vi.advanceTimersByTime(60_000);
    });
    releaseFakeTimers();

    expect(
      screen.getByText(/moto konnte sie noch nicht übernehmen/),
    ).toBeVisible();
    expect(mocks.getAdminRequest).toHaveBeenCalledTimes(2);
  });
});
