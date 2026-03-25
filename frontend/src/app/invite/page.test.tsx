import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// Hoisted mocks
const { mockGet } = vi.hoisted(() => ({
  mockGet: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: vi.fn(() => ({
    get: mockGet,
  })),
}));

vi.mock("~/components/auth/invitation-page-content", () => ({
  InvitationPageContent: ({ token }: { token: string | null }) => (
    <div data-testid="invitation-content">{token ?? "no-token"}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading">{fullPage ? "Full Page" : "Loading..."}</div>
  ),
}));

import InvitePage from "./page";

describe("InvitePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("passes token from search params to InvitationPageContent", () => {
    mockGet.mockReturnValue("abc123");
    render(<InvitePage />);
    expect(screen.getByTestId("invitation-content")).toHaveTextContent(
      "abc123",
    );
  });

  it("passes null when token param is missing", () => {
    mockGet.mockReturnValue(null);
    render(<InvitePage />);
    expect(screen.getByTestId("invitation-content")).toHaveTextContent(
      "no-token",
    );
  });
});
