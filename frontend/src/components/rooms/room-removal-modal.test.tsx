import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  fetchRoomRemovalImpact: vi.fn(),
  removeRoom: vi.fn(),
}));

vi.mock("~/lib/room-removal-api", () => ({
  fetchRoomRemovalImpact: mocks.fetchRoomRemovalImpact,
  removeRoom: mocks.removeRoom,
}));

import { RoomRemovalModal } from "./room-removal-modal";
import type { RoomRemovalImpact } from "~/lib/room-removal-api";

const unused: RoomRemovalImpact = {
  outcome: "delete",
  active_session: false,
  care_offering: false,
  references: {
    activity_groups: 0,
    upcoming_blocks: 0,
    education_groups: 0,
    devices: 0,
    past_blocks: 0,
    archived_activity_groups: 0,
    has_session_history: false,
  },
};

function renderModal(onRemoved = vi.fn()) {
  render(
    <RoomRemovalModal
      isOpen
      roomId="7"
      roomName="Igelraum"
      onClose={vi.fn()}
      onRemoved={onRemoved}
    />,
  );
  return onRemoved;
}

describe("RoomRemovalModal (#6845)", () => {
  beforeEach(() => {
    mocks.fetchRoomRemovalImpact.mockReset();
    mocks.removeRoom.mockReset();
  });

  it("names what still uses the room and keeps it", async () => {
    mocks.fetchRoomRemovalImpact.mockResolvedValue({
      ...unused,
      outcome: "blocked",
      references: { ...unused.references, devices: 1, upcoming_blocks: 3 },
    });
    renderModal();

    expect(
      await screen.findByRole("heading", { name: "Raum wird noch genutzt" }),
    ).toBeVisible();
    expect(screen.getByText("Gerät steht hier")).toBeVisible();
    expect(screen.getByText("Kommende Termine")).toBeVisible();
    expect(screen.getByText("3")).toBeVisible();
    expect(screen.getByRole("button", { name: /löschen/i })).toBeDisabled();
    expect(mocks.removeRoom).not.toHaveBeenCalled();
  });

  it("archives a room that history still names", async () => {
    mocks.fetchRoomRemovalImpact.mockResolvedValue({
      ...unused,
      outcome: "archive",
      references: {
        ...unused.references,
        past_blocks: 12,
        has_session_history: true,
      },
    });
    mocks.removeRoom.mockResolvedValue("archive");
    const onRemoved = renderModal();

    expect(
      await screen.findByRole("heading", { name: "Raum archivieren?" }),
    ).toBeVisible();
    expect(screen.getByText("Bleibt im Verlauf erhalten")).toBeVisible();
    expect(screen.getByText("Vergangene Termine")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Ja, archivieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));

    await waitFor(() => expect(onRemoved).toHaveBeenCalledWith("archive"));
    expect(mocks.removeRoom).toHaveBeenCalledWith("7");
  });

  it("deletes a room nothing uses", async () => {
    mocks.fetchRoomRemovalImpact.mockResolvedValue(unused);
    renderModal();

    expect(await screen.findByText(/wird nirgends genutzt/)).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Raum löschen?" }),
    ).toBeVisible();
  });
});
