import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import SearchableGuardianSelect from "./searchable-guardian-select";
import type { GuardianSearchResult } from "@/lib/guardian-helpers";

// Mock the guardian-api module
vi.mock("@/lib/guardian-api", () => ({
  searchGuardiansWithStudents: vi.fn(),
}));

// Import mocked function after mock setup
import { searchGuardiansWithStudents } from "@/lib/guardian-api";
const mockSearchGuardiansWithStudents = vi.mocked(searchGuardiansWithStudents);

describe("SearchableGuardianSelect", () => {
  const mockOnSelect = vi.fn();

  const mockGuardians: GuardianSearchResult[] = [
    {
      id: "1",
      firstName: "Anna",
      lastName: "Müller",
      email: "anna.mueller@example.com",
      phone: "030-12345678",
      students: [
        {
          studentId: "101",
          firstName: "Max",
          lastName: "Müller",
          schoolClass: "1a",
        },
        {
          studentId: "102",
          firstName: "Lisa",
          lastName: "Müller",
          schoolClass: "3b",
        },
      ],
    },
    {
      id: "2",
      firstName: "Peter",
      lastName: "Schmidt",
      email: "peter.schmidt@example.com",
      students: [],
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders search input", () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    expect(
      screen.getByPlaceholderText("Name oder E-Mail eingeben (min. 2 Zeichen)"),
    ).toBeInTheDocument();
  });

  it("renders help text", () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    expect(
      screen.getByText(/Suchen Sie nach dem Namen oder der E-Mail-Adresse/),
    ).toBeInTheDocument();
  });

  it("does not search when query is too short", async () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "A" } });

    // Advance timers past debounce
    vi.advanceTimersByTime(500);

    expect(mockSearchGuardiansWithStudents).not.toHaveBeenCalled();
  });

  it("searches when query is at least 2 characters", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "An" } });

    // Advance timers past debounce
    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(mockSearchGuardiansWithStudents).toHaveBeenCalledWith(
        "An",
        undefined,
      );
    });
  });

  it("passes excludeStudentId to search function", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
        excludeStudentId="123"
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Anna" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(mockSearchGuardiansWithStudents).toHaveBeenCalledWith(
        "Anna",
        "123",
      );
    });
  });

  it("displays search results in dropdown", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
      expect(screen.getByText("Peter Schmidt")).toBeInTheDocument();
    });
  });

  it("shows email in search results", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("anna.mueller@example.com")).toBeInTheDocument();
    });
  });

  it("shows linked students (siblings) in results", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText(/Geschwister:/)).toBeInTheDocument();
      expect(screen.getByText(/Max \(1a\)/)).toBeInTheDocument();
    });
  });

  it("shows 'no students' message for guardians without students", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(
        screen.getByText("Noch keinem Schüler zugeordnet"),
      ).toBeInTheDocument();
    });
  });

  it("calls onSelect when guardian is clicked", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Anna Müller"));

    expect(mockOnSelect).toHaveBeenCalledWith(mockGuardians[0]);
  });

  it("clears input and results after selection", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Anna Müller"));

    expect(input.value).toBe("");
    expect(screen.queryByText("Anna Müller")).not.toBeInTheDocument();
  });

  it("shows clear button when query exists", async () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );

    // Initially no clear button
    expect(screen.queryByLabelText("Suche leeren")).not.toBeInTheDocument();

    fireEvent.change(input, { target: { value: "Test" } });

    // Clear button should appear
    expect(screen.getByLabelText("Suche leeren")).toBeInTheDocument();
  });

  it("clears search when clear button is clicked", async () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "Test" } });

    fireEvent.click(screen.getByLabelText("Suche leeren"));

    expect(input.value).toBe("");
  });

  it("shows no results message when search returns empty", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue([]);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "NotFound" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Erziehungsberechtigten gefunden"),
      ).toBeInTheDocument();
    });
  });

  it("shows error message on search failure", async () => {
    mockSearchGuardiansWithStudents.mockRejectedValue(new Error("API Error"));

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("API Error")).toBeInTheDocument();
    });
  });

  it("shows generic error message for non-Error exceptions", async () => {
    mockSearchGuardiansWithStudents.mockRejectedValue("Unknown error");

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    await waitFor(() => {
      expect(screen.getByText("Fehler bei der Suche")).toBeInTheDocument();
    });
  });

  it("disables input when disabled prop is true", () => {
    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
        disabled={true}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    expect(input).toBeDisabled();
  });

  it("debounces search requests", async () => {
    mockSearchGuardiansWithStudents.mockResolvedValue(mockGuardians);

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );

    // Type multiple characters quickly
    fireEvent.change(input, { target: { value: "An" } });
    fireEvent.change(input, { target: { value: "Ann" } });
    fireEvent.change(input, { target: { value: "Anna" } });

    // Advance timers but not past debounce
    vi.advanceTimersByTime(200);

    // Search should not have been called yet
    expect(mockSearchGuardiansWithStudents).not.toHaveBeenCalled();

    // Advance past debounce
    vi.advanceTimersByTime(150);

    // Should only be called once with final value
    await waitFor(() => {
      expect(mockSearchGuardiansWithStudents).toHaveBeenCalledTimes(1);
      expect(mockSearchGuardiansWithStudents).toHaveBeenCalledWith(
        "Anna",
        undefined,
      );
    });
  });

  it("shows loading spinner during search", async () => {
    let resolveSearch: (value: GuardianSearchResult[]) => void;
    mockSearchGuardiansWithStudents.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSearch = resolve;
        }),
    );

    render(
      <SearchableGuardianSelect
        onSelect={mockOnSelect}
      />,
    );

    const input = screen.getByPlaceholderText(
      "Name oder E-Mail eingeben (min. 2 Zeichen)",
    );
    fireEvent.change(input, { target: { value: "Test" } });

    vi.advanceTimersByTime(350);

    // Check that the search was initiated
    await waitFor(() => {
      expect(mockSearchGuardiansWithStudents).toHaveBeenCalled();
    });

    // Resolve the search
    resolveSearch!(mockGuardians);

    // Results should appear
    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
    });
  });
});
