import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { fetchRunningSupervision, addSupervisor, showSuccess, createExternal } =
  vi.hoisted(() => ({
    fetchRunningSupervision: vi.fn(),
    addSupervisor: vi.fn(),
    showSuccess: vi.fn(),
    createExternal: vi.fn(),
  }));

vi.mock("~/lib/staff-api", () => ({
  staffService: { createExternal },
}));

vi.mock("~/lib/substitution-api", () => ({
  substitutionService: { fetchRunningSupervision, addSupervisor },
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: showSuccess }),
}));
vi.mock("~/components/ui/modal", () => ({
  dialogAriaProps: { role: "dialog" as const, "aria-modal": true },
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer: React.ReactNode;
  }) =>
    isOpen ? (
      <section aria-label={title}>
        {children}
        {footer}
      </section>
    ) : null,
}));
vi.mock("~/components/ui/custom-select", () => ({
  CustomSelect: ({
    value,
    options,
    onChange,
    ariaLabelledBy,
  }: {
    value: string;
    options: Array<{ value: string; label: string }>;
    onChange: (value: string) => void;
    ariaLabelledBy: string;
  }) => (
    <select
      aria-labelledby={ariaLabelledBy}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
}));

import { AddSupervisorModal } from "./add-supervisor-modal";

const overview = {
  id: "41",
  name: "Trommel-AG",
  roomName: "Musikraum",
  supervisors: [{ id: "11", fullName: "Alex Alt" }],
  availableTargets: [
    { id: "73", fullName: "Toni Test" },
    { id: "74", fullName: "Jonas Becker", isExternal: true as const },
  ],
  isCurrentUserSupervising: true,
  canAssign: true,
};

function renderModal() {
  render(
    <AddSupervisorModal
      activeGroupId="41"
      isOpen
      onAdded={vi.fn().mockResolvedValue(undefined)}
      onClose={vi.fn()}
    />,
  );
}

// #3823: externe Betreuungskräfte ohne Konto in der laufenden Aufsicht.
describe("AddSupervisorModal external caregivers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchRunningSupervision.mockResolvedValue(overview);
    addSupervisor.mockResolvedValue({ id: "91", targetName: "Lea Gast" });
    createExternal.mockResolvedValue({
      id: "88",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
    });
  });

  it("labels external people in the picker", async () => {
    renderModal();

    expect(
      await screen.findByRole("option", { name: "Jonas Becker (extern)" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Toni Test" }),
    ).toBeInTheDocument();
  });

  it("records a new external person, selects it and adds it", async () => {
    renderModal();

    fireEvent.click(
      await screen.findByRole("button", { name: "Externe Person eintragen" }),
    );
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Lea" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Gast" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(screen.getByLabelText("Betreuer auswählen")).toHaveValue("88"),
    );
    expect(
      screen.getByRole("option", { name: "Lea Gast (extern)" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => expect(addSupervisor).toHaveBeenCalledWith("41", "88"));
  });

  it("still offers a new external person when every caregiver is assigned", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      availableTargets: [],
    });
    renderModal();

    expect(
      await screen.findByText(
        "Alle verfügbaren Betreuungskräfte sind schon eingetragen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    ).toBeInTheDocument();
  });

  it("offers no entry when the user may not assign anyone", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      canAssign: false,
    });
    renderModal();

    expect(
      await screen.findByText(/Deshalb können Sie niemanden hinzufügen/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Externe Person eintragen" }),
    ).not.toBeInTheDocument();
  });
});
