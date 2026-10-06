import type { ReactNode } from "react";
import {
  render as renderPlain,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { ERROR_CATALOG } from "~/lib/error-catalog.generated";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

/** What the shared path shows for a failure that is no API error. */
function crashText(object: string) {
  const text = ERROR_CATALOG.de.actions.crash.replace("{object}", object);
  return text.charAt(0).toUpperCase() + text.slice(1);
}
import { SetApiKeyModal } from "./set-api-key-modal";

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
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
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
      new ApiError("not found", 404, { code: "general.input" }),
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

    expect(
      await screen.findByText(
        catalogText("general.input", "die Änderung des API-Keys"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("not found")).toBeNull();
  });

  it("shows duplicate api key conflict", async () => {
    mockSetDeviceAPIKey.mockRejectedValue(
      new ApiError("already exists", 409, {
        code: "general.business_rejection",
        errors: [{ field: "api_key", reason: "taken" }],
      }),
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

    expect(
      await screen.findByText(
        catalogText("general.business_rejection", "die Änderung des API-Keys"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText("API-Key eingeben...")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
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
      expect(
        screen.getByText(crashText("die Änderung des API-Keys")),
      ).toBeInTheDocument();
      expect(screen.queryByText("kaputt")).toBeNull();
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
    expect(
      await screen.findByText(
        "Der API-Key konnte nicht kopiert werden. Bitte markieren und kopieren Sie ihn selbst.",
      ),
    ).toBeInTheDocument();
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
