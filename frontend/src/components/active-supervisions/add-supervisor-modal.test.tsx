import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { fetchRunningSupervision, addSupervisor, showSuccess } = vi.hoisted(
  () => ({
    fetchRunningSupervision: vi.fn(),
    addSupervisor: vi.fn(),
    showSuccess: vi.fn(),
  }),
);

vi.mock("~/lib/substitution-api", () => ({
  substitutionService: { fetchRunningSupervision, addSupervisor },
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: showSuccess }),
}));
vi.mock("~/components/ui/modal", () => ({
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
  name: "Freispiel",
  roomName: "Atelier",
  supervisors: [{ id: "11", fullName: "Alex Alt" }],
  availableTargets: [{ id: "73", fullName: "Toni Test" }],
  isCurrentUserSupervising: true,
  canAssign: true,
};

describe("AddSupervisorModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    fetchRunningSupervision.mockResolvedValue(overview);
    addSupervisor.mockResolvedValue({ id: "91", targetName: "Toni Test" });
  });

  it("shows existing supervisors and assigns only an available person", async () => {
    const onAdded = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(
      <AddSupervisorModal
        activeGroupId="41"
        isOpen
        onAdded={onAdded}
        onClose={onClose}
      />,
    );

    expect(await screen.findByText(/Alex Alt/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Betreuer auswählen"), {
      target: { value: "73" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => expect(addSupervisor).toHaveBeenCalledWith("41", "73"));
    expect(onAdded).toHaveBeenCalledOnce();
    expect(showSuccess).toHaveBeenCalledWith(
      "Toni Test ist jetzt als Betreuer eingetragen.",
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("explains when nobody else can be added", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      availableTargets: [],
    });

    render(
      <AddSupervisorModal
        activeGroupId="41"
        isOpen
        onAdded={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      await screen.findByText(
        "Alle verfügbaren Betreuungskräfte sind schon eingetragen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();
  });

  it("blocks a stale action when the user no longer supervises the session", async () => {
    fetchRunningSupervision.mockResolvedValue({
      ...overview,
      isCurrentUserSupervising: false,
      canAssign: false,
    });

    render(
      <AddSupervisorModal
        activeGroupId="41"
        isOpen
        onAdded={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      await screen.findByText(
        "Sie beaufsichtigen diese Betreuung nicht mehr. Deshalb können Sie niemanden hinzufügen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeDisabled();
    expect(
      screen.queryByLabelText("Betreuer auswählen"),
    ).not.toBeInTheDocument();
  });

  it("keeps the modal open and resets saving after the assignment fails", async () => {
    addSupervisor.mockRejectedValue(new Error("Diese Betreuung ist beendet."));
    const onAdded = vi.fn();
    const onClose = vi.fn();
    render(
      <AddSupervisorModal
        activeGroupId="41"
        isOpen
        onAdded={onAdded}
        onClose={onClose}
      />,
    );

    await screen.findByText(/Alex Alt/);
    fireEvent.change(screen.getByLabelText("Betreuer auswählen"), {
      target: { value: "73" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    expect(
      await screen.findByText("Diese Betreuung ist beendet."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hinzufügen" })).toBeEnabled();
    expect(onAdded).not.toHaveBeenCalled();
    expect(showSuccess).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
