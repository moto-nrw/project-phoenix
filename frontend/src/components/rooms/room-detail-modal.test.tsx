import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { RoomDetailModal } from "./room-detail-modal";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button data-testid="modal-close" onClick={onClose}>
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

vi.mock("~/components/ui/detail-modal-actions", () => ({
  DetailModalActions: ({
    onEdit,
    onDelete,
    onDeleteClick,
    entityName,
    entityType,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick?: () => void;
    entityName: string;
    entityType: string;
  }) => (
    <div data-testid="detail-actions">
      <span data-testid="entity-name">{entityName}</span>
      <span data-testid="entity-type">{entityType}</span>
      <button data-testid="edit-btn" onClick={onEdit}>
        Edit
      </button>
      <button data-testid="delete-btn" onClick={onDeleteClick ?? onDelete}>
        Delete
      </button>
    </div>
  ),
}));

vi.mock("~/components/ui/modal-loading-state", () => ({
  ModalLoadingState: ({ accentColor }: { accentColor: string }) => (
    <div data-testid="loading-state">
      <span data-testid="accent-color">{accentColor}</span>
      Loading...
    </div>
  ),
}));

vi.mock("~/components/ui/detail-modal-components", () => ({
  DataField: ({
    label,
    children,
    fullWidth,
  }: {
    label: string;
    children: React.ReactNode;
    fullWidth?: boolean;
  }) => (
    <div data-testid={`field-${label.toLowerCase().replace(/\s/g, "-")}`}>
      <span data-testid="label">{label}</span>
      <span data-testid="value" data-fullwidth={fullWidth}>
        {children}
      </span>
    </div>
  ),
  DataGrid: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="data-grid">{children}</div>
  ),
  InfoSection: ({
    title,
    children,
  }: {
    title: string;
    icon: React.ReactNode;
    accentColor: string;
    children: React.ReactNode;
  }) => (
    <div data-testid={`section-${title.toLowerCase().replace(/\s/g, "-")}`}>
      <span data-testid="section-title">{title}</span>
      {children}
    </div>
  ),
  DetailIcons: {
    building: <span>BuildingIcon</span>,
  },
}));

vi.mock("@/lib/room-helpers", () => ({
  formatFloor: (floor: number | undefined) => {
    if (floor === undefined) return "";
    if (floor === 0) return "Erdgeschoss";
    if (floor > 0) return `${floor}. OG`;
    return `${Math.abs(floor)}. UG`;
  },
}));

const mockRoom = {
  id: "1",
  name: "Raum 101",
  category: "Klassenzimmer",
  building: "Hauptgebäude",
  floor: 1,
  isOccupied: false,
  activityName: undefined,
  groupName: undefined,
};

describe("RoomDetailModal", () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    room: mockRoom,
    onEdit: vi.fn(),
    onDelete: vi.fn(),
  };

  it("renders nothing when room is null", () => {
    const { container } = render(
      <RoomDetailModal {...defaultProps} room={null} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders modal when open with room", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays room name in header", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getAllByText("Raum 101").length).toBeGreaterThan(0);
  });

  it("displays room initial in avatar", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByText("R")).toBeInTheDocument();
  });

  it("shows loading state when loading is true", () => {
    render(<RoomDetailModal {...defaultProps} loading={true} />);

    expect(screen.getByTestId("loading-state")).toBeInTheDocument();
    expect(screen.getByTestId("accent-color")).toHaveTextContent("indigo");
  });

  it("displays room details section", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("section-raumdetails")).toBeInTheDocument();
  });

  it("displays category field", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-kategorie")).toBeInTheDocument();
    expect(screen.getByText("Klassenzimmer")).toBeInTheDocument();
  });

  it("displays building field", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-gebäude")).toBeInTheDocument();
    expect(screen.getAllByText("Hauptgebäude").length).toBeGreaterThan(0);
  });

  it("displays floor field", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-etage")).toBeInTheDocument();
    expect(screen.getAllByText("1. OG").length).toBeGreaterThan(0);
  });

  it("displays status as Frei when not occupied", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-status")).toBeInTheDocument();
    expect(screen.getByText("Frei")).toBeInTheDocument();
  });

  it("displays status as Belegt when occupied", () => {
    const occupiedRoom = { ...mockRoom, isOccupied: true };
    render(<RoomDetailModal {...defaultProps} room={occupiedRoom} />);

    expect(screen.getByText("Belegt")).toBeInTheDocument();
  });

  it("displays activity name when present", () => {
    const roomWithActivity = { ...mockRoom, activityName: "Schach AG" };
    render(<RoomDetailModal {...defaultProps} room={roomWithActivity} />);

    expect(screen.getByTestId("field-aktivität")).toBeInTheDocument();
    expect(screen.getByText("Schach AG")).toBeInTheDocument();
  });

  it("displays group name when present", () => {
    const roomWithGroup = { ...mockRoom, groupName: "Gruppe 1a" };
    render(<RoomDetailModal {...defaultProps} room={roomWithGroup} />);

    expect(screen.getByTestId("field-gruppe")).toBeInTheDocument();
    expect(screen.getByText("Gruppe 1a")).toBeInTheDocument();
  });

  it("shows 'Nicht angegeben' when category is missing", () => {
    const roomNoCategory = { ...mockRoom, category: undefined };
    render(<RoomDetailModal {...defaultProps} room={roomNoCategory} />);

    expect(screen.getAllByText("Nicht angegeben").length).toBeGreaterThan(0);
  });

  it("calls onEdit when edit button clicked", () => {
    const onEdit = vi.fn();
    render(<RoomDetailModal {...defaultProps} onEdit={onEdit} />);

    fireEvent.click(screen.getByTestId("edit-btn"));

    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("calls onDeleteClick when provided", () => {
    const onDeleteClick = vi.fn();
    render(<RoomDetailModal {...defaultProps} onDeleteClick={onDeleteClick} />);

    fireEvent.click(screen.getByTestId("delete-btn"));

    expect(onDeleteClick).toHaveBeenCalledTimes(1);
  });

  it("passes correct entity info to DetailModalActions", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByTestId("entity-name")).toHaveTextContent("Raum 101");
    expect(screen.getByTestId("entity-type")).toHaveTextContent("Raum");
  });

  it("displays building and floor in subtitle", () => {
    render(<RoomDetailModal {...defaultProps} />);

    expect(screen.getByText("Hauptgebäude, 1. OG")).toBeInTheDocument();
  });

  it("shows only building when floor is undefined", () => {
    const roomNoFloor = { ...mockRoom, floor: undefined };
    render(<RoomDetailModal {...defaultProps} room={roomNoFloor} />);

    // Building should still be visible in subtitle
    const buildingTexts = screen.getAllByText("Hauptgebäude");
    expect(buildingTexts.length).toBeGreaterThan(0);
  });
});
