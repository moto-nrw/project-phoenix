/**
 * Tests for DeleteDeviceModal.
 *
 * Covers: null device returns null, two-step confirmation flow, cancel at
 * each stage, success path triggers onDeleted + onClose, and error path
 * surfaces the API message and keeps the modal open.
 */
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockDeleteDevice } = vi.hoisted(() => ({
  mockDeleteDevice: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    deleteDevice: mockDeleteDevice,
  },
}));

import { DeleteDeviceModal } from "./delete-device-modal";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";

function makeDevice(overrides: Partial<OperatorDevice> = {}): OperatorDevice {
  return {
    id: "1",
    deviceId: "OGS-001",
    deviceType: "ogs",
    name: "Empfang",
    status: "active",
    apiKey: "",
    maskedApiKey: "abc***",
    lastSeen: null,
    isOnline: false,
    schoolId: "10",
    schoolName: "Schule A",
    organizationId: "42",
    organizationName: "Träger A",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("DeleteDeviceModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when device is null", () => {
    const { container } = render(
      <DeleteDeviceModal device={null} onClose={vi.fn()} onDeleted={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders device id, optional name, and school name", () => {
    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    expect(screen.getByText("Gerät löschen")).toBeInTheDocument();
    expect(screen.getByText("OGS-001")).toBeInTheDocument();
    expect(screen.getByText(/Empfang/)).toBeInTheDocument();
    expect(screen.getByText("Schule A")).toBeInTheDocument();
  });

  it("calls onClose when Abbrechen is clicked at the first step", () => {
    const onClose = vi.fn();
    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={onClose}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(mockDeleteDevice).not.toHaveBeenCalled();
  });

  it("requires confirmation before invoking the delete API", () => {
    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));

    // The "Endgültig löschen" button is now visible; deleteDevice has not yet been called
    expect(screen.getByText("Endgültig löschen")).toBeInTheDocument();
    expect(mockDeleteDevice).not.toHaveBeenCalled();
  });

  it("deletes the device, calls onDeleted, and closes on success", async () => {
    mockDeleteDevice.mockResolvedValue(undefined);
    const onClose = vi.fn();
    const onDeleted = vi.fn().mockResolvedValue(undefined);

    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={onClose}
        onDeleted={onDeleted}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(mockDeleteDevice).toHaveBeenCalledWith("1");
    });
    await waitFor(() => {
      expect(onDeleted).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  it("surfaces the API error message and keeps the modal open on failure", async () => {
    mockDeleteDevice.mockRejectedValue(new Error("device in use"));
    const onClose = vi.fn();
    const onDeleted = vi.fn();

    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={onClose}
        onDeleted={onDeleted}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(screen.getByText("device in use")).toBeInTheDocument();
    });
    expect(onDeleted).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("disables both buttons and shows in-flight label while delete is pending", async () => {
    let resolveDelete: (() => void) | undefined;
    mockDeleteDevice.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveDelete = resolve;
      }),
    );

    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(screen.getByText("Wird gelöscht…")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: "Wird gelöscht…" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();

    resolveDelete?.();
  });

  it("does not fire a second delete request when the button is clicked twice rapidly", async () => {
    let resolveDelete: (() => void) | undefined;
    mockDeleteDevice.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveDelete = resolve;
      }),
    );

    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));
    const confirmButton = screen.getByRole("button", {
      name: "Endgültig löschen",
    });
    fireEvent.click(confirmButton);
    fireEvent.click(confirmButton);
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockDeleteDevice).toHaveBeenCalledTimes(1);
    });

    resolveDelete?.();
  });

  it("falls back to a generic German error when the thrown value has no message", async () => {
    mockDeleteDevice.mockRejectedValue("non-error value");

    render(
      <DeleteDeviceModal
        device={makeDevice()}
        onClose={vi.fn()}
        onDeleted={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Ja, löschen"));
    fireEvent.click(screen.getByText("Endgültig löschen"));

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Löschen des Geräts"),
      ).toBeInTheDocument();
    });
  });
});
