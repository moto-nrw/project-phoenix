import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { mockUseSession, mockUpdateSession, mockFetch } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUpdateSession: vi.fn(),
  mockFetch: vi.fn(),
}));

global.fetch = mockFetch;

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>Loading...</div>,
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

import { EmailConfirmContent } from "./email-confirm-content";

/** Sets window.location.hash and returns a cleanup function. */
function setHash(hash: string) {
  window.location.hash = hash;
}

describe("EmailConfirmContent", () => {
  let replaceStateSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockUpdateSession.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      update: mockUpdateSession,
      status: "unauthenticated",
    });
    replaceStateSpy = vi
      .spyOn(window.history, "replaceState")
      .mockImplementation(() => {});
    // Clear hash before each test
    window.location.hash = "";
  });

  afterEach(() => {
    window.location.hash = "";
  });

  it("shows error when no token is provided", async () => {
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
      expect(screen.getByText("Kein Token angegeben.")).toBeInTheDocument();
    });
  });

  it("shows settings link when no token", async () => {
    render(<EmailConfirmContent />);

    await waitFor(() => {
      const settingsLink = screen.getByText("Zu den Einstellungen");
      expect(settingsLink).toHaveAttribute("href", "/operator/settings");
    });
  });

  it("does not show retry button when no token", async () => {
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText("Erneut versuchen")).not.toBeInTheDocument();
  });

  it("shows idle state with confirm button when token is provided", async () => {
    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse bestätigen")).toBeInTheDocument();
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
  });

  it("strips token from URL on mount", async () => {
    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(replaceStateSpy).toHaveBeenCalledWith(
        {},
        "",
        window.location.pathname,
      );
    });
  });

  it("does not strip URL when no token", async () => {
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
    });
    expect(replaceStateSpy).not.toHaveBeenCalled();
  });

  it("shows success state after successful confirmation", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "Email changed successfully" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
  });

  it("shows success link to settings profile tab", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      const link = screen.getByText("Weiter");
      expect(link).toHaveAttribute("href", "/operator/settings?tab=profile");
    });
  });

  it("triggers session update on success when authenticated", async () => {
    mockUseSession.mockReturnValue({
      update: mockUpdateSession,
      status: "authenticated",
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(mockUpdateSession).toHaveBeenCalledWith({ emailChanged: true });
    });
  });

  it("does not trigger session update when unauthenticated", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
    expect(mockUpdateSession).not.toHaveBeenCalled();
  });

  it("handles session update failure gracefully", async () => {
    mockUseSession.mockReturnValue({
      update: mockUpdateSession,
      status: "authenticated",
    });

    mockUpdateSession.mockRejectedValueOnce(new Error("session error"));

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
  });

  it("shows retryable error on 5xx response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({ error: "Serverfehler" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
      expect(screen.getByText("Serverfehler")).toBeInTheDocument();
      expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
    });
  });

  it("uses default 5xx error message when response has no error field", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: async () => ({}),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Ein Serverfehler ist aufgetreten. Bitte versuche es später erneut.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("falls back to message field for 5xx errors", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 502,
      json: async () => ({ message: "Bad Gateway" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("Bad Gateway")).toBeInTheDocument();
    });
  });

  it("shows non-retryable error on 4xx response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => ({ error: "Token abgelaufen" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
      expect(screen.getByText("Token abgelaufen")).toBeInTheDocument();
      expect(screen.queryByText("Erneut versuchen")).not.toBeInTheDocument();
    });
  });

  it("uses default 4xx error message when response has no error field", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => ({}),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText("Dieser Link ist abgelaufen oder ungültig."),
      ).toBeInTheDocument();
    });
  });

  it("shows retryable error on network failure", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Ein Fehler ist aufgetreten. Bitte versuche es später erneut.",
        ),
      ).toBeInTheDocument();
      expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
    });
  });

  it("handles retry after error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({ error: "Serverfehler" }),
    });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Erneut versuchen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
  });

  it("sends correct request to API", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setHash("#token=my-test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/operator/auth/email-confirm",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token: "my-test-token" }),
        }),
      );
    });
  });

  it("prevents double-click during confirmation", async () => {
    let resolveResponse: (value: unknown) => void;
    mockFetch.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveResponse = resolve;
      }),
    );

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    // While confirming, the idle UI is gone — can't double-click
    expect(screen.queryByText("Jetzt bestätigen")).not.toBeInTheDocument();

    resolveResponse!({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("handles non-Error thrown by fetch", async () => {
    mockFetch.mockRejectedValueOnce("string error");

    setHash("#token=test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
    });
  });
});
