import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { ParentMessagesCard } from "./parent-messages-card";

const mutate = vi.fn(async () => undefined);
const swrResult: { data: unknown; error: unknown } = {
  data: undefined,
  error: undefined,
};

vi.mock("swr", () => ({
  default: () => ({ ...swrResult, mutate }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => "demo",
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: () => undefined,
}));
vi.mock("~/components/messaging/new-message-modal", () => ({
  NewMessageModal: () => null,
}));

describe("ParentMessagesCard", () => {
  it("shows a failed load in place instead of an empty list", async () => {
    swrResult.error = new ApiError("down", 503, {
      code: "general.unavailable",
    });
    render(<ParentMessagesCard studentId="7" />);

    expect(
      await screen.findByText(
        "Die Liste der Nachrichten ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Noch keine Unterhaltungen/),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledOnce();
  });
});
