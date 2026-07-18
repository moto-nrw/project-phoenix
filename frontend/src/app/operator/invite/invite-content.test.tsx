import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const { mockEstablish, mockValidate, mockAccept } = vi.hoisted(() => ({
  mockEstablish: vi.fn(),
  mockValidate: vi.fn(),
  mockAccept: vi.fn(),
}));

vi.mock("~/lib/operator/operator-invitation-api", () => ({
  establishOperatorInvitationSession: mockEstablish,
  validateOperatorInvitation: mockValidate,
  acceptOperatorInvitation: mockAccept,
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

import { InviteContent } from "./invite-content";

/** Sets the current URL to /operator/invite?token={value} via pushState. */
function setQueryToken(token: string) {
  window.history.pushState({}, "", `/operator/invite?token=${token}`);
}

function setQueryFlow(flowID: string) {
  window.history.pushState({}, "", `/operator/invite?flow=${flowID}`);
}

describe("InviteContent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockEstablish.mockResolvedValue("flow-123");
    mockValidate.mockRejectedValue(new Error("Kein Token angegeben."));
    // Reset URL (strips any leftover ?token=... from a prior test)
    window.history.pushState({}, "", "/operator/invite");
  });

  it("shows error when no token is available", async () => {
    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Einladung ungültig")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Kein Einladungsvorgang angegeben."),
    ).toBeInTheDocument();
  });

  it("validates token from URL query and shows form", async () => {
    setQueryToken("valid-token-123");
    mockValidate.mockResolvedValue({
      email: "invited@example.com",
      displayName: "Test User",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Operator-Konto erstellen")).toBeInTheDocument();
    });
    expect(screen.getByText(/invited@example.com/)).toBeInTheDocument();
    expect(mockEstablish).toHaveBeenCalledWith("valid-token-123");
    expect(mockValidate).toHaveBeenCalledWith("flow-123");
    expect(window.location.search).toBe("?flow=flow-123");
  });

  it("pre-fills display name from invitation", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "invited@example.com",
      displayName: "Pre-filled Name",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Operator-Konto erstellen")).toBeInTheDocument();
    });
    const input = screen.getByLabelText("Anzeigename *") as HTMLInputElement;
    expect(input.value).toBe("Pre-filled Name");
  });

  it("reuses the HttpOnly invitation session when the URL has no token", async () => {
    setQueryFlow("existing-flow");
    mockValidate.mockResolvedValue({
      email: "session@example.com",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Operator-Konto erstellen")).toBeInTheDocument();
    });
    expect(mockEstablish).not.toHaveBeenCalled();
    expect(mockValidate).toHaveBeenCalledWith("existing-flow");
  });

  it("shows error state when validation fails", async () => {
    setQueryToken("expired-token");
    mockValidate.mockRejectedValue(
      new Error("Dieser Link ist abgelaufen oder ungültig"),
    );

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Einladung ungültig")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Dieser Link ist abgelaufen oder ungültig"),
    ).toBeInTheDocument();
  });

  it("shows password rules when typing", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "a" },
    });

    expect(screen.getByText("Mindestens 8 Zeichen")).toBeInTheDocument();
    expect(screen.getByText("Ein Großbuchstabe")).toBeInTheDocument();
    expect(screen.getByText("Ein Kleinbuchstabe")).toBeInTheDocument();
    expect(screen.getByText("Eine Zahl")).toBeInTheDocument();
    expect(screen.getByText("Ein Sonderzeichen")).toBeInTheDocument();
  });

  it("shows password mismatch message", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "Test1234!" },
    });
    fireEvent.change(screen.getByLabelText("Passwort bestätigen *"), {
      target: { value: "Different" },
    });

    expect(
      screen.getByText("Passwörter stimmen nicht überein"),
    ).toBeInTheDocument();
  });

  it("submits form and shows success", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      displayName: "Test",
      expiresAt: "2026-04-06T00:00:00Z",
    });
    mockAccept.mockResolvedValue(undefined);

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Anzeigename *"), {
      target: { value: "New Operator" },
    });
    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "Str0ng!Pass" },
    });
    fireEvent.change(screen.getByLabelText("Passwort bestätigen *"), {
      target: { value: "Str0ng!Pass" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Konto erstellen" }));

    await waitFor(() => {
      expect(screen.getByText("Konto erstellt")).toBeInTheDocument();
    });
    expect(mockAccept).toHaveBeenCalledWith("flow-123", {
      displayName: "New Operator",
      password: "Str0ng!Pass",
      confirmPassword: "Str0ng!Pass",
    });
    expect(window.location.search).toBe("");
  });

  it("shows form error when accept fails", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      displayName: "Test",
      expiresAt: "2026-04-06T00:00:00Z",
    });
    mockAccept.mockRejectedValue(new Error("Passwort zu schwach"));

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Anzeigename *"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "Str0ng!Pass" },
    });
    fireEvent.change(screen.getByLabelText("Passwort bestätigen *"), {
      target: { value: "Str0ng!Pass" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Konto erstellen" }));

    await waitFor(() => {
      expect(screen.getByText("Passwort zu schwach")).toBeInTheDocument();
    });
  });

  it("shows validation error for empty display name on submit", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    // Clear display name and fill passwords
    fireEvent.change(screen.getByLabelText("Anzeigename *"), {
      target: { value: "" },
    });
    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "Str0ng!Pass" },
    });
    fireEvent.change(screen.getByLabelText("Passwort bestätigen *"), {
      target: { value: "Str0ng!Pass" },
    });

    // Submit button is disabled when display name is empty, so submit via form
    const form = screen.getByLabelText("Passwort *").closest("form")!;
    fireEvent.submit(form);

    await waitFor(() => {
      expect(
        screen.getByText("Anzeigename ist erforderlich."),
      ).toBeInTheDocument();
    });
    expect(mockAccept).not.toHaveBeenCalled();
  });

  it("has login link on success page", async () => {
    setQueryToken("valid-token");
    mockValidate.mockResolvedValue({
      email: "test@example.com",
      displayName: "Test",
      expiresAt: "2026-04-06T00:00:00Z",
    });
    mockAccept.mockResolvedValue(undefined);

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByLabelText("Passwort *")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Anzeigename *"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByLabelText("Passwort *"), {
      target: { value: "Str0ng!Pass" },
    });
    fireEvent.change(screen.getByLabelText("Passwort bestätigen *"), {
      target: { value: "Str0ng!Pass" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Konto erstellen" }));

    await waitFor(() => {
      expect(screen.getByText("Konto erstellt")).toBeInTheDocument();
    });
    expect(screen.getByText("Zur Anmeldung")).toBeInTheDocument();
  });

  it("has login link on error page", async () => {
    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Einladung ungültig")).toBeInTheDocument();
    });
    expect(screen.getByText("Zur Anmeldung")).toBeInTheDocument();
  });
});
