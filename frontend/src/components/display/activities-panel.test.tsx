import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ActivitiesPanel } from "./activities-panel";

describe("ActivitiesPanel", () => {
  it("keeps same-name, same-time activities in different rooms distinct", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    render(
      <ActivitiesPanel
        running={[]}
        upcoming={[
          {
            name: "Schach",
            category: "AG",
            start_time: "14:00",
            room_name: "Raum 1",
          },
          {
            name: "Schach",
            category: "AG",
            start_time: "14:00",
            room_name: "Raum 2",
          },
        ]}
      />,
    );

    expect(screen.getAllByText("Schach")).toHaveLength(2);
    expect(
      consoleError.mock.calls.some((call) =>
        String(call[0]).includes("same key"),
      ),
    ).toBe(false);

    consoleError.mockRestore();
  });
});
