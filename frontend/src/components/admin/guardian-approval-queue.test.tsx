import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import GuardianApprovalQueue from "./guardian-approval-queue";
import type { PendingApproval } from "@/lib/guardian-api";

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({ success: vi.fn(), error: vi.fn() })),
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
};

describe("GuardianApprovalQueue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([sampleRequest]);
    mockApprove.mockResolvedValue(undefined);
    mockReject.mockResolvedValue(undefined);
  });

  it("renders a pending request with guardian, child and requester", async () => {
    render(<GuardianApprovalQueue />);
    await waitFor(() =>
      expect(screen.getByText("Julia Schröder")).toBeInTheDocument(),
    );
    expect(screen.getByText("julia.schroeder@email.de")).toBeInTheDocument();
    expect(screen.getByText(/Felix Schneider/)).toBeInTheDocument();
    expect(screen.getByText(/karin.klein@email.de/)).toBeInTheDocument();
  });

  it("shows the empty state when there are no requests", async () => {
    mockList.mockResolvedValue([]);
    render(<GuardianApprovalQueue />);
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
    render(<GuardianApprovalQueue inviteMode="disabled" />);
    await waitFor(() =>
      expect(
        screen.getByText(/Eltern können derzeit niemanden einladen/),
      ).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Einstellungen/ }));
    expect(mockPush).toHaveBeenCalledWith("/settings?tab=operations");
  });

  it("explains an empty queue when invites are sent without approval", async () => {
    mockList.mockResolvedValue([]);
    render(<GuardianApprovalQueue inviteMode="direct" />);
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
    render(<GuardianApprovalQueue inviteMode="staff_approval" />);
    await waitFor(() =>
      expect(screen.getByText(/Keine offenen Anfragen/)).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole("button", { name: /Einstellungen/ }),
    ).not.toBeInTheDocument();
  });

  it("approves a request and reloads the list", async () => {
    render(<GuardianApprovalQueue />);
    await waitFor(() => screen.getByText("Julia Schröder"));

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() => expect(mockApprove).toHaveBeenCalledWith("42"));
    // initial load + reload after approve
    expect(mockList).toHaveBeenCalledTimes(2);
  });

  it("rejects a request via the confirmation modal", async () => {
    render(<GuardianApprovalQueue />);
    await waitFor(() => screen.getByText("Julia Schröder"));

    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));
    // confirmation modal opens
    expect(screen.getByTestId("confirm-modal")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("confirm-reject"));

    await waitFor(() => expect(mockReject).toHaveBeenCalledWith("42"));
  });
});
