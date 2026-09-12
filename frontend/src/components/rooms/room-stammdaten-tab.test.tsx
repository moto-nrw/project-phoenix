import "@testing-library/jest-dom/vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Room } from "~/lib/room-helpers";
import { RoomStammdatenTab } from "./room-stammdaten-tab";

// The form remounts (new key) after save, cancel and room switch so a stale
// draft never survives. The stand-in records each mount with a nonce.
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
    initialData,
  }: {
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
    initialData?: { name?: string };
  }) => {
    const nonce = String(Math.random());
    const mounts = (globalThis as unknown as { __formMounts?: string[] })
      .__formMounts;
    mounts?.push(nonce);
    return (
      <div data-testid="database-form" data-nonce={nonce}>
        <span data-testid="form-name">{initialData?.name}</span>
        <button
          type="button"
          onClick={() => void onSubmit({ name: "Updated Room" })}
        >
          Save
        </button>
        <button type="button" onClick={onCancel}>
          Cancel
        </button>
      </div>
    );
  },
}));

const room: Room = {
  id: "1",
  name: "Raum 101",
  category: "Normaler Raum",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 24,
  color: "#4F46E5",
  isOccupied: false,
};

function getCurrentNonce(): string | null {
  return screen.getByTestId("database-form").getAttribute("data-nonce") ?? null;
}

describe("RoomStammdatenTab", () => {
  beforeEach(() => {
    (globalThis as unknown as { __formMounts?: string[] }).__formMounts = [];
  });

  it("shows the occupancy summary and the form for the room", () => {
    render(
      <RoomStammdatenTab
        room={{
          ...room,
          isOccupied: true,
          activityName: "Fußball AG",
          groupName: "Füchse",
        }}
        showOccupancy
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(screen.getByText("24 Plätze")).toBeInTheDocument();
    expect(screen.getByText("Fußball AG")).toBeInTheDocument();
    expect(screen.getByText("Füchse")).toBeInTheDocument();
    expect(screen.queryByText("Systemraum")).not.toBeInTheDocument();
    expect(screen.getByTestId("form-name")).toHaveTextContent("Raum 101");
  });

  it("marks system rooms", () => {
    render(
      <RoomStammdatenTab
        room={{ ...room, name: "Schulhof" }}
        showOccupancy
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByText("Systemraum")).toBeInTheDocument();
  });

  it("remounts the form after a successful save so stale field state is dropped", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<RoomStammdatenTab room={room} showOccupancy onSave={onSave} />);

    const before = getCurrentNonce();
    expect(before).not.toBeNull();

    await act(async () => {
      fireEvent.click(screen.getByText("Save"));
    });

    await waitFor(() => {
      expect(getCurrentNonce()).not.toBe(before);
    });
    expect(onSave).toHaveBeenCalledWith({ name: "Updated Room" });
  });

  it("remounts the form when the user cancels (so unsaved field edits are discarded)", () => {
    render(<RoomStammdatenTab room={room} showOccupancy onSave={vi.fn()} />);

    const before = getCurrentNonce();
    fireEvent.click(screen.getByText("Cancel"));

    expect(getCurrentNonce()).not.toBe(before);
  });

  it("remounts the form when the room changes (so prior values do not leak)", () => {
    const { rerender } = render(
      <RoomStammdatenTab room={room} showOccupancy onSave={vi.fn()} />,
    );

    const before = getCurrentNonce();
    rerender(
      <RoomStammdatenTab
        room={{ ...room, id: "2", name: "Raum 202" }}
        showOccupancy
        onSave={vi.fn()}
      />,
    );

    expect(getCurrentNonce()).not.toBe(before);
    expect(screen.getByTestId("form-name")).toHaveTextContent("Raum 202");
  });

  it("hides occupancy data when the tenant does not track room occupancy", () => {
    render(
      <RoomStammdatenTab
        room={{
          ...room,
          isOccupied: true,
          activityName: "Fußball AG",
          groupName: "Füchse",
        }}
        showOccupancy={false}
        onSave={vi.fn()}
      />,
    );

    expect(screen.queryByText("Belegung")).not.toBeInTheDocument();
    expect(screen.queryByText("Belegt")).not.toBeInTheDocument();
    expect(screen.queryByText("Fußball AG")).not.toBeInTheDocument();
    expect(screen.getByTestId("form-name")).toHaveTextContent("Raum 101");
  });
});
