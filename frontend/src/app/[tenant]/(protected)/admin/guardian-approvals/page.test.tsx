import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  useRequireAdmin: vi.fn(),
  useSettingsSchema: vi.fn(),
}));

vi.mock("~/lib/hooks/use-require-admin", () => ({
  useRequireAdmin: () => mocks.useRequireAdmin(),
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => mocks.useSettingsSchema(),
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

function schemaWithInviteMode(value?: string) {
  return {
    tabs: [
      {
        key: "operations",
        label: "Betrieb",
        categories: [
          {
            key: "guardians",
            label: "Bezugspersonen",
            items:
              value === undefined
                ? []
                : [
                    {
                      key: "guardians.parent_invite_mode",
                      value,
                    },
                  ],
          },
        ],
      },
    ],
  };
}

function setSettingsResult({
  data,
  error,
  isLoading = false,
  isValidating = false,
}: {
  data?: ReturnType<typeof schemaWithInviteMode> | null;
  error?: Error;
  isLoading?: boolean;
  isValidating?: boolean;
}) {
  mocks.useSettingsSchema.mockReturnValue({
    data,
    error,
    isLoading,
    isValidating,
    mutate: mocks.mutate,
  });
}

describe("GuardianApprovalsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useRequireAdmin.mockReturnValue({ isReady: true });
    setSettingsResult({ data: schemaWithInviteMode("staff_approval") });
  });

  it("mounts the queue while the invite mode is loading", () => {
    setSettingsResult({ data: undefined, isLoading: true });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("loading:");
  });

  it("passes a valid invite mode to the queue", () => {
    setSettingsResult({ data: schemaWithInviteMode("disabled") });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent(
      "ready:disabled",
    );
  });

  it("passes an error state when settings access returns no schema", () => {
    setSettingsResult({ data: null });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
    fireEvent.click(
      screen.getByRole("button", { name: "Einstellungen erneut laden" }),
    );
    expect(mocks.mutate).toHaveBeenCalledOnce();
  });

  it("shows an error when the schema request fails", () => {
    setSettingsResult({
      data: undefined,
      error: new Error("request failed"),
    });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
  });

  it("shows an error when the required setting is missing", () => {
    setSettingsResult({ data: schemaWithInviteMode() });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("error:");
  });
});
