import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { PlanningContextBar, PlanningDayChip } from "./planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "./tabs";

describe("PlanningContextBar", () => {
  it("renders the page title and fires the previous/next navigation callbacks", () => {
    const onPrevious = vi.fn();
    const onNext = vi.fn();
    render(
      <PlanningContextBar
        title="Vertretung"
        onPrevious={onPrevious}
        onNext={onNext}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "Vertretung" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(onPrevious).toHaveBeenCalledTimes(1);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it("renders a plain date label between the arrows when no navigation slot is given", () => {
    render(<PlanningContextBar title="Vertretung" dateLabel="Mo 13.07." />);

    expect(screen.getByText("Mo 13.07.")).toBeInTheDocument();
  });

  it("renders custom navigation content instead of the date label when navigationSlot is given", () => {
    render(
      <PlanningContextBar
        title="Vertretung"
        dateLabel="Mo 13.07."
        navigationSlot={<div data-testid="week-strip">Woche</div>}
      />,
    );

    expect(screen.getByTestId("week-strip")).toBeInTheDocument();
    expect(screen.queryByText("Mo 13.07.")).not.toBeInTheDocument();
  });

  it('shows the "Heute" button only when onToday is provided', () => {
    const { rerender } = render(<PlanningContextBar title="Vertretung" />);
    expect(
      screen.queryByRole("button", { name: "Heute" }),
    ).not.toBeInTheDocument();

    const onToday = vi.fn();
    rerender(<PlanningContextBar title="Vertretung" onToday={onToday} />);
    fireEvent.click(screen.getByRole("button", { name: "Heute" }));
    expect(onToday).toHaveBeenCalledTimes(1);
  });

  it("renders the view-switcher and actions slots", () => {
    render(
      <PlanningContextBar
        title="Vertretung"
        viewSwitcher={<div data-testid="view-switcher" />}
        actions={<button type="button">Speichern</button>}
      />,
    );

    expect(screen.getByTestId("view-switcher")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });

  it("renders the second row only when children are given", () => {
    const { rerender } = render(<PlanningContextBar title="Vertretung" />);
    expect(screen.queryByText("2 Lücken")).not.toBeInTheDocument();

    rerender(
      <PlanningContextBar title="Vertretung">
        <span>2 Lücken</span>
      </PlanningContextBar>,
    );
    expect(screen.getByText("2 Lücken")).toBeInTheDocument();
  });

  it("supports embedding a real ui/Tabs view switcher (Radix activates on mousedown)", () => {
    const onValueChange = vi.fn();
    render(
      <PlanningContextBar
        title="Vertretung"
        viewSwitcher={
          <Tabs defaultValue="stoerungen" onValueChange={onValueChange}>
            <TabsList>
              <TabsTrigger value="stoerungen">Nur Störungen</TabsTrigger>
              <TabsTrigger value="ganzerTag">Ganzer Tag</TabsTrigger>
            </TabsList>
          </Tabs>
        }
      />,
    );

    const tab = screen.getByRole("tab", { name: "Ganzer Tag" });
    fireEvent.mouseDown(tab, { button: 0 });
    expect(onValueChange).toHaveBeenCalledWith("ganzerTag");
  });
});

describe("PlanningDayChip", () => {
  it("renders the weekday label, date label, and a numeric count badge", () => {
    render(<PlanningDayChip weekdayLabel="Mo" dateLabel="13.07." count={2} />);

    expect(screen.getByText("Mo")).toBeInTheDocument();
    expect(screen.getByText("13.07.")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders a placeholder dash when no count is available", () => {
    render(
      <PlanningDayChip weekdayLabel="Sa" dateLabel="18.07." showPlaceholder />,
    );

    expect(screen.getByText("–")).toBeInTheDocument();
  });

  it("uses the dark fill instead of neutral gray when selected", () => {
    render(
      <PlanningDayChip
        weekdayLabel="Mo"
        dateLabel="13.07."
        count={2}
        selected
      />,
    );

    expect(screen.getByRole("button")).toHaveClass("bg-gray-900", "text-white");
  });

  it("fires onClick when clicked", () => {
    const onClick = vi.fn();
    render(
      <PlanningDayChip
        weekdayLabel="Mo"
        dateLabel="13.07."
        count={2}
        onClick={onClick}
      />,
    );

    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
