import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import GuardianPickerPanel from "./guardian-picker-panel";
import type { Guardian } from "@/lib/guardian-helpers";

const mockSearchGuardians = vi.fn();

// The panel imports both the search function and the result-limit constant from
// guardian-api; the mock must supply both. Keep the limit equal to the real
// value (50) so the truncation-hint assertions exercise the real threshold.
vi.mock("@/lib/guardian-api", () => ({
  searchGuardians: (query: string) => mockSearchGuardians(query),
  GUARDIAN_PICKER_RESULT_LIMIT: 50,
}));

function makeGuardian(i: number): Guardian {
  return {
    id: String(i),
    firstName: `First${i}`,
    lastName: `Last${i}`,
    email: `g${i}@example.com`,
    phoneNumbers: [],
    preferredContactMethod: "",
    languagePreference: "",
    hasAccount: false,
  };
}

describe("GuardianPickerPanel truncation hint", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the narrow-your-search hint when results hit the cap", async () => {
    // Exactly the cap (50) → more may exist, so the hint must appear.
    mockSearchGuardians.mockResolvedValue(
      Array.from({ length: 50 }, (_, i) => makeGuardian(i)),
    );

    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "mueller" },
    });

    await waitFor(() => {
      expect(
        screen.getByText("Viele Treffer – bitte den Namen weiter eingrenzen."),
      ).toBeInTheDocument();
    });
  });

  it("does not show the hint when results are below the cap", async () => {
    mockSearchGuardians.mockResolvedValue(
      Array.from({ length: 3 }, (_, i) => makeGuardian(i)),
    );

    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "schmidt" },
    });

    // Wait for the results to render, then assert the hint is absent.
    await waitFor(() => {
      expect(screen.getByText("First0 Last0")).toBeInTheDocument();
    });
    expect(
      screen.queryByText("Viele Treffer – bitte den Namen weiter eingrenzen."),
    ).not.toBeInTheDocument();
  });
});

