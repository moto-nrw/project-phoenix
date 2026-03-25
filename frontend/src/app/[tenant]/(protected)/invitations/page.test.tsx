import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import InvitationsPage from "./page";

const mockReplace = vi.fn();

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({
    replace: mockReplace,
    push: vi.fn(),
    back: vi.fn(),
  }),
}));

describe("InvitationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("redirects to /database/personal", () => {
    render(<InvitationsPage />);
    expect(mockReplace).toHaveBeenCalledWith("/database/personal");
  });

  it("renders nothing (returns null)", () => {
    const { container } = render(<InvitationsPage />);
    expect(container.innerHTML).toBe("");
  });
});
