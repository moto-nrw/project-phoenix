/**
 * Tests for PrivacyConsentSection Component
 * Tests privacy consent display functionality
 */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PrivacyConsentSection } from "./privacy-consent-section";
import type { PrivacyConsent } from "~/lib/student-helpers";
import { ApiError } from "~/lib/api-error";

// Mock the student API
vi.mock("~/lib/student-api", () => ({
  fetchStudentPrivacyConsent: vi.fn(),
}));

import { fetchStudentPrivacyConsent } from "~/lib/student-api";

describe("PrivacyConsentSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state initially", () => {
    vi.mocked(fetchStudentPrivacyConsent).mockImplementation(
      () =>
        new Promise(() => {
          /* noop */
        }), // Never resolves
    );

    render(<PrivacyConsentSection studentId="123" />);

    expect(
      screen.getByText("Lade Datenschutzeinstellungen..."),
    ).toBeInTheDocument();
  });

  it("displays consent data when loaded", async () => {
    const mockConsent = {
      dataRetentionDays: 30,
      accepted: true,
      acceptedAt: "2024-01-15T10:00:00Z",
      expiresAt: null,
      renewalRequired: false,
    } as unknown as PrivacyConsent;

    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(mockConsent);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(screen.getByText("30 Tage")).toBeInTheDocument();
    });

    expect(screen.getByText("Erteilt")).toBeInTheDocument();
  });

  it("displays default retention when not set", async () => {
    const mockConsent = {
      dataRetentionDays: null,
      accepted: false,
      acceptedAt: null,
      expiresAt: null,
      renewalRequired: false,
    } as unknown as PrivacyConsent;

    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(mockConsent);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(screen.getByText("30 Tage")).toBeInTheDocument();
    });
  });

  it("shows not accepted status", async () => {
    const mockConsent = {
      dataRetentionDays: 30,
      accepted: false,
      acceptedAt: null,
      expiresAt: null,
      renewalRequired: false,
    } as unknown as PrivacyConsent;

    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(mockConsent);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(screen.getByText("Nicht erteilt")).toBeInTheDocument();
    });
  });

  it("shows expiry date when provided", async () => {
    const mockConsent = {
      dataRetentionDays: 30,
      accepted: true,
      acceptedAt: "2024-01-15T10:00:00Z",
      expiresAt: "2025-01-15T10:00:00Z",
      renewalRequired: false,
    } as unknown as PrivacyConsent;

    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(mockConsent);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(screen.getByText(/Gültig bis:/)).toBeInTheDocument();
    });
  });

  it("shows renewal warning when required", async () => {
    const mockConsent = {
      dataRetentionDays: 30,
      accepted: true,
      acceptedAt: "2024-01-15T10:00:00Z",
      expiresAt: "2024-12-01T10:00:00Z",
      renewalRequired: true,
    } as unknown as PrivacyConsent;

    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(mockConsent);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(
        screen.getByText("⚠️ Einwilligung muss erneuert werden"),
      ).toBeInTheDocument();
    });
  });

  it("shows no consent message when consent is null", async () => {
    vi.mocked(fetchStudentPrivacyConsent).mockResolvedValue(null);

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Datenschutzeinwilligung hinterlegt/),
      ).toBeInTheDocument();
    });
  });

  // Ein Ladefehler ist kein „keine Einwilligung hinterlegt“ (#2513): er steht
  // vor Ort, mit Wiederholen.
  it("shows a failed load in place instead of an empty consent, with retry", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => {
        /* noop */
      });
    vi.mocked(fetchStudentPrivacyConsent)
      .mockRejectedValueOnce(
        new ApiError("API Error", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({
        dataRetentionDays: 14,
        accepted: true,
      } as unknown as PrivacyConsent);

    render(<PrivacyConsentSection studentId="123" />);

    expect(
      await screen.findByText(
        "Die Einwilligung ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Keine Datenschutzeinwilligung hinterlegt/),
    ).not.toBeInTheDocument();
    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "failed to load privacy consent",
      expect.objectContaining({
        error: "API Error",
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("14 Tage")).toBeInTheDocument();

    consoleErrorSpy.mockRestore();
  });
});
