import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ActivityDetailModal } from "./activity-detail-modal";

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
    entityName,
    entityType,
    onDeleteClick,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    entityName: string;
    entityType: string;
    onDeleteClick?: () => void;
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

const mockActivity = {
  id: "1",
  name: "Chess Club",
  max_participant: 12,
  is_open_ags: true,
  supervisor_id: "sup-1",
  supervisor_name: "John Doe",
  ag_category_id: "cat-1",
  category_name: "Games",
  created_at: new Date(),
  updated_at: new Date(),
};

describe("ActivityDetailModal", () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    activity: mockActivity,
    onEdit: vi.fn(),
    onDelete: vi.fn(),
  };

  it("renders nothing when activity is null", () => {
    const { container } = render(
      <ActivityDetailModal {...defaultProps} activity={null} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders modal when open with activity", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays activity name", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    // Activity name appears in header and entity-name
    expect(screen.getAllByText("Chess Club").length).toBeGreaterThan(0);
  });

  it("displays activity initials in header", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getByText("CH")).toBeInTheDocument();
  });

  it("displays category name", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getAllByText("Games").length).toBeGreaterThan(0);
  });

  it("displays max participants", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getByText("12")).toBeInTheDocument();
  });

  it("displays supervisor name", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("shows loading state when loading is true", () => {
    render(<ActivityDetailModal {...defaultProps} loading={true} />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    expect(screen.queryByText("Chess Club")).not.toBeInTheDocument();
  });

  it("shows no category placeholder when category is missing", () => {
    const activityNoCategory = { ...mockActivity, category_name: undefined };
    render(
      <ActivityDetailModal {...defaultProps} activity={activityNoCategory} />,
    );

    expect(screen.getAllByText("Keine Kategorie").length).toBeGreaterThan(0);
  });

  it("hides supervisor section when supervisor name is missing", () => {
    const activityNoSupervisor = { ...mockActivity, supervisor_name: undefined };
    render(
      <ActivityDetailModal {...defaultProps} activity={activityNoSupervisor} />,
    );

    expect(screen.queryByText("Hauptbetreuer")).not.toBeInTheDocument();
  });

  it("calls onEdit when edit button is clicked", () => {
    const onEdit = vi.fn();
    render(<ActivityDetailModal {...defaultProps} onEdit={onEdit} />);

    fireEvent.click(screen.getByTestId("edit-btn"));

    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("calls onDeleteClick when provided", () => {
    const onDeleteClick = vi.fn();
    render(
      <ActivityDetailModal {...defaultProps} onDeleteClick={onDeleteClick} />,
    );

    fireEvent.click(screen.getByTestId("delete-btn"));

    expect(onDeleteClick).toHaveBeenCalledTimes(1);
  });

  it("passes correct entity info to DetailModalActions", () => {
    render(<ActivityDetailModal {...defaultProps} />);

    expect(screen.getByTestId("entity-name")).toHaveTextContent("Chess Club");
    expect(screen.getByTestId("entity-type")).toHaveTextContent("Aktivität");
  });

  it("uses AG as fallback initials when name is undefined", () => {
    const activityNoName = { ...mockActivity, name: undefined as unknown as string };
    render(<ActivityDetailModal {...defaultProps} activity={activityNoName} />);

    expect(screen.getByText("AG")).toBeInTheDocument();
  });
});
