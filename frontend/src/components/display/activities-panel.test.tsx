import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ActivitiesPanel } from "./activities-panel";

describe("ActivitiesPanel", () => {
  it("keeps visually identical activity instances distinct by ID", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    render(
      <ActivitiesPanel
        running={[
          {
            id: "running-1",
            name: "Fußball",
            category: "AG",
            room_name: "Turnhalle",
            participants: 8,
          },
          {
            id: "running-2",
            name: "Fußball",
            category: "AG",
            room_name: "Turnhalle",
            participants: 10,
          },
        ]}
        upcoming={[
          {
            id: "upcoming-1",
            name: "Schach",
            category: "AG",
            start_time: "14:00",
            room_name: "Raum 1",
          },
          {
            id: "upcoming-2",
            name: "Schach",
            category: "AG",
            start_time: "14:00",
            room_name: "Raum 1",
          },
        ]}
      />,
    );

    expect(screen.getAllByText("Schach")).toHaveLength(2);
    expect(screen.getAllByText("Fußball")).toHaveLength(2);
    expect(
      consoleError.mock.calls.some((call) =>
        String(call[0]).includes("same key"),
      ),
    ).toBe(false);

    consoleError.mockRestore();
  });
});
