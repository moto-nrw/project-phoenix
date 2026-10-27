import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  useRequirePermission: vi.fn(),
  useSchoolParentInviteMode: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => mocks.useRequirePermission(),
}));

// The invite mode rides on the signed-in navigation context since #4996.
vi.mock("~/lib/hooks/use-school-switches", () => ({
  useSchoolParentInviteMode: () => mocks.useSchoolParentInviteMode(),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => mocks.mutate,
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

function setInviteMode({
  mode,
  error,
  isLoading = false,
}: {
  mode?: string;
  error?: Error;
  isLoading?: boolean;
}) {
  mocks.useSchoolParentInviteMode.mockReturnValue({ mode, error, isLoading });
}

describe("GuardianApprovalsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequirePermission.mockReturnValue({ isReady: true });
    setInviteMode({ mode: "staff_approval" });
  });

  it("mounts the queue while the invite mode is loading", () => {
    setInviteMode({ isLoading: true });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("loading:");
  });

  it.each(["disabled", "direct", "staff_approval"])(
    "passes the invite mode %s to the queue",
    (mode) => {
      setInviteMode({ mode });

      render(<GuardianApprovalsPage />);

      expect(screen.getByTestId("approval-queue")).toHaveTextContent(
        `ready:${mode}`,
      );
    },
  );

  it("shows an error with a reload when the request fails", () => {
    setInviteMode({ error: new Error("request failed") });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
    fireEvent.click(
      screen.getByRole("button", { name: "Einstellungen erneut laden" }),
    );
    expect(mocks.mutate).toHaveBeenCalledWith("user-context");
  });

  it("shows an error when the school sent no known mode", () => {
    setInviteMode({ mode: undefined });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
  });
});
