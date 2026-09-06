import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { DevicesMasterDetail } from "./devices-master-detail";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Device } from "@/lib/iot-helpers";

const onlineDevice: Device = {
  id: "1",
  device_id: "kiosk-001",
  device_type: "kiosk",
  name: "Eingang Kiosk",
  status: "active",
  is_online: true,
  room_name: "Foyer",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-02T00:00:00Z",
};

const offlineDeviceWithKey: Device = {
  ...onlineDevice,
  id: "2",
  device_id: "kiosk-002",
  name: "Backup Kiosk",
  is_online: false,
  status: "offline",
  api_key: "secret-token-abc",
  last_seen: "2026-01-01T00:00:00Z",
};

function flatGroup(devices: Device[]): GroupDefinition<Device>[] {
  if (devices.length === 0) return [];
  return [
    {
      id: "__flat__",
      title: `Alle Geräte (${devices.length})`,
      items: devices,
    },
  ];
}

describe("DevicesMasterDetail", () => {
  const onSelect = vi.fn();
  const onSaveDevice = vi.fn(async () => undefined);
  const onDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([onlineDevice])}
        selectedId={null}
        selectedDevice={null}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.getByText("Eingang Kiosk")).toBeInTheDocument();
  });

  it("renders the selected device detail with id, room, and status", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([onlineDevice])}
        selectedId="1"
        selectedDevice={onlineDevice}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getAllByText("Eingang Kiosk").length).toBeGreaterThan(0);
    expect(screen.getByText("kiosk-001")).toBeInTheDocument();
    expect(screen.getByText("Foyer")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
  });

  it("opens the inline edit form from the header and triggers delete", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([onlineDevice])}
        selectedId="1"
        selectedDevice={onlineDevice}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();

    // Bearbeitet wird im Detailbereich, nicht in einem Modal daneben: der
    // Knopf schaltet das Formular ein, mit Speichern und Abbrechen unten.
    fireEvent.click(screen.getByText("Bearbeiten"));
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abbrechen" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
  });

  it("renders the API key section only when api_key is present", () => {
    const { rerender } = render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([onlineDevice])}
        selectedId="1"
        selectedDevice={onlineDevice}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.queryByText(/API-Schlüssel/)).not.toBeInTheDocument();

    rerender(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([offlineDeviceWithKey])}
        selectedId="2"
        selectedDevice={offlineDeviceWithKey}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(
      screen.getByText(/API-Schlüssel \(nur einmal sichtbar\)/),
    ).toBeInTheDocument();
    // The hidden input value should match the api key.
    expect(
      (screen.getByDisplayValue("secret-token-abc") as HTMLInputElement).type,
    ).toBe("password");

    fireEvent.click(screen.getByText("Anzeigen"));
    expect(
      (screen.getByDisplayValue("secret-token-abc") as HTMLInputElement).type,
    ).toBe("text");
  });

  it("renders the group titles supplied by the page and a Stammdaten tab", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={[
          {
            id: "kiosk",
            title: "Kiosk (1)",
            items: [onlineDevice],
          },
          {
            id: "info_point",
            title: "Info Point (1)",
            items: [offlineDeviceWithKey],
          },
        ]}
        selectedId="1"
        selectedDevice={onlineDevice}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getByText("Kiosk (1)")).toBeInTheDocument();
    expect(screen.getByText("Info Point (1)")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("toggles the API key reveal back to masked after a second click", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([offlineDeviceWithKey])}
        selectedId="2"
        selectedDevice={offlineDeviceWithKey}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    const reveal = screen.getByText("Anzeigen");
    fireEvent.click(reveal);
    expect(
      (screen.getByDisplayValue("secret-token-abc") as HTMLInputElement).type,
    ).toBe("text");

    fireEvent.click(screen.getByText("Verbergen"));
    expect(
      (screen.getByDisplayValue("secret-token-abc") as HTMLInputElement).type,
    ).toBe("password");
  });

  it("copies the API key to the clipboard and flips the button to 'Kopiert!'", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([offlineDeviceWithKey])}
        selectedId="2"
        selectedDevice={offlineDeviceWithKey}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    fireEvent.click(screen.getByText("Kopieren"));
    expect(writeText).toHaveBeenCalledWith("secret-token-abc");

    await waitFor(() => {
      expect(screen.getByText("Kopiert!")).toBeInTheDocument();
    });
  });

  it("renders the offline status text and a labeled indicator dot when the device is offline", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([offlineDeviceWithKey])}
        selectedId="2"
        selectedDevice={offlineDeviceWithKey}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    // Multiple "Offline" texts surface intentionally: the connection field, the
    // status pill, and the list dot. We only assert that at least one is shown
    // and that the labeled list-item indicator is also present.
    expect(screen.getAllByText("Offline").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByLabelText("Offline").length).toBeGreaterThan(0);
  });

  it("labels the list-item connection dot as 'Online' for is_online devices", () => {
    render(
      <DevicesMasterDetail
        groupDefinitions={flatGroup([onlineDevice])}
        selectedId={null}
        selectedDevice={null}
        onSelect={onSelect}
        onSaveDevice={onSaveDevice}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getAllByLabelText("Online").length).toBeGreaterThan(0);
  });
});
