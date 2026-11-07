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

// Errors and success messages run through the real display path (#2515).
const render = (ui: ReactElement) =>
  renderComponent(ui, { wrapper: ToastProvider });

import type { AdminRequestChild } from "~/lib/enrollment-admin-api";
import { useCareOfferingsEnabled } from "~/lib/tenant-context";

const mocks = vi.hoisted(() => ({
  getAdminRequest: vi.fn(),
  getAdminRequestMails: vi.fn(),
  decideAdminChild: vi.fn(),
  correctAdminChildData: vi.fn(),
  listAdminChildOfferingAdjustments: vi.fn(),
  restoreAdminRequest: vi.fn(),
  routerPush: vi.fn(),
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
