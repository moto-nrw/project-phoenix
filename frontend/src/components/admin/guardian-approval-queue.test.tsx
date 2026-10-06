import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import GuardianApprovalQueue from "./guardian-approval-queue";
import type { PendingApproval } from "@/lib/guardian-api";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockToastSuccess, mockToastError, mockShowActionError } = vi.hoisted(
  () => ({
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockShowActionError: vi.fn(),
  }),
);
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
  useApiErrorDisplay: () => ({ show: mockShowActionError }),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onConfirm: () => void;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="confirm-modal">
        <h3>{title}</h3>
        {children}
        <button type="button" onClick={onConfirm} data-testid="confirm-reject">
          Ablehnen
        </button>
        <button type="button" onClick={onClose} data-testid="cancel-reject">
          Abbrechen
        </button>
      </div>
    ) : null,
}));

const mockPush = vi.fn();

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush }),
}));

const mockList = vi.fn();
const mockApprove = vi.fn();
const mockReject = vi.fn();

vi.mock("@/lib/guardian-api", () => ({
  listPendingApprovals: (): unknown => mockList(),
  approveGuardianInvitation: (id: string): unknown => mockApprove(id),
  rejectGuardianInvitation: (id: string): unknown => mockReject(id),
}));

const sampleRequest: PendingApproval = {
  id: "42",
  guardianProfileId: "10",
  guardianName: "Julia Schröder",
  guardianEmail: "julia.schroeder@email.de",
  studentId: "1",
  studentName: "Felix Schneider",
  requestedByEmail: "karin.klein@email.de",
  createdAt: "2026-06-10T08:00:00Z",
  expiresAt: "2026-06-12T08:00:00Z",
  roleUpgrade: false,
};

const staffApprovalMode = {
  status: "ready",
  mode: "staff_approval",
} as const;

