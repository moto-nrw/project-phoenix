import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { groupEmptyState } from "./group-empty-state";

function renderEmpty(state: ReturnType<typeof groupEmptyState>) {
  if (!state) throw new Error("no empty state");
  return render(
    <div>
      <p>{state.title}</p>
      <p>{state.description}</p>
      {state.action}
    </div>,
  );
}

describe("groupEmptyState", () => {
  const base = {
    visible: true,
    groupName: "Ferienbetreuung Januar",
    severalGroups: false,
    holidayHint: undefined,
    onOpenSearch: vi.fn(),
  };

  it("sends an ordinary empty group to the child search", () => {
    renderEmpty(groupEmptyState(base));

    expect(
      screen.getByText("Keine Kinder in Ferienbetreuung Januar"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zur Kindersuche" }),
    ).toBeInTheDocument();
  });

  it("explains an empty holiday group before its week instead (#6680)", () => {
    renderEmpty(
      groupEmptyState({
        ...base,
        holidayHint: {
          title: "Feriengruppe: 3 Kinder kommen ab 04.01.2027 dazu.",
          description:
            "Feriengäste stehen erst ab dem ersten Tag in der Gruppe.",
        },
      }),
    );

    expect(
      screen.getByText("Feriengruppe: 3 Kinder kommen ab 04.01.2027 dazu."),
    ).toBeInTheDocument();
    // The guests come by themselves: nobody is sent to assign children.
    expect(screen.queryByText(/Keine Kinder in/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zur Kindersuche" }),
    ).not.toBeInTheDocument();
  });

  it("stays out of the way while the group has children or still loads", () => {
    expect(groupEmptyState({ ...base, visible: false })).toBeNull();
  });
});
