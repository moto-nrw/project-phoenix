import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Room } from "~/lib/room-helpers";
import { buildRoomSubtitle, RoomsList } from "./rooms-list";

vi.mock("~/components/database/database-list-layout", () => ({
  DatabaseListLayout: (props: { children: React.ReactNode }) => (
    <div data-testid="list-layout">{props.children}</div>
  ),
}));

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const room: Room = {
  id: "1",
  name: "Raum 101",
  category: "Normaler Raum",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 24,
  isOccupied: false,
};

describe("RoomsList", () => {
  it("links every room to the room page", () => {
    render(
      <RoomsList
        groupDefinitions={[
          {
            id: "Hauptgebäude",
            title: "Hauptgebäude",
            items: [room, { ...room, id: "2", name: "Raum 202" }],
          },
        ]}
        objectHref={(entry) => `/rooms/${entry.id}?from=%2Fdatabase%2Frooms`}
      />,
    );

    expect(screen.getByRole("link", { name: /Raum 101/ })).toHaveAttribute(
      "href",
      "/rooms/1?from=%2Fdatabase%2Frooms",
    );
    expect(screen.getByRole("link", { name: /Raum 202/ })).toHaveAttribute(
      "href",
      "/rooms/2?from=%2Fdatabase%2Frooms",
    );
  });

  it("shows the empty state when no group has entries", () => {
    render(<RoomsList groupDefinitions={[]} objectHref={() => "/rooms/0"} />);

    expect(screen.getByText("Keine Räume gefunden.")).toBeInTheDocument();
  });
});

describe("buildRoomSubtitle", () => {
  it("renders 'Belegt' as a leading marker for occupied rooms", () => {
    expect(buildRoomSubtitle({ ...room, isOccupied: true })).toBe(
      "Belegt · Hauptgebäude · Etage 1 · Normaler Raum",
    );
  });

  it("falls back to a dash when building, floor and category are missing", () => {
    expect(
      buildRoomSubtitle({
        ...room,
        building: undefined,
        floor: undefined,
        category: undefined,
      }),
    ).toBe("–");
  });

  it("uses only the building when floor is undefined and only the floor when building is undefined", () => {
    expect(
      buildRoomSubtitle({ ...room, floor: undefined, category: undefined }),
    ).toBe("Hauptgebäude");
    expect(
      buildRoomSubtitle({ ...room, building: undefined, category: undefined }),
    ).toBe("Etage 1");
  });
});
