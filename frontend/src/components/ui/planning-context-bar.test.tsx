import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { PlanningContextBar, PlanningDayChip } from "./planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "./tabs";

describe("PlanningContextBar", () => {
  it("shows the page title on mobile and hides it from the duplicate desktop header", () => {
    const onPrevious = vi.fn();
    const onNext = vi.fn();
    render(
      <PlanningContextBar
        title="Vertretung"
        onPrevious={onPrevious}
        onNext={onNext}
      />,
    );

    expect(screen.getByRole("heading", { name: "Vertretung" })).toHaveClass(
      "md:sr-only",
    );
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

  it('keeps the "Heute" button in place and only enables it with onToday', () => {
    // #2031: der Button verschwindet nicht mehr, wenn er wirkungslos ist —
    // ein auftauchendes und verschwindendes Segment ließ die Navigationsgruppe
    // seitlich springen. Ohne Callback ist er deaktiviert.
    const { rerender } = render(<PlanningContextBar title="Vertretung" />);
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();

    const onToday = vi.fn();
    rerender(<PlanningContextBar title="Vertretung" onToday={onToday} />);
    const todayButton = screen.getByRole("button", { name: "Heute" });
    expect(todayButton).toBeEnabled();
    fireEvent.click(todayButton);
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

  it("reserves the context row even without children", () => {
    // #2031: die zweite Zeile wird IMMER gerendert, sonst ist die Bar auf einer
    // Fläche ohne Kontext eine Zeile niedriger und der Inhalt darunter springt
    // beim Seitenwechsel. Leer heißt leer, nicht abwesend.
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

  it("stays quiet on a day without open gaps", () => {
    // #2031: eine Null ist keine Information, die eine Ziffer verdient — nur
    // Tage mit offenen Lücken tragen einen Zähler.
    render(<PlanningDayChip weekdayLabel="Di" dateLabel="14.07." count={0} />);

    expect(screen.getByText("Di")).toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("stays quiet when no count is available at all", () => {
    // Ein Tag ohne verfügbaren Zähler (vergangener Tag, Abruf übersprungen
    // oder fehlgeschlagen) zeigt NICHTS. Früher stand dort ein Strich, dessen
    // Bedeutung man nur mit Kenntnis der Ladelogik erraten konnte.
    render(<PlanningDayChip weekdayLabel="Sa" dateLabel="18.07." />);

    expect(screen.getByText("Sa")).toBeInTheDocument();
    expect(screen.getByText("18.07.")).toBeInTheDocument();
    expect(screen.queryByText("–")).not.toBeInTheDocument();
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
