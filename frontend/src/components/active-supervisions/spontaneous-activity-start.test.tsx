import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SpontaneousActivityStart as ProductionSpontaneousActivityStart } from "./spontaneous-activity-start";

const mocks = vi.hoisted(() => ({
  getActivities: vi.fn(),
  getAllStaff: vi.fn(),
}));

vi.mock("~/lib/activity-service", () => ({
  activityService: {
    getActivities: mocks.getActivities,
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: mocks.getAllStaff,
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

function selectRoom(optionName: string): void {
  fireEvent.click(getRoomCombobox());
  fireEvent.click(screen.getByRole("option", { name: optionName }));
}

function submitForm(): void {
  const form = screen
    .getByPlaceholderText("Aktivität suchen oder neu eingeben")
    .closest("form");
  if (!form) throw new Error("spontaneous activity form not found");
  fireEvent.submit(form);
}

describe("SpontaneousActivityStart", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getActivities.mockResolvedValue([
      { id: "7", name: "Freispiel" },
      { id: "9", name: "Basteln" },
    ]);
    mocks.getAllStaff.mockResolvedValue([
      { id: "11", name: "Ada Staff", firstName: "Ada", lastName: "Staff" },
      { id: "12", name: "Ben Staff", firstName: "Ben", lastName: "Staff" },
    ]);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        json: async () => ({
          data: [
            { id: 3, name: "Mensa" },
            { id: 4, name: "Atelier", building: "Haus A" },
          ],
        }),
      }),
    );
  });

  it("starts an existing activity in the selected room with optional staff", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="4"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: "Basteln" },
      },
    );
    selectRoom("Mensa");
    fireEvent.click(screen.getByLabelText("Ben Staff"));
    submitForm();

    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Basteln",
        roomId: "3",
        activityGroupId: "9",
        additionalStaffIds: ["12"],
      });
    });
  });

  it("accepts the raw /api/rooms array response used by the route wrapper", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        json: async () => [
          { id: 3, name: "Mensa" },
          { id: 4, name: "Atelier", building: "Haus A" },
        ],
      }),
    );
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="4"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.click(getRoomCombobox());
    expect(screen.getByRole("option", { name: "Mensa" })).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Haus A - Atelier" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("option", { name: "Mensa" }));

    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: "Tennis" },
      },
    );
    submitForm();

    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Tennis",
        roomId: "3",
        activityGroupId: undefined,
        additionalStaffIds: [],
      });
    });
  });

  it("treats Schulhof as a normal startable room (#2161)", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        json: async () => ({
          data: [
            { id: 3, name: "Mensa" },
            { id: 5, name: "Schulhof" },
          ],
        }),
      }),
    );
    const onStart = vi.fn();
    render(<SpontaneousActivityStart currentStaffId="11" onStart={onStart} />);

    await openModalAndWaitForRefs();
    fireEvent.click(getRoomCombobox());
    const schulhofOption = screen.getByRole("option", { name: "Schulhof" });
    expect(schulhofOption).toBeEnabled();
    fireEvent.click(schulhofOption);

    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      { target: { value: "Hofpause" } },
    );
    expect(
      screen.getByRole("button", { name: "Aktivität starten" }),
    ).toBeInTheDocument();
    submitForm();

    expect(onStart).toHaveBeenCalledWith({
      title: "Hofpause",
      roomId: "5",
      activityGroupId: undefined,
      additionalStaffIds: [],
    });
  });

  it("allows a custom spontaneous activity title without template binding", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: " spontane Lego-Werkstatt " },
      },
    );
    submitForm();

    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "spontane Lego-Werkstatt",
        roomId: "3",
        activityGroupId: undefined,
        additionalStaffIds: [],
      });
    });
  });

  it("does not submit when every room is occupied", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        occupiedRoomIds={["3", "4"]}
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.click(getRoomCombobox());
    expect(
      screen.getByRole("option", { name: "Mensa (belegt)" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("option", { name: "Haus A - Atelier (belegt)" }),
    ).toBeDisabled();
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: "Tennis" },
      },
    );

    expect(
      screen.getByRole("button", { name: "Aktivität starten" }),
    ).toBeDisabled();
    expect(onStart).not.toHaveBeenCalled();
  });

  it("uses suggested activities and removes staff when toggled twice", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.click(screen.getByRole("button", { name: "Freispiel" }));
    fireEvent.click(screen.getByLabelText("Ben Staff"));
    fireEvent.click(screen.getByLabelText("Ben Staff"));
    submitForm();

    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Freispiel",
        roomId: "3",
        activityGroupId: "7",
        additionalStaffIds: [],
      });
    });
  });

  it("falls back to available labels when reference fetches fail", async () => {
    mocks.getActivities.mockRejectedValueOnce(new Error("activities down"));
    mocks.getAllStaff.mockRejectedValueOnce(new Error("staff down"));
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValueOnce(new Error("rooms down")),
    );

    render(<SpontaneousActivityStart onStart={vi.fn()} />);

    await openModalAndWaitForRefs();
    // No rooms loaded -> the picker shows its placeholder and has no options.
    expect(screen.getByText("Raum auswählen")).toBeInTheDocument();
    expect(screen.queryByText("Weitere Betreuer")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Aktivität starten" }),
    ).toBeDisabled();
  });

  it("disables an occupied room so it cannot be selected for a new activity", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        occupiedRoomIds={["3"]}
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.click(getRoomCombobox());
    // The occupied room is rendered but not selectable.
    expect(
      screen.getByRole("option", { name: "Mensa (belegt)" }),
    ).toBeDisabled();
    // The remaining free room stays usable.
    expect(
      screen.getByRole("option", { name: "Haus A - Atelier" }),
    ).toBeEnabled();
    expect(onStart).not.toHaveBeenCalled();
  });

  it("resets draft state when the modal is cancelled", async () => {
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={vi.fn()}
      />,
    );

    await openModalAndWaitForRefs();
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: "Tennis" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    await openModalAndWaitForRefs();
    expect(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
    ).toHaveValue("");
  });

  it("dismisses the activity suggestions on Escape without closing the modal", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    const activityInput = screen.getByPlaceholderText(
      "Aktivität suchen oder neu eingeben",
    );
    fireEvent.change(activityInput, { target: { value: "Bast" } });
    // The suggestion list is open (the listbox option, not the chip button).
    expect(screen.getByRole("option", { name: "Basteln" })).toBeInTheDocument();

    fireEvent.keyDown(activityInput, { key: "Escape" });

    // Escape only closes the suggestion list...
    expect(
      screen.queryByRole("option", { name: "Basteln" }),
    ).not.toBeInTheDocument();
    // ...the modal stays open and the draft survives (no bubble to the
    // FormModal document-level Escape handler).
    expect(screen.getByTestId("modal")).toBeInTheDocument();
    expect(activityInput).toHaveValue("Bast");
  });

  it("lets Escape close the modal when no activity suggestions are visible", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    const activityInput = screen.getByPlaceholderText(
      "Aktivität suchen oder neu eingeben",
    );
    fireEvent.change(activityInput, { target: { value: "ZZZ" } });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    fireEvent.keyDown(activityInput, { key: "Escape" });

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("closes activity suggestions when keyboard focus leaves the field", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    const activityInput = screen.getByPlaceholderText(
      "Aktivität suchen oder neu eingeben",
    );
    fireEvent.change(activityInput, { target: { value: "Bast" } });
    expect(screen.getByRole("option", { name: "Basteln" })).toBeInTheDocument();

    fireEvent.blur(activityInput, { relatedTarget: getRoomCombobox() });

    expect(
      screen.queryByRole("option", { name: "Basteln" }),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("selects the active activity suggestion on Enter before submitting", async () => {
    mocks.getActivities.mockResolvedValueOnce([
      { id: "5", name: "Ballspiele" },
      { id: "9", name: "Basteln" },
      { id: "7", name: "Freispiel" },
    ]);
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    const activityInput = screen.getByPlaceholderText(
      "Aktivität suchen oder neu eingeben",
    );
    fireEvent.change(activityInput, { target: { value: "Ba" } });
    expect(screen.getByRole("option", { name: "Ballspiele" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    fireEvent.keyDown(activityInput, { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Basteln" })).toHaveAttribute(
      "aria-selected",
      "true",
    );

    fireEvent.keyDown(activityInput, { key: "Enter" });
    expect(activityInput).toHaveValue("Basteln");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    submitForm();
    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Basteln",
        roomId: "3",
        activityGroupId: "9",
        additionalStaffIds: [],
      });
    });
  });

  it("selects an activity suggestion on pointer-down even when the input blurs with no relatedTarget (Safari/iOS)", async () => {
    const onStart = vi.fn();
    render(
      <SpontaneousActivityStart
        currentStaffId="11"
        defaultRoomId="3"
        onStart={onStart}
      />,
    );

    await openModalAndWaitForRefs();
    const activityInput = screen.getByPlaceholderText(
      "Aktivität suchen oder neu eingeben",
    );
    fireEvent.change(activityInput, { target: { value: "Bast" } });
    const option = screen.getByRole("option", { name: "Basteln" });

    // Safari/iOS does not focus a clicked button, so the input blurs with a
    // null relatedTarget. Selection must happen on pointer-down (which the
    // component preventDefaults to keep focus on the input) BEFORE that blur
    // can close the menu and unmount the option.
    fireEvent.pointerDown(option, { button: 0, pointerType: "mouse" });
    fireEvent.blur(activityInput, { relatedTarget: null });

    expect(activityInput).toHaveValue("Basteln");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    submitForm();
    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Basteln",
        roomId: "3",
        activityGroupId: "9",
        additionalStaffIds: [],
      });
    });
  });
});
