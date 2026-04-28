/**
 * Tests for RoomCreateModal
 * Tests the rendering and functionality of the room creation modal
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomCreateModal } from "./room-create-modal";

const mockPickRoomColor = vi.fn();
const mockUseSWR = vi.fn();
const mockFetchRooms = vi.fn();

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h1>{title}</h1>
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock DatabaseForm component
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
    submitLabel,
    initialData,
  }: {
    onSubmit: (data: unknown) => Promise<void>;
    onCancel: () => void;
    submitLabel: string;
    initialData: Record<string, unknown>;
  }) => (
    <div data-testid="database-form">
      <span data-testid="initial-data">{JSON.stringify(initialData)}</span>
      <button onClick={() => onSubmit({ name: "Test Room" })}>
        {submitLabel}
      </button>
      <button onClick={onCancel}>Cancel</button>
    </div>
  ),
}));

// Mock roomsConfig and configToFormSection
vi.mock("@/lib/database/configs/rooms.config", () => ({
  roomsConfig: {
    labels: {
      createModalTitle: "Neuer Raum",
    },
    theme: {
      primary: "#6366f1",
    },
    form: {
      sections: [
        {
          title: "Grundinformationen",
          fields: [
            {
              name: "name",
              label: "Raumname",
              type: "text",
              required: true,
            },
          ],
        },
      ],
      defaultValues: {},
    },
  },
}));

vi.mock("@/lib/database/types", () => ({
  configToFormSection: (section: unknown) => section,
}));

vi.mock("~/lib/location-helper", () => ({
  pickRoomColor: (...args: unknown[]) => mockPickRoomColor(...args),
}));

vi.mock("swr", () => ({
  default: (...args: unknown[]) => mockUseSWR(...args),
}));

vi.mock("~/lib/rooms-api", () => ({
  fetchRooms: (...args: unknown[]) => mockFetchRooms(...args),
}));

describe("RoomCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockPickRoomColor.mockReset();
    mockPickRoomColor
      .mockReturnValueOnce("#111111")
      .mockReturnValueOnce("#222222");
    mockUseSWR.mockReturnValue({ data: [], isLoading: false });
    mockFetchRooms.mockResolvedValue([]);
  });

  it("renders nothing when modal is closed", () => {
    render(
      <RoomCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", async () => {
    render(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Neuer Raum")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders DatabaseForm when not loading", async () => {
    render(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
      expect(screen.getByText("Erstellen")).toBeInTheDocument();
    });
  });

  it("waits for rooms before choosing the initial default color on cold open", async () => {
    let swrState:
      | { data: Array<{ color: string }>; isLoading: false }
      | { data: undefined; isLoading: true } = {
      data: undefined,
      isLoading: true,
    };

    mockUseSWR.mockImplementation(() => swrState);

    const { rerender } = render(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(
      screen.getByText("Formular wird vorbereitet..."),
    ).toBeInTheDocument();
    expect(mockPickRoomColor).not.toHaveBeenCalled();

    swrState = { data: [{ color: "#34D399" }], isLoading: false };
    rerender(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });

    expect(screen.getByTestId("initial-data")).toHaveTextContent("#111111");
    expect(mockPickRoomColor).toHaveBeenCalledWith(["#34D399"]);
  });

  it("keeps initial data stable after rooms revalidate", async () => {
    let swrState = {
      data: [{ color: "#34D399" }],
      isLoading: false,
    };

    mockUseSWR.mockImplementation(() => swrState);

    const { rerender } = render(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });

    expect(screen.getByTestId("initial-data")).toHaveTextContent("#111111");

    swrState = {
      data: [{ color: "#34D399" }, { color: "#14B8A6" }],
      isLoading: false,
    };
    rerender(
      <RoomCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });

    expect(screen.getByTestId("initial-data")).toHaveTextContent("#111111");
    expect(screen.getByTestId("initial-data")).not.toHaveTextContent("#222222");
  });
});
