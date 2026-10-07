import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render as renderPlain,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const OBJECT = "die Bestätigung der E-Mail-Adresse";

function errorResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status });
}

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

/** Sets the current URL to /operator/email-confirm?token={value} via pushState. */
function setQueryToken(token: string) {
  window.history.pushState({}, "", `/operator/email-confirm?token=${token}`);
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
    // Reset URL before installing the replaceState spy (spy is a no-op, so
    // once installed it cannot be used to mutate the URL).
    window.history.pushState({}, "", "/operator/email-confirm");
    replaceStateSpy = vi
      .spyOn(window.history, "replaceState")
      .mockImplementation(() => {});
  });

  afterEach(() => {
    replaceStateSpy.mockRestore();
    window.history.pushState({}, "", "/operator/email-confirm");
  });

  it("shows error when no token is provided", async () => {
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(
        screen.getByText("Bestätigung fehlgeschlagen"),
      ).toBeInTheDocument();
      expect(
        screen.getByText(
          "Der Link enthält keinen Bestätigungscode. Bitte öffnen Sie den Link aus der E-Mail erneut.",
        ),
      ).toBeInTheDocument();
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
    expect(screen.queryByText("Wiederholen")).not.toBeInTheDocument();
  });

  it("shows idle state with confirm button when token is provided", async () => {
    setQueryToken("test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse bestätigen")).toBeInTheDocument();
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
  });

  it("strips token from URL on mount", async () => {
    setQueryToken("test-token");
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

    setQueryToken("test-token");
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

    setQueryToken("test-token");
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

    setQueryToken("test-token");
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

    setQueryToken("test-token");
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

    setQueryToken("test-token");
    render(<EmailConfirmContent />);

    await waitFor(() => {
      expect(screen.getByText("Jetzt bestätigen")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Jetzt bestätigen"));

    await waitFor(() => {
      expect(screen.getByText("E-Mail-Adresse geändert")).toBeInTheDocument();
    });
  });

  async function confirmWith(response: unknown) {
    if (response instanceof Error) mockFetch.mockRejectedValueOnce(response);
    else mockFetch.mockResolvedValueOnce(response);
    setQueryToken("test-token");
    render(<EmailConfirmContent />);
    fireEvent.click(await screen.findByText("Jetzt bestätigen"));
    expect(
      await screen.findByText("Bestätigung fehlgeschlagen"),
    ).toBeInTheDocument();
  }

  // #2519: catalog text by code, retry only for server and unavailable
  // errors, never the backend sentence.
  it("shows a server error with retry", async () => {
    await confirmWith(errorResponse(500, { error: "Serverfehler" }));

    expect(
      await screen.findByText(catalogText("general.server", OBJECT)),
    ).toBeInTheDocument();
    expect(screen.getByText("Wiederholen")).toBeInTheDocument();
    expect(screen.queryByText("Serverfehler")).toBeNull();
  });

  it("shows an expired link by its code without retry", async () => {
    await confirmWith(
      errorResponse(400, {
        error: "Token abgelaufen",
        code: "general.input",
      }),
    );

    expect(
      await screen.findByText(catalogText("general.input", OBJECT)),
    ).toBeInTheDocument();
    expect(screen.queryByText("Wiederholen")).not.toBeInTheDocument();
    expect(screen.queryByText("Token abgelaufen")).toBeNull();
  });

  it("shows a network failure as unavailable with retry", async () => {
    await confirmWith(new TypeError("Failed to fetch"));

    expect(
      await screen.findByText(catalogText("general.unavailable", OBJECT)),
    ).toBeInTheDocument();
    expect(screen.getByText("Wiederholen")).toBeInTheDocument();
  });

  it("handles retry after error", async () => {
    mockFetch.mockResolvedValueOnce(errorResponse(500, {}));
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ message: "OK" }),
    });

    setQueryToken("test-token");
    render(<EmailConfirmContent />);
    fireEvent.click(await screen.findByText("Jetzt bestätigen"));

    fireEvent.click(await screen.findByText("Wiederholen"));

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

    setQueryToken("my-test-token");
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

    setQueryToken("test-token");
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

    setQueryToken("test-token");
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
