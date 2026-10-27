import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  refresh: vi.fn(),
  useRequirePermission: vi.fn(),
  useParentInviteMode: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => mocks.useRequirePermission(),
}));

// The invite mode rides on the tenant resolve since #4996.
vi.mock("~/lib/tenant-context", () => ({
  useParentInviteMode: () => mocks.useParentInviteMode(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mocks.refresh }),
}));

vi.mock("~/components/admin/guardian-approval-queue", () => ({
  default: ({
    inviteModeState,
  }: {
    inviteModeState: { status: string; mode?: string; retry?: () => void };
  }) => (
    <div data-testid="approval-queue">
      {inviteModeState.status}:{inviteModeState.mode}
      {inviteModeState.retry ? (
        <button type="button" onClick={inviteModeState.retry}>
          Einstellungen erneut laden
        </button>
      ) : null}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

import GuardianApprovalsPage from "./page";

describe("GuardianApprovalsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequirePermission.mockReturnValue({ isReady: true });
    mocks.useParentInviteMode.mockReturnValue("staff_approval");
  });

  it.each(["disabled", "direct", "staff_approval"])(
    "passes the invite mode %s to the queue",
    (mode) => {
      mocks.useParentInviteMode.mockReturnValue(mode);

      render(<GuardianApprovalsPage />);

      expect(screen.getByTestId("approval-queue")).toHaveTextContent(
        `ready:${mode}`,
      );
    },
  );

  it("shows an error with a reload when the school sent no known mode", () => {
    mocks.useParentInviteMode.mockReturnValue(undefined);

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
    fireEvent.click(
      screen.getByRole("button", { name: "Einstellungen erneut laden" }),
    );
    expect(mocks.refresh).toHaveBeenCalledOnce();
  });
});
