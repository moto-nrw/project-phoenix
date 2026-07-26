import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { CreateDeviceModal } from "./create-device-modal";
import { OperatorApiError } from "~/lib/operator/api-helpers";

function selectCustomOption(
  labelMatcher: RegExp | string,
  optionName: RegExp | string,
) {
  fireEvent.click(screen.getByLabelText(labelMatcher));
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

const { mockCreateDevice, mockLoggerError } = vi.hoisted(() => ({
  mockCreateDevice: vi.fn(),
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
    createDevice: mockCreateDevice,
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

describe("CreateDeviceModal", () => {
  const onClose = vi.fn();
  const onCreated = vi.fn();
  const school = {
    id: "10",
    organizationId: "1",
    name: "Test School",
    slug: "test-school",
    subdomain: "test-school",
    address: "",
    city: "",
    zip: "",
    phone: "",
    email: "",
    active: true,
    hidden: false,
    deletedAt: null,
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    organization: undefined,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("submits a device with a manual api key", async () => {
    mockCreateDevice.mockResolvedValue({
      id: "200",
      deviceId: "DEV-001",
      deviceType: "terminal",
      name: "Eingang",
      status: "active",
      apiKey: "manual-key",
      maskedApiKey: "****key",
      lastSeen: null,
      isOnline: false,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2025-01-01T00:00:00Z",
      updatedAt: "2025-01-01T00:00:00Z",
    });

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "  DEV-001  " },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "  Eingang  " },
    });
    fireEvent.click(screen.getByLabelText("Eigenen Key eingeben"));
    fireEvent.change(screen.getByPlaceholderText("API-Key eingeben..."), {
      target: { value: "  manual-key  " },
    });

    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(mockCreateDevice).toHaveBeenCalledWith({
        school_id: 10,
        device_id: "DEV-001",
        device_type: "terminal",
        name: "Eingang",
        api_key: "manual-key",
      });
      expect(onCreated).toHaveBeenCalledWith(
        expect.objectContaining({ id: "200", apiKey: "manual-key" }),
      );
      expect(
        screen.getByText("Gerät erfolgreich erstellt"),
      ).toBeInTheDocument();
    });
  });

  it("shows duplicate api key conflict message", async () => {
    mockCreateDevice.mockRejectedValue(
      new OperatorApiError("api_key already exists", 409),
    );

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText("Dieser API-Key wird bereits verwendet."),
      ).toBeInTheDocument();
    });
  });

  it("shows duplicate device id conflict message", async () => {
    mockCreateDevice.mockRejectedValue(
      new OperatorApiError("device already exists", 409),
    );

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(
        screen.getByText(
          "Ein Gerät mit dieser ID existiert bereits für diese Schule.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("logs and shows a generic error", async () => {
    mockCreateDevice.mockRejectedValue(new Error("kaputt"));

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(screen.getByText("kaputt")).toBeInTheDocument();
      expect(mockLoggerError).toHaveBeenCalledWith(
        "device_create_failed",
        expect.objectContaining({ error: "kaputt" }),
      );
    });
  });

  it("allows selecting a school when multiple schools exist", async () => {
    mockCreateDevice.mockResolvedValue({
      id: "200",
      deviceId: "DEV-001",
      deviceType: "terminal",
      name: "Eingang",
      status: "active",
      apiKey: "",
      maskedApiKey: "****key",
      lastSeen: null,
      isOnline: false,
      schoolId: "11",
      schoolName: "Other School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2025-01-01T00:00:00Z",
      updatedAt: "2025-01-01T00:00:00Z",
    });

    const { container } = render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school, { ...school, id: "11", name: "Other School" }]}
        onCreated={onCreated}
      />,
    );

    selectCustomOption(/Schule/, "Other School");
    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");

    fireEvent.submit(container.querySelector("#create-device-form")!);

    await waitFor(() => {
      expect(mockCreateDevice).toHaveBeenCalledWith(
        expect.objectContaining({ school_id: 11 }),
      );
    });
  });

  it("copies the created api key to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });
    mockCreateDevice.mockResolvedValue({
      id: "200",
      deviceId: "DEV-001",
      deviceType: "terminal",
      name: "Eingang",
      status: "active",
      apiKey: "manual-key",
      maskedApiKey: "****key",
      lastSeen: null,
      isOnline: false,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2025-01-01T00:00:00Z",
      updatedAt: "2025-01-01T00:00:00Z",
    });

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.click(screen.getByText("Erstellen"));

    await waitFor(() => {
      expect(screen.getByText("Kopieren")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Kopieren"));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("manual-key");
      expect(screen.getByText("Kopiert!")).toBeInTheDocument();
    });
  });

  it("logs clipboard errors after device creation", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("denied"));
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      writable: true,
      configurable: true,
    });
    mockCreateDevice.mockResolvedValue({
      id: "200",
      deviceId: "DEV-001",
      deviceType: "terminal",
      name: "Eingang",
      status: "active",
      apiKey: "manual-key",
      maskedApiKey: "****key",
      lastSeen: null,
      isOnline: false,
      schoolId: "10",
      schoolName: "Test School",
      organizationId: "1",
      organizationName: "Test Org",
      createdAt: "2025-01-01T00:00:00Z",
      updatedAt: "2025-01-01T00:00:00Z",
    });

    render(
      <CreateDeviceModal
        isOpen={true}
        onClose={onClose}
        schools={[school]}
        onCreated={onCreated}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Geräte-ID/), {
      target: { value: "DEV-001" },
    });
    selectCustomOption(/Typ/, "Terminal");
    fireEvent.click(screen.getByText("Erstellen"));

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
});
