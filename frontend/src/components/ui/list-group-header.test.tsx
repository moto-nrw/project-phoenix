import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ListGroupHeader } from "./list-group-header";

describe("ListGroupHeader", () => {
  it("shows the group name as a heading with its figure beside it", () => {
    render(<ListGroupHeader as="h2" title="Mo 09.11." meta="3 Einträge" />);
    expect(
      screen.getByRole("heading", { level: 2, name: "Mo 09.11." }),
    ).toBeInTheDocument();
    expect(screen.getByText("3 Einträge")).toBeInTheDocument();
  });

  it("keeps an addon outside the heading's name", () => {
    render(
      <ListGroupHeader title="Di 10.11." addon={<span>Schließtag</span>} />,
    );
    expect(
      screen.getByRole("heading", { level: 3, name: "Di 10.11." }),
    ).toBeInTheDocument();
    expect(screen.getByText("Schließtag")).toBeInTheDocument();
  });

  it("toggles a collapsible group from its name", () => {
    const onToggle = vi.fn();
    const { rerender } = render(
      <ListGroupHeader
        title="Aktivitäten"
        collapsible={{ expanded: true, onToggle }}
      />,
    );
    const toggle = screen.getByRole("button", { name: "Aktivitäten" });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("heading", { name: "Aktivitäten" }),
    ).toContainElement(toggle);

    fireEvent.click(toggle);
    expect(onToggle).toHaveBeenCalledTimes(1);

    rerender(
      <ListGroupHeader
        title="Aktivitäten"
        collapsible={{ expanded: false, onToggle }}
      />,
    );
    expect(screen.getByRole("button", { name: "Aktivitäten" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
  });

  it("describes a collapsible group with its addon", () => {
    render(
      <ListGroupHeader
        title="Aktivitäten"
        addon="2/5"
        collapsible={{ expanded: true, onToggle: vi.fn() }}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Aktivitäten" }),
    ).toHaveAccessibleDescription("2/5");
  });

  it("renders no toggle without collapsible", () => {
    render(<ListGroupHeader title="Mi 11.11." />);
    expect(screen.getByRole("heading", { name: "Mi 11.11." })).toBeVisible();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
