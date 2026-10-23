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
  supervisors: [
    { id: "11", fullName: "Alex Alt" },
    { id: "75", fullName: "Elli Extern", isExternal: true as const },
  ],
  availableTargets: [
    { id: "73", fullName: "Toni Test" },
    { id: "74", fullName: "Jonas Becker", isExternal: true as const },
    {
      id: "76",
      fullName: "Mira Neu",
      isExternal: true as const,
      externalOrganization: "Musikschule",
      externalAccountStatus: "invitation_pending" as const,
    },
  ],
  isCurrentUserSupervising: true,
  canAssign: true,
};

function renderModal(canCreateExternalCaregiver = true) {
  render(
    <AddSupervisorModal
      activeGroupId="41"
      canCreateExternalCaregiver={canCreateExternalCaregiver}
      isOpen
      onAdded={vi.fn().mockResolvedValue(undefined)}
      onClose={vi.fn()}
    />,
  );
}

// #3823/#4573: externe Betreuungskräfte in der laufenden Aufsicht.
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
      externalAccountStatus: "invitation_pending",
    });
  });

  it("labels external people in the picker", async () => {
    renderModal();

    expect(
      await screen.findByRole("option", { name: "Jonas Becker (Extern)" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Toni Test" }),
    ).toBeInTheDocument();
  });

  // #4573: Organisation und offene Einladung stehen am Namen, wie in den
  // anderen Auswahllisten („Extern · Musikschule, Einladung offen“).
  it("labels an external person with organization and open invitation", async () => {
    renderModal();

    expect(
      await screen.findByRole("option", {
        name: "Mira Neu (Extern · Musikschule, Einladung offen)",
      }),
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
    fireEvent.click(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "lea@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(screen.getByLabelText("Betreuer auswählen")).toHaveValue("88"),
    );
    expect(
      screen.getByRole("option", {
        name: "Lea Gast (Extern, Einladung offen)",
      }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Hinzufügen" }));

    await waitFor(() => expect(addSupervisor).toHaveBeenCalledWith("41", "88"));
  });

  // #4573 decision: the quick entry always records a new person. A name
  // match no longer picks or blocks anyone; the address decides, and the
  // backend refuses one that already signs in at the school.
  it("records a namesake of an assigned external as a new person", async () => {
    createExternal.mockResolvedValue({
      id: "89",
      name: "Elli Extern",
      firstName: "Elli",
      lastName: "Extern",
      isExternal: true,
      externalAccountStatus: "invitation_pending",
    });
    renderModal();

    fireEvent.click(
      await screen.findByRole("button", { name: "Externe Person eintragen" }),
    );
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Elli" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Extern" },
    });
    fireEvent.click(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "elli@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(screen.getByLabelText("Betreuer auswählen")).toHaveValue("89"),
    );
    expect(createExternal).toHaveBeenCalledWith({
      firstName: "Elli",
      lastName: "Extern",
      email: "elli@example.test",
      organization: "",
      access: "supervise",
    });
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

  it("does not offer the entry without permission to create users", async () => {
    renderModal(false);

    expect(
      await screen.findByRole("option", { name: "Jonas Becker (Extern)" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Externe Person eintragen" }),
    ).not.toBeInTheDocument();
  });
});