describe("GuardianPickerPanel select-and-confirm flow", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Search for one guardian, wait for it to render, and click it — leaving the
  // panel on the relationship step. Shared by the flow tests below.
  async function searchAndPick(guardian: Guardian) {
    mockSearchGuardians.mockResolvedValue([guardian]);
    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "schmidt" },
    });
    const fullName = `${guardian.firstName} ${guardian.lastName}`;
    await waitFor(() => {
      expect(screen.getByText(fullName)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(fullName));
  }

  it("confirms with the chosen guardian and the default relationship", async () => {
    const onSelect = vi.fn();
    const guardian = makeGuardian(7);
    render(<GuardianPickerPanel onSelect={onSelect} onCancel={vi.fn()} />);

    await searchAndPick(guardian);

    // The relationship step is now shown; confirm without changing anything.
    fireEvent.click(screen.getByText("Hinzufügen"));

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith(guardian, {
      relationshipType: "parent",
      guardianRole: "legal_guardian",
      isPrimary: false,
      isEmergencyContact: false,
      canPickup: false,
      emergencyPriority: 1,
    });
  });

  it("carries a toggled permission flag through to onSelect", async () => {
    const onSelect = vi.fn();
    const guardian = makeGuardian(8);
    render(<GuardianPickerPanel onSelect={onSelect} onCancel={vi.fn()} />);

    await searchAndPick(guardian);

    // Flip "Hauptansprechpartner" (isPrimary) before confirming.
    fireEvent.click(screen.getByLabelText("Hauptansprechpartner"));
    fireEvent.click(screen.getByText("Hinzufügen"));

    expect(onSelect).toHaveBeenCalledWith(
      guardian,
      expect.objectContaining({
        guardianRole: "legal_guardian",
        isPrimary: true,
        canPickup: false,
      }),
    );
  });

  it("applies pickup-only role defaults before confirming", async () => {
    const onSelect = vi.fn();
    const guardian = makeGuardian(10);
    render(<GuardianPickerPanel onSelect={onSelect} onCancel={vi.fn()} />);

    await searchAndPick(guardian);

    fireEvent.click(screen.getByLabelText("Portalrolle"));
    fireEvent.click(screen.getByRole("option", { name: "Nur Abholung" }));
    fireEvent.click(screen.getByText("Hinzufügen"));

    expect(onSelect).toHaveBeenCalledWith(
      guardian,
      expect.objectContaining({
        guardianRole: "pickup_only",
        isPrimary: false,
        isEmergencyContact: false,
        canPickup: true,
      }),
    );
  });

  it("resets portal role when relationship type changes to non-parent", async () => {
    const onSelect = vi.fn();
    const guardian = makeGuardian(11);
    render(<GuardianPickerPanel onSelect={onSelect} onCancel={vi.fn()} />);

    await searchAndPick(guardian);

    fireEvent.click(screen.getByLabelText("Beziehung zum Kind"));
    fireEvent.click(screen.getByRole("option", { name: "Verwandte/r" }));
    fireEvent.click(screen.getByText("Hinzufügen"));

    expect(onSelect).toHaveBeenCalledWith(
      guardian,
      expect.objectContaining({
        relationshipType: "relative",
        guardianRole: "custom",
      }),
    );
  });

  it("returns to the result list via 'Andere wählen' without confirming", async () => {
    const onSelect = vi.fn();
    const guardian = makeGuardian(9);
    render(<GuardianPickerPanel onSelect={onSelect} onCancel={vi.fn()} />);

    await searchAndPick(guardian);
    expect(screen.getByText("Hinzufügen")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Andere wählen"));

    // Back on the search step: the input reappears and nothing was selected.
    expect(
      screen.getByPlaceholderText("Name oder E-Mail suchen…"),
    ).toBeInTheDocument();
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("greys out and disables an already-added guardian", async () => {
    const guardian = makeGuardian(3);
    mockSearchGuardians.mockResolvedValue([guardian]);

    render(
      <GuardianPickerPanel
        onSelect={vi.fn()}
        onCancel={vi.fn()}
        excludeProfileIds={[guardian.id]}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "schmidt" },
    });

    await waitFor(() => {
      expect(screen.getByText("(bereits hinzugefügt)")).toBeInTheDocument();
    });
    // The result button for an excluded guardian must be non-interactive.
    const row = screen.getByText("First3 Last3").closest("button");
    expect(row).toBeDisabled();
  });

  it("shows the linked-children count for disambiguation", async () => {
    const guardian = { ...makeGuardian(4), linkedChildrenCount: 2 };
    mockSearchGuardians.mockResolvedValue([guardian]);

    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "schmidt" },
    });

    await waitFor(() => {
      expect(
        screen.getByText("Erziehungsberechtigte/r von 2 weiteren Kindern"),
      ).toBeInTheDocument();
    });
  });

  it("shows a German error message when the search fails", async () => {
    mockSearchGuardians.mockRejectedValue(new Error("backend exploded"));

    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "schmidt" },
    });

    await waitFor(() => {
      expect(
        screen.getByText("Suche fehlgeschlagen. Bitte erneut versuchen."),
      ).toBeInTheDocument();
    });
    // The raw backend message must never reach the user.
    expect(screen.queryByText("backend exploded")).not.toBeInTheDocument();
  });

  it("shows the minimum-length hint and does not search for short queries", async () => {
    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText("Name oder E-Mail suchen…"), {
      target: { value: "ab" },
    });

    expect(
      screen.getByText("Mindestens 3 Zeichen eingeben."),
    ).toBeInTheDocument();
    expect(mockSearchGuardians).not.toHaveBeenCalled();
  });

  it("calls onCancel when the close button is clicked", () => {
    const onCancel = vi.fn();
    render(<GuardianPickerPanel onSelect={vi.fn()} onCancel={onCancel} />);

    fireEvent.click(screen.getByLabelText("Suche schließen"));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
