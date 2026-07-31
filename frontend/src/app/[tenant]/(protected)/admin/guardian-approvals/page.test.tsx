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
  default: ({ inviteMode }: { inviteMode: string }) => (
    <div data-testid="approval-queue">{inviteMode}</div>
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

  it("waits for the invite mode before rendering the queue", () => {
    setSettingsResult({ data: undefined, isLoading: true });

    render(<GuardianApprovalsPage />);

    expect(
      screen.getByLabelText("Einladungs-Einstellung wird geladen…"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("approval-queue")).not.toBeInTheDocument();
  });

  it("passes a valid invite mode to the queue", () => {
    setSettingsResult({ data: schemaWithInviteMode("disabled") });

    render(<GuardianApprovalsPage />);

    expect(screen.getByTestId("approval-queue")).toHaveTextContent("disabled");
  });

  it("shows a retryable error when settings access returns no schema", () => {
    setSettingsResult({ data: null });

    render(<GuardianApprovalsPage />);

    expect(
      screen.getByText(/Einladungs-Einstellung konnte nicht geladen werden/),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("approval-queue")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    expect(mocks.mutate).toHaveBeenCalledOnce();
  });

  it("shows an error when the schema request fails", () => {
    setSettingsResult({
      data: undefined,
      error: new Error("request failed"),
    });

    render(<GuardianApprovalsPage />);

    expect(
      screen.getByText(/Einladungs-Einstellung konnte nicht geladen werden/),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("approval-queue")).not.toBeInTheDocument();
  });

  it("shows an error when the required setting is missing", () => {
    setSettingsResult({ data: schemaWithInviteMode() });

    render(<GuardianApprovalsPage />);

    expect(
      screen.getByText(/Einladungs-Einstellung konnte nicht geladen werden/),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("approval-queue")).not.toBeInTheDocument();
  });
});
