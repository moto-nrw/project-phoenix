import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { SetApiKeyModal } from "./set-api-key-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";

const { mockSetDeviceAPIKey, mockLoggerError } = vi.hoisted(() => ({
  mockSetDeviceAPIKey: vi.fn(),
  mockLoggerError: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    setDeviceAPIKey: mockSetDeviceAPIKey,
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

describe("SetApiKeyModal", () => {
  const onClose = vi.fn();
  const onKeySet = vi.fn();
  const device = {
    id: "200",
    deviceId: "DEV-001",
    deviceType: "terminal",
    name: "Eingang",
    status: "active",
    apiKey: "",
    maskedApiKey: "****key",
    lastSeen: null,
    isOnline: false,
    schoolId: "10",
    schoolName: "Test School",
    organizationId: "1",
    organizationName: "Test Org",
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("submits a manual replacement key", async () => {
    mockSetDeviceAPIKey.mockResolvedValue({ ...device, apiKey: "manual-key" });

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Key eingeben"));
    fireEvent.change(screen.getByPlaceholderText("API-Key eingeben..."), {
      target: { value: "  manual-key  " },
    });
    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(mockSetDeviceAPIKey).toHaveBeenCalledWith("200", "manual-key");
      expect(onKeySet).toHaveBeenCalledWith(
        expect.objectContaining({ apiKey: "manual-key" }),
      );
      expect(screen.getByText("Neuer API-Key:")).toBeInTheDocument();
    });
  });

  it("shows not found error for missing device", async () => {
    mockSetDeviceAPIKey.mockRejectedValue(
      new OperatorApiError("not found", 404),
    );

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(screen.getByText("Gerät nicht gefunden.")).toBeInTheDocument();
    });
  });

  it("shows duplicate api key conflict", async () => {
    mockSetDeviceAPIKey.mockRejectedValue(
      new OperatorApiError("already exists", 409),
    );

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Key eingeben"));
    fireEvent.change(screen.getByPlaceholderText("API-Key eingeben..."), {
      target: { value: "manual-key" },
    });
    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(
        screen.getByText("Dieser API-Key wird bereits verwendet."),
      ).toBeInTheDocument();
    });
  });

  it("logs and shows a generic error", async () => {
    mockSetDeviceAPIKey.mockRejectedValue(new Error("kaputt"));

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(screen.getByText("kaputt")).toBeInTheDocument();
      expect(mockLoggerError).toHaveBeenCalledWith(
        "set_api_key_failed",
        expect.objectContaining({ error: "kaputt", device_id: "200" }),
      );
    });
  });

  it("copies the updated api key to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });
    mockSetDeviceAPIKey.mockResolvedValue({ ...device, apiKey: "manual-key" });

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(screen.getByText("Kopieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Kopieren"));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("manual-key");
      expect(screen.getByText("Kopiert!")).toBeInTheDocument();
    });
  });

  it("logs clipboard errors after updating the key", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("denied"));
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });
    mockSetDeviceAPIKey.mockResolvedValue({ ...device, apiKey: "manual-key" });

    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByText("Übernehmen"));

    await waitFor(() => {
      expect(screen.getByText("Kopieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Kopieren"));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "clipboard_copy_failed",
        expect.objectContaining({
          error: "Failed to copy API key to clipboard",
        }),
      );
    });
  });

  it("disables submit for manual mode without a custom key", () => {
    render(
      <SetApiKeyModal
        isOpen={true}
        onClose={onClose}
        device={device}
        onKeySet={onKeySet}
      />,
    );

    fireEvent.click(screen.getByLabelText("Eigenen Key eingeben"));

    expect(screen.getByText("Übernehmen")).toBeDisabled();
  });
});
