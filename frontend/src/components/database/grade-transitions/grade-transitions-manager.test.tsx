// grade-transitions-manager.test.tsx
// Behavior tests for the Jahrgangsstufenwechsel admin flow (#405)

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { GradeTransitionsManager } from "./grade-transitions-manager";
import type {
  GradeTransition,
  SuggestedMapping,
  TransitionPreview,
  TransitionResult,
} from "~/lib/grade-transition-api";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

const api = {
  listGradeTransitions: vi.fn<() => Promise<GradeTransition[]>>(),
  fetchSuggestedMappings: vi.fn<() => Promise<SuggestedMapping[]>>(),
  fetchTransitionClasses: vi.fn<() => Promise<string[]>>(),
  createGradeTransition: vi.fn(),
  updateGradeTransition: vi.fn(),
  deleteGradeTransition: vi.fn(),
  previewGradeTransition: vi.fn<(id: string) => Promise<TransitionPreview>>(),
  applyGradeTransition: vi.fn<(id: string) => Promise<TransitionResult>>(),
  revertGradeTransition: vi.fn<(id: string) => Promise<TransitionResult>>(),
  fetchTransitionHistory: vi.fn(),
};

vi.mock("~/lib/grade-transition-api", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/grade-transition-api")>();
  return {
    ...original,
    listGradeTransitions: (...args: unknown[]) =>
      api.listGradeTransitions(...(args as [])),
    fetchSuggestedMappings: (...args: unknown[]) =>
      api.fetchSuggestedMappings(...(args as [])),
    fetchTransitionClasses: (...args: unknown[]) =>
      api.fetchTransitionClasses(...(args as [])),
    createGradeTransition: (...args: unknown[]) =>
      api.createGradeTransition(...(args as [unknown])),
    updateGradeTransition: (...args: unknown[]) =>
      api.updateGradeTransition(...(args as [unknown, unknown])),
    deleteGradeTransition: (...args: unknown[]) =>
      api.deleteGradeTransition(...(args as [string])),
    previewGradeTransition: (...args: unknown[]) =>
      api.previewGradeTransition(...(args as [string])),
    applyGradeTransition: (...args: unknown[]) =>
      api.applyGradeTransition(...(args as [string])),
    revertGradeTransition: (...args: unknown[]) =>
      api.revertGradeTransition(...(args as [string])),
    fetchTransitionHistory: (...args: unknown[]) =>
      api.fetchTransitionHistory(...(args as [string])),
  };
});

const draftTransition: GradeTransition = {
  id: "7",
  academicYear: "2026-2027",
  status: "draft",
  appliedAt: null,
  revertedAt: null,
  createdAt: "2026-07-24T09:00:00Z",
  notes: null,
  mappings: [
    { id: "1", fromClass: "1a", toClass: "2a", action: "promote" },
    { id: "2", fromClass: "4a", toClass: null, action: "graduate" },
  ],
  canModify: true,
  canApply: true,
  canRevert: false,
};

const appliedTransition: GradeTransition = {
  ...draftTransition,
  id: "8",
  status: "applied",
  appliedAt: "2026-08-01T06:00:00Z",
  canModify: false,
  canApply: false,
  canRevert: true,
};

const suggestions: SuggestedMapping[] = [
  { fromClass: "1a", toClass: "2a", studentCount: 20, isGraduating: false },
  { fromClass: "4a", toClass: null, studentCount: 10, isGraduating: true },
];

const preview: TransitionPreview = {
  transitionId: "7",
  academicYear: "2026-2027",
  totalStudents: 30,
  toPromote: 20,
  toGraduate: 10,
  byMapping: [
    { fromClass: "1a", toClass: "2a", studentCount: 20, action: "promote" },
    { fromClass: "4a", toClass: null, studentCount: 10, action: "graduate" },
  ],
  unmappedClasses: [{ className: "3c", studentCount: 15 }],
  warnings: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.listGradeTransitions.mockResolvedValue([]);
  api.fetchSuggestedMappings.mockResolvedValue(suggestions);
  api.fetchTransitionClasses.mockResolvedValue(["1a", "2a", "3c", "4a"]);
  api.previewGradeTransition.mockResolvedValue(preview);
});

