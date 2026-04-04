import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const { mockValidate, mockAccept } = vi.hoisted(() => ({
  mockValidate: vi.fn(),
  mockAccept: vi.fn(),
}));

vi.mock("~/lib/operator/operator-invitation-api", () => ({
  validateOperatorInvitation: mockValidate,
  acceptOperatorInvitation: mockAccept,
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

import { InviteContent } from "./invite-content";

describe("InviteContent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset sessionStorage
    sessionStorage.clear();
    // Reset URL hash
    window.location.hash = "";
  });

  it("shows error when no token is available", async () => {
    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Einladung ungültig")).toBeInTheDocument();
    });
    expect(screen.getByText("Kein Token angegeben.")).toBeInTheDocument();
  });

  it("validates token from URL hash and shows form", async () => {
    window.location.hash = "#token=valid-token-123";
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
    expect(mockValidate).toHaveBeenCalledWith("valid-token-123");
  });

  it("pre-fills display name from invitation", async () => {
    window.location.hash = "#token=valid-token";
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

  it("falls back to sessionStorage token", async () => {
    sessionStorage.setItem("operator_invite_token", "session-token");
    mockValidate.mockResolvedValue({
      email: "session@example.com",
      expiresAt: "2026-04-06T00:00:00Z",
    });

    render(<InviteContent />);

    await waitFor(() => {
      expect(screen.getByText("Operator-Konto erstellen")).toBeInTheDocument();
    });
    expect(mockValidate).toHaveBeenCalledWith("session-token");
  });

  it("shows error state when validation fails", async () => {
    window.location.hash = "#token=expired-token";
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
    window.location.hash = "#token=valid-token";
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
    window.location.hash = "#token=valid-token";
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
    window.location.hash = "#token=valid-token";
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
    expect(mockAccept).toHaveBeenCalledWith("valid-token", {
      display_name: "New Operator",
      password: "Str0ng!Pass",
      confirm_password: "Str0ng!Pass",
    });
  });

  it("shows form error when accept fails", async () => {
    window.location.hash = "#token=valid-token";
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
    window.location.hash = "#token=valid-token";
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
    window.location.hash = "#token=valid-token";
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
