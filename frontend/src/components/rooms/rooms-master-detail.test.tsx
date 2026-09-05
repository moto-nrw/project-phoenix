import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    sections,
    onSubmit,
    onCancel,
  }: {
    sections: Array<{
      fields: Array<{ name: string; disabled?: boolean; helperText?: string }>;
    }>;
    onSubmit: (data: { name: string }) => Promise<void>;
    onCancel: () => void;
  }) => {
    const nameField = sections
      .flatMap((section) => section.fields)
      .find((field) => field.name === "name");

    return (
      <div data-testid="database-form">
        {nameField?.disabled ? (
          <span data-testid="name-disabled">{nameField.helperText}</span>
        ) : null}
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

const regularRoom: Room = {
  id: "1",
  name: "Raum 101",
  category: "Normaler Raum",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 24,
  color: "#4F46E5",
  isOccupied: false,
};

const systemRoom: Room = {
  ...regularRoom,
  id: "2",
  name: "Schulhof",
};

function flatGroup(rooms: Room[]): GroupDefinition<Room>[] {
  if (rooms.length === 0) return [];
  return [
    {
      id: "__flat__",
      title: `Alle Räume (${rooms.length})`,
      items: rooms,
    },
  ];
}

describe("RoomsMasterDetail", () => {
  const onSelect = vi.fn();
  const onSaveRoom = vi.fn();
  const onDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([regularRoom])}
        selectedId={null}
        selectedRoom={null}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.queryByTestId("database-form")).not.toBeInTheDocument();
    expect(screen.getByText("Raum 101")).toBeInTheDocument();
  });

  it("renders the selected room detail form and save path", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([regularRoom])}
        selectedId="1"
        selectedRoom={regularRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getAllByText("Raum 101")).toHaveLength(2);
    expect(screen.getByTestId("database-form")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Save"));
    expect(onSaveRoom).toHaveBeenCalledWith({ name: "Updated Room" });
  });

  it("hides the delete action for system rooms and disables the name field", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([systemRoom])}
        selectedId="2"
        selectedRoom={systemRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
    expect(screen.getByTestId("name-disabled")).toHaveTextContent(
      "Systemraum: Name kann nicht geändert werden",
    );
  });

  it("passes delete clicks through for regular rooms", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([regularRoom])}
        selectedId="1"
        selectedRoom={regularRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();
  });

  it("renders the active activity and group when the room is occupied", () => {
    const occupiedRoom: Room = {
      ...regularRoom,
      isOccupied: true,
      activityName: "Mathe AG",
      groupName: "Gruppe Rot",
    };
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([occupiedRoom])}
        selectedId="1"
        selectedRoom={occupiedRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getByText("Aktivität")).toBeInTheDocument();
    expect(screen.getByText("Mathe AG")).toBeInTheDocument();
    expect(screen.getByText("Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Gruppe Rot")).toBeInTheDocument();
  });

  it("renders the group titles supplied by the page and a Stammdaten tab", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={[
          {
            id: "Hauptgebäude",
            title: "Hauptgebäude (1)",
            items: [regularRoom],
          },
          {
            id: "__no_building__",
            title: "Ohne Gebäude (1)",
            items: [systemRoom],
          },
        ]}
        selectedId="1"
        selectedRoom={regularRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    expect(screen.getByText("Hauptgebäude (1)")).toBeInTheDocument();
    expect(screen.getByText("Ohne Gebäude (1)")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("forwards a list-row click to onSelect with the room's id", () => {
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([regularRoom])}
        selectedId={null}
        selectedRoom={null}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    fireEvent.click(screen.getByText("Raum 101"));
    expect(onSelect).toHaveBeenCalledWith("1");
  });

  it("renders 'Belegt' as a leading subtitle marker for occupied rooms", () => {
    const occupiedRoom: Room = { ...regularRoom, isOccupied: true };
    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([occupiedRoom])}
        selectedId={null}
        selectedRoom={null}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    // The subtitle must start with "Belegt · ..." so admins can spot occupied
    // rooms at a glance from the list.
    expect(screen.getByText(/^Belegt · /)).toBeInTheDocument();
  });

  it("falls back to a 'no location' subtitle when building and floor are missing", () => {
    const locationlessRoom: Room = {
      ...regularRoom,
      id: "9",
      name: "Phantomraum",
      building: undefined,
      floor: undefined,
      category: undefined,
    };

    render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([locationlessRoom])}
        selectedId="9"
        selectedRoom={locationlessRoom}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    // Detail header subtitle: buildRoomLocation returns
    // "Kein Standort hinterlegt" when both building and floor are absent.
    expect(screen.getByText("Kein Standort hinterlegt")).toBeInTheDocument();
    // List-item subtitle: buildRoomSubtitle falls back to "–" when no location
    // and no category exists.
    expect(screen.getByText("–")).toBeInTheDocument();
  });

  it("uses only the building when floor is undefined and only the floor when building is undefined", () => {
    const buildingOnly: Room = {
      ...regularRoom,
      id: "10",
      name: "Nebengebäude-Raum",
      building: "Nebengebäude",
      floor: undefined,
    };
    const floorOnly: Room = {
      ...regularRoom,
      id: "11",
      name: "Floor-Only-Raum",
      building: undefined,
      floor: 2,
    };

    const { rerender } = render(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([buildingOnly])}
        selectedId="10"
        selectedRoom={buildingOnly}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    // Detail header subtitle includes the building name without a separator.
    expect(screen.getByText("Nebengebäude")).toBeInTheDocument();

    rerender(
      <RoomsMasterDetail
        groupDefinitions={flatGroup([floorOnly])}
        selectedId="11"
        selectedRoom={floorOnly}
        onSelect={onSelect}
        onSaveRoom={onSaveRoom}
        onDeleteClick={onDeleteClick}
      />,
    );

    // formatFloor renders the floor number — assert it appears in the subtitle
    // area without forcing a specific localized format.
    expect(screen.getAllByText(/2/).length).toBeGreaterThan(0);
  });
});