describe("GradeTransitionsManager", () => {
  it("shows an empty state and the new-transition action", async () => {
    render(<GradeTransitionsManager />);
    expect(
      await screen.findByRole("button", { name: /Neuer Jahrgangswechsel/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Noch kein Jahrgangswechsel/i)).toBeInTheDocument();
  });

  it("prefills the editor from suggestions", async () => {
    render(<GradeTransitionsManager />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Neuer Jahrgangswechsel/i }),
    );

    // Both suggested rows appear with counts
    expect(await screen.findByText("1a")).toBeInTheDocument();
    expect(screen.getByText("4a")).toBeInTheDocument();
    expect(screen.getByDisplayValue("2a")).toBeInTheDocument();
    // The graduating row shows the Abgang selection
    expect(screen.getAllByText(/Abgang/i).length).toBeGreaterThan(0);
  });

  it("creates a draft with null to_class for Abgang rows and shows the preview", async () => {
    api.createGradeTransition.mockResolvedValue(draftTransition);
    render(<GradeTransitionsManager />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Neuer Jahrgangswechsel/i }),
    );
    await screen.findByText("1a");

    fireEvent.click(
      screen.getByRole("button", { name: /Weiter zur Vorschau/i }),
    );

    await waitFor(() => {
      expect(api.createGradeTransition).toHaveBeenCalledTimes(1);
    });
    const input = api.createGradeTransition.mock.calls[0]?.[0] as {
      academicYear: string;
      mappings: { fromClass: string; toClass: string | null }[];
    };
    expect(input.mappings).toEqual([
      { fromClass: "1a", toClass: "2a" },
      { fromClass: "4a", toClass: null },
    ]);

    // Preview modal shows counts and the red graduation warning
    expect(await screen.findByText(/20/)).toBeInTheDocument();
    expect(
      screen.getByText(/10 Kinder verlassen die OGS/i),
    ).toBeInTheDocument();
    // Unmapped class warning
    expect(screen.getByText(/3c/)).toBeInTheDocument();
  });

  it("applies the transition after explicit confirmation", async () => {
    api.createGradeTransition.mockResolvedValue(draftTransition);
    api.applyGradeTransition.mockResolvedValue({
      transitionId: "7",
      status: "applied",
      studentsPromoted: 20,
      studentsGraduated: 10,
      canRevert: true,
      warnings: [],
    });
    render(<GradeTransitionsManager />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Neuer Jahrgangswechsel/i }),
    );
    await screen.findByText("1a");
    fireEvent.click(
      screen.getByRole("button", { name: /Weiter zur Vorschau/i }),
    );
    await screen.findByText(/10 Kinder verlassen die OGS/i);

    fireEvent.click(
      screen.getByRole("button", { name: /Jahrgangswechsel anwenden/i }),
    );
    // Second, explicit confirmation
    const dialog = await screen.findByText(/Wirklich anwenden/i);
    expect(dialog).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Ja, anwenden/i }));

    await waitFor(() => {
      expect(api.applyGradeTransition).toHaveBeenCalledWith("7");
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("reverts an applied transition after confirmation", async () => {
    api.listGradeTransitions.mockResolvedValue([appliedTransition]);
    api.revertGradeTransition.mockResolvedValue({
      transitionId: "8",
      status: "reverted",
      studentsPromoted: 20,
      studentsGraduated: 10,
      canRevert: false,
      warnings: [],
    });
    render(<GradeTransitionsManager />);

    const row = await screen.findByText("2026-2027");
    expect(row).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Zurücksetzen/i }));
    await screen.findByText(/wirklich zurücksetzen/i);
    fireEvent.click(screen.getByRole("button", { name: /Ja, zurücksetzen/i }));

    await waitFor(() => {
      expect(api.revertGradeTransition).toHaveBeenCalledWith("8");
    });
    expect(toastSuccess).toHaveBeenCalled();
  });

  it("deletes a draft after confirmation", async () => {
    api.listGradeTransitions.mockResolvedValue([draftTransition]);
    api.deleteGradeTransition.mockResolvedValue(undefined);
    render(<GradeTransitionsManager />);

    await screen.findByText("2026-2027");
    fireEvent.click(screen.getByRole("button", { name: /Löschen/i }));
    await screen.findByText(/wirklich löschen/i);
    fireEvent.click(screen.getByRole("button", { name: /Ja, löschen/i }));

    await waitFor(() => {
      expect(api.deleteGradeTransition).toHaveBeenCalledWith("7");
    });
  });
});

describe("status labels", () => {
  it("renders German status badges", async () => {
    api.listGradeTransitions.mockResolvedValue([
      draftTransition,
      appliedTransition,
    ]);
    render(<GradeTransitionsManager />);
    expect(await screen.findByText("Entwurf")).toBeInTheDocument();
    expect(screen.getByText("Angewendet")).toBeInTheDocument();
  });
});
