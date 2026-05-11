import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SpontaneousActivityStart } from "./spontaneous-activity-start";

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

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {children}
      </div>
    ) : null,
}));

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

    fireEvent.click(
      screen.getByRole("button", { name: /Spontane Aktivität starten/ }),
    );

    await screen.findByTestId("modal");
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: "Basteln" },
      },
    );
    fireEvent.change(screen.getByLabelText("Raum"), {
      target: { value: "3" },
    });
    fireEvent.click(screen.getByLabelText("Ben Staff"));
    fireEvent.click(screen.getByRole("button", { name: "Aktivität starten" }));

    await waitFor(() => {
      expect(onStart).toHaveBeenCalledWith({
        title: "Basteln",
        roomId: "3",
        activityGroupId: "9",
        additionalStaffIds: ["12"],
      });
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

    fireEvent.click(
      screen.getByRole("button", { name: /Spontane Aktivität starten/ }),
    );

    await screen.findByTestId("modal");
    fireEvent.change(
      screen.getByPlaceholderText("Aktivität suchen oder neu eingeben"),
      {
        target: { value: " spontane Lego-Werkstatt " },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Aktivität starten" }));

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

    fireEvent.click(
      screen.getByRole("button", { name: /Spontane Aktivität starten/ }),
    );

    await screen.findByTestId("modal");
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
});
