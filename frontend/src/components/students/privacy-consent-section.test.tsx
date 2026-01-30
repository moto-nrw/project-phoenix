/**
 * Tests for PrivacyConsentSection Component
 * Tests privacy consent display functionality
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PrivacyConsentSection } from "./privacy-consent-section";
import type { PrivacyConsent } from "~/lib/student-helpers";

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

  it("handles errors gracefully", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => {
        /* noop */
      });
    vi.mocked(fetchStudentPrivacyConsent).mockRejectedValue(
      new Error("API Error"),
    );

    render(<PrivacyConsentSection studentId="123" />);

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Datenschutzeinwilligung hinterlegt/),
      ).toBeInTheDocument();
    });

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Error loading privacy consent:",
      expect.any(Error),
    );

    consoleErrorSpy.mockRestore();
  });
});
