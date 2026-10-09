import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SpontaneousActivityStart as ProductionSpontaneousActivityStart } from "./spontaneous-activity-start";

const mocks = vi.hoisted(() => ({
  getActivities: vi.fn(),
  getAllStaff: vi.fn(),
  createExternal: vi.fn(),
}));

vi.mock("~/lib/activity-service", () => ({
  activityService: {
    getActivities: mocks.getActivities,
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: mocks.getAllStaff,
    createExternal: mocks.createExternal,
  },
}));

const SpontaneousActivityStart = ProductionSpontaneousActivityStart;

// The modal renders to a portal with animations in the real kit component;
// stub it to a plain container so tests stay synchronous. The footer is a
// separate prop from the body, so render both (the submit/cancel buttons live
// in the footer). The stub keeps the real kit's document-level Escape handler
// (form-modal.tsx) so the autocomplete-vs-modal Escape regression is covered.
vi.mock("~/components/ui/form-modal", async () => {
  const { useEffect } = await import("react");
  return {
    FormModal: ({
      isOpen,
      onClose,
      children,
      footer,
      title,
    }: {
      isOpen: boolean;
      onClose: () => void;
      children: React.ReactNode;
      footer?: React.ReactNode;
      title: string;
    }) => {
      useEffect(() => {
        if (!isOpen) return;
        const handleEscKey = (event: KeyboardEvent) => {
          if (event.key === "Escape") onClose();
        };
        document.addEventListener("keydown", handleEscKey);
        return () => document.removeEventListener("keydown", handleEscKey);
      }, [isOpen, onClose]);
      return isOpen ? (
        <div data-testid="modal" data-title={title}>
          {children}
          {footer}
        </div>
      ) : null;
    },
  };
});

// The kit room picker (CustomSelect) is a listbox button, not a native
// <select>: options only render once the trigger is opened, and the submit
// button lives outside the <form> (wired via the form="" attribute), so we
// submit the form element directly.
function getRoomCombobox(): HTMLElement {
  return screen.getByRole("combobox", { name: "Raum" });
}

async function openModalAndWaitForRefs(): Promise<void> {
  fireEvent.click(
    screen.getByRole("button", { name: /Spontane Aktivität starten/ }),
  );
  await screen.findByTestId("modal");
  // References load asynchronously; the room picker is disabled until then
  // (and the default room is auto-selected at the same moment).
  await waitFor(() => expect(getRoomCombobox()).toBeEnabled());
}

function submitForm(): void {
  const form = screen
    .getByPlaceholderText("Aktivität suchen oder neu eingeben")
    .closest("form");
  if (!form) throw new Error("spontaneous activity form not found");
  fireEvent.submit(form);
}

// #3823: externe Betreuungskräfte ohne Konto beim Start eintragen.
describe("SpontaneousActivityStart external caregivers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getActivities.mockResolvedValue([{ id: "7", name: "Freispiel" }]);
    mocks.getAllStaff.mockResolvedValue([
      { id: "12", name: "Ben Staff", firstName: "Ben", lastName: "Staff" },
      {
        id: "74",
        name: "Jonas Becker",
        firstName: "Jonas",
        lastName: "Becker",
        isExternal: true,
        externalOrganization: "Musikschule",
      },
    ]);
    mocks.createExternal.mockResolvedValue({
      id: "88",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      isExternal: true,
    });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        json: async () => ({ data: [{ id: 3, name: "Mensa" }] }),
      }),
    );
  });

  it("marks external people in the list", async () => {
    render(
      <SpontaneousActivityStart
        canCreateExternalCaregiver
        currentStaffId="11"
        onStart={vi.fn()}
      />,
    );
    await openModalAndWaitForRefs();

    expect(screen.getByText("Extern · Musikschule")).toBeInTheDocument();
  });

  it("records a new external person, ticks it and starts with it", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        canCreateExternalCaregiver
        currentStaffId="11"
        onStart={onStart}
      />,
    );
    await openModalAndWaitForRefs();

    fireEvent.click(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    );
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Lea" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Gast" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(await screen.findByLabelText(/Lea Gast/)).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Freispiel" }));
    submitForm();

    await waitFor(() =>
      expect(onStart).toHaveBeenCalledWith({
        title: "Freispiel",
        roomId: "3",
        activityGroupId: "7",
        additionalStaffIds: ["88"],
      }),
    );
  });

  it("does not offer the entry without permission to create users", async () => {
    render(<SpontaneousActivityStart currentStaffId="11" onStart={vi.fn()} />);
    await openModalAndWaitForRefs();

    expect(
      screen.queryByRole("button", { name: "Externe Person eintragen" }),
    ).not.toBeInTheDocument();
  });
});
