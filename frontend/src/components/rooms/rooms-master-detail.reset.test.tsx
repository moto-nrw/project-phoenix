import "@testing-library/jest-dom/vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useEffect, useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

// The form mock embeds a per-mount nonce so the test can assert that
// `key={formResetKey}` actually remounts the form (rather than only
// rerendering it) on save and cancel.
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
  }: {
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
  }) => {
    const [nonce] = useState(() => Math.random().toString(36).slice(2));
    useEffect(() => {
      const list = (globalThis as unknown as { __formMounts?: string[] })
        .__formMounts;
      list?.push(nonce);
    }, [nonce]);
    return (
      <div data-testid="database-form" data-nonce={nonce}>
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

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { RoomsMasterDetail } from "./rooms-master-detail";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Room } from "@/lib/room-helpers";

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

const groups: GroupDefinition<Room>[] = [
  { id: "__flat__", title: "Alle Räume (1)", items: [room] },
];

describe("RoomsMasterDetail form reset wiring", () => {
  beforeEach(() => {
    (globalThis as unknown as { __formMounts?: string[] }).__formMounts = [];
  });

  function getMounts(): string[] {
    return (
      (globalThis as unknown as { __formMounts?: string[] }).__formMounts ?? []
    );
  }

  function getCurrentNonce(): string | null {
    return (
      screen.getByTestId("database-form").getAttribute("data-nonce") ?? null
    );
  }

  it("remounts the form after a successful save so stale field state is dropped", async () => {
    const onSaveRoom = vi.fn().mockResolvedValue(undefined);
    render(
      <RoomsMasterDetail
        groupDefinitions={groups}
        selectedId="1"
        selectedRoom={room}
        onSelect={vi.fn()}
        onSaveRoom={onSaveRoom}
        onDeleteClick={vi.fn()}
      />,
    );

    const before = getCurrentNonce();
    expect(before).not.toBeNull();
    expect(getMounts()).toHaveLength(1);

    await act(async () => {
      fireEvent.click(screen.getByText("Save"));
    });

    await waitFor(() => {
      expect(getCurrentNonce()).not.toBe(before);
    });

    expect(onSaveRoom).toHaveBeenCalledWith({ name: "Updated Room" });
    expect(getMounts().length).toBeGreaterThanOrEqual(2);
  });

  it("remounts the form when the user cancels (so unsaved field edits are discarded)", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={groups}
        selectedId="1"
        selectedRoom={room}
        onSelect={vi.fn()}
        onSaveRoom={vi.fn()}
        onDeleteClick={vi.fn()}
      />,
    );

    const before = getCurrentNonce();
    fireEvent.click(screen.getByText("Cancel"));
    const after = getCurrentNonce();

    expect(after).not.toBe(before);
  });

  it("remounts the form when switching between rooms (so prior values do not leak across selections)", () => {
    const otherRoom: Room = { ...room, id: "2", name: "Raum 202" };
    const { rerender } = render(
      <RoomsMasterDetail
        groupDefinitions={[
          { id: "__flat__", title: "Alle Räume (2)", items: [room, otherRoom] },
        ]}
        selectedId="1"
        selectedRoom={room}
        onSelect={vi.fn()}
        onSaveRoom={vi.fn()}
        onDeleteClick={vi.fn()}
      />,
    );

    const before = getCurrentNonce();
    rerender(
      <RoomsMasterDetail
        groupDefinitions={[
          { id: "__flat__", title: "Alle Räume (2)", items: [room, otherRoom] },
        ]}
        selectedId="2"
        selectedRoom={otherRoom}
        onSelect={vi.fn()}
        onSaveRoom={vi.fn()}
        onDeleteClick={vi.fn()}
      />,
    );
    const after = getCurrentNonce();

    expect(after).not.toBe(before);
  });
});
