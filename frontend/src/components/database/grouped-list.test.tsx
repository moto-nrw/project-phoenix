import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { GroupedList, type GroupDefinition } from "./grouped-list";

interface Item {
  id: string;
  name: string;
}

const items: Item[] = [
  { id: "a", name: "Alpha" },
  { id: "b", name: "Beta" },
];

function groups(): GroupDefinition<Item>[] {
  return [
    { id: "g1", title: "Group 1", items },
    { id: "g2", title: "Group 2", items: [{ id: "c", name: "Gamma" }] },
  ];
}

describe("GroupedList", () => {
  it("renders all groups and their items when open by default", () => {
    render(
      <GroupedList
        groups={groups()}
        renderItem={(item) => <span>{item.name}</span>}
        keyFor={(item) => item.id}
      />,
    );

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
    expect(screen.getByText("Gamma")).toBeInTheDocument();
  });

  it("respects defaultOpenIds for initial state", () => {
    render(
      <GroupedList
        groups={groups()}
        renderItem={(item) => <span>{item.name}</span>}
        keyFor={(item) => item.id}
        defaultOpenIds={["g1"]}
      />,
    );

    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.queryByText("Gamma")).not.toBeInTheDocument();
  });

  it("toggles a group when its header is clicked", () => {
    render(
      <GroupedList
        groups={groups()}
        renderItem={(item) => <span>{item.name}</span>}
        keyFor={(item) => item.id}
      />,
    );

    const header = screen.getByRole("button", { name: /Group 1 2/ });
    fireEvent.click(header);

    expect(screen.queryByText("Alpha")).not.toBeInTheDocument();

    fireEvent.click(header);
    expect(screen.getByText("Alpha")).toBeInTheDocument();
  });

  it("renders default empty message when no groups", () => {
    render(
      <GroupedList groups={[]} renderItem={() => null} keyFor={() => "x"} />,
    );

    expect(screen.getByText("Keine Eintraege")).toBeInTheDocument();
  });

  it("renders custom emptyState", () => {
    render(
      <GroupedList
        groups={[]}
        renderItem={() => null}
        keyFor={() => "x"}
        emptyState={<div data-testid="empty">Nix</div>}
      />,
    );

    expect(screen.getByTestId("empty")).toBeInTheDocument();
  });

  it("applies custom className to outer container", () => {
    const { container } = render(
      <GroupedList
        groups={groups()}
        renderItem={(item) => <span>{item.name}</span>}
        keyFor={(item) => item.id}
        className="custom-class"
      />,
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("uses provided keyFor for list keys", () => {
    const keyFor = vi.fn((item: Item) => item.id);
    render(
      <GroupedList
        groups={groups()}
        renderItem={(item) => <span>{item.name}</span>}
        keyFor={keyFor}
      />,
    );

    expect(keyFor).toHaveBeenCalled();
  });
});