describe("GuardianApprovalQueue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([sampleRequest]);
    mockApprove.mockResolvedValue(undefined);
    mockReject.mockResolvedValue(undefined);
  });

  it("renders a pending request with guardian, child and requester", async () => {
    render(<GuardianApprovalQueue inviteModeState={{ status: "loading" }} />);
    await waitFor(() =>
      expect(screen.getByText("Julia Schröder")).toBeInTheDocument(),
    );
    expect(screen.getByText("julia.schroeder@email.de")).toBeInTheDocument();
    expect(screen.getByText(/Felix Schneider/)).toBeInTheDocument();
    expect(screen.getByText(/karin.klein@email.de/)).toBeInTheDocument();
    // A plain new-account request carries no upgrade hint.
    expect(
      screen.queryByText(/Stuft einen bestehenden Kontakt/),
    ).not.toBeInTheDocument();
  });

  it("marks requests that upgrade an existing contact", async () => {
    mockList.mockResolvedValue([{ ...sampleRequest, roleUpgrade: true }]);
    render(<GuardianApprovalQueue inviteModeState={{ status: "loading" }} />);
    await waitFor(() =>
      expect(screen.getByText("Julia Schröder")).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        "Stuft einen bestehenden Kontakt auf vollen Zugriff hoch.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the empty state when there are no requests", async () => {
    mockList.mockResolvedValue([]);
    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);
    await waitFor(() =>
      expect(screen.getByText(/Keine offenen Anfragen/)).toBeInTheDocument(),
    );
    // Nothing is misconfigured, so no settings shortcut is offered.
    expect(
      screen.queryByRole("button", { name: /Einstellungen/ }),
    ).not.toBeInTheDocument();
  });

  it("explains an empty queue when parent invites are disabled", async () => {
    mockList.mockResolvedValue([]);
    const { container } = render(
      <GuardianApprovalQueue
        inviteModeState={{ status: "ready", mode: "disabled" }}
      />,
    );
    await waitFor(() =>
      expect(
        screen.getByText(/Eltern können derzeit niemanden einladen/),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Einstellungen/ }));
    expect(mockPush).toHaveBeenCalledWith("/settings?tab=operations");

    const iconBadge = container.querySelector(
      '[data-testid="approvals-empty-icon"]',
    );
    expect(iconBadge).toHaveAttribute("data-concept", "settings");
    expect(iconBadge).toHaveClass("bg-gray-100");
  });

  it("explains an empty queue when invites are sent without approval", async () => {
    mockList.mockResolvedValue([]);
    render(
      <GuardianApprovalQueue
        inviteModeState={{ status: "ready", mode: "direct" }}
      />,
    );
    await waitFor(() =>
      expect(
        screen.getByText(/Einladungen gehen ohne Freigabe raus/),
      ).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: /Einstellungen/ }),
    ).toBeInTheDocument();
  });

  it("keeps the neutral empty state when approval is the active mode", async () => {
    mockList.mockResolvedValue([]);
    const { container } = render(
      <GuardianApprovalQueue inviteModeState={staffApprovalMode} />,
    );
    await waitFor(() =>
      expect(screen.getByText(/Keine offenen Anfragen/)).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("button", { name: /Einstellungen/ }),
    ).not.toBeInTheDocument();

    const iconBadge = container.querySelector(
      '[data-testid="approvals-empty-icon"]',
    );
    expect(iconBadge).toHaveAttribute("data-concept", "accounts");
    expect(iconBadge).toHaveClass("bg-gray-100");
  });

  it("defers only an empty result while the invite mode is loading", async () => {
    mockList.mockResolvedValue([]);

    render(<GuardianApprovalQueue inviteModeState={{ status: "loading" }} />);

    const loading = await screen.findByLabelText(
      "Einladungs-Einstellung wird geladen…",
    );
    expect(loading).toHaveAttribute("aria-live", "polite");
    expect(mockList).toHaveBeenCalledOnce();
  });

  it("shows a retryable settings error only for an empty result", async () => {
    const retry = vi.fn();
    mockList.mockResolvedValue([]);

    render(
      <GuardianApprovalQueue
        inviteModeState={{ status: "error", isRetrying: false, retry }}
      />,
    );

    expect(
      await screen.findByText(/Einladungs-Einstellung konnte nicht geladen/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  // #2517: a failed settings request shows the catalog text it carries.
  it("shows the catalog text of a failed settings request", async () => {
    mockList.mockResolvedValue([]);

    render(
      <GuardianApprovalQueue
        inviteModeState={{
          status: "error",
          isRetrying: false,
          retry: vi.fn(),
          error: "Die Einladungs-Einstellung ist gerade nicht erreichbar.",
        }}
      />,
    );

    expect(
      await screen.findByText(
        "Die Einladungs-Einstellung ist gerade nicht erreichbar.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a failed load where the list belongs and retries", async () => {
    mockList
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([sampleRequest]);

    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Anfragen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine offenen Anfragen")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Julia Schröder")).toBeInTheDocument();
  });

  it("reports a failed approval on the action path", async () => {
    const failure = new ApiError("nope", 409, {
      code: "general.business_rejection",
    });
    mockApprove.mockRejectedValueOnce(failure);

    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);
    await waitFor(() => screen.getByText("Julia Schröder"));
    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() =>
      expect(mockShowActionError).toHaveBeenCalledWith(failure, {
        object: "die Freigabe der Anfrage",
        retry: expect.any(Function),
      }),
    );
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("keeps a failed rejection in the confirmation dialog", async () => {
    mockReject.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );

    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);
    await waitFor(() => screen.getByText("Julia Schröder"));
    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));
    fireEvent.click(screen.getByTestId("confirm-reject"));

    expect(
      await screen.findByText(
        catalogText("general.server", "das Ablehnen der Anfrage"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("confirm-modal")).toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("approves a request and reloads the list", async () => {
    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);
    await waitFor(() => screen.getByText("Julia Schröder"));

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() => expect(mockApprove).toHaveBeenCalledWith("42"));
    // initial load + reload after approve
    expect(mockList).toHaveBeenCalledTimes(2);
  });

  it("rejects a request via the confirmation modal", async () => {
    render(<GuardianApprovalQueue inviteModeState={staffApprovalMode} />);
    await waitFor(() => screen.getByText("Julia Schröder"));

    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));
    // confirmation modal opens
    expect(screen.getByTestId("confirm-modal")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("confirm-reject"));

    await waitFor(() => expect(mockReject).toHaveBeenCalledWith("42"));
  });
});
