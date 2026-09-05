import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { PlanningContextBar, PlanningDayChip } from "./planning-context-bar";
import { SegmentedControl } from "./segmented-control";

describe("PlanningContextBar", () => {
  it("navigiert zurück und weiter und trägt keine eigene Überschrift", () => {
    const onPrevious = vi.fn();
    const onNext = vi.fn();
    const { container } = render(
      <PlanningContextBar onPrevious={onPrevious} onNext={onNext} />,
    );

    // Die Leiste ist ein Bedienband IN der Kopfkarte: keine zweite
    // Überschrift, keine zweite Kartenfläche.
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
    expect(container.querySelector(".moto-content-surface")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(onPrevious).toHaveBeenCalledTimes(1);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it("renders a plain date label between the arrows when no navigation slot is given", () => {
    render(<PlanningContextBar dateLabel="Mo 13.07." />);

    expect(screen.getByText("Mo 13.07.")).toBeInTheDocument();
  });

  it("renders custom navigation content instead of the date label when navigationSlot is given", () => {
    render(
      <PlanningContextBar
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
    const { rerender } = render(<PlanningContextBar />);
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();

    const onToday = vi.fn();
    rerender(<PlanningContextBar onToday={onToday} />);
    const todayButton = screen.getByRole("button", { name: "Heute" });
    expect(todayButton).toBeEnabled();
    fireEvent.click(todayButton);
    expect(onToday).toHaveBeenCalledTimes(1);
  });

  it("puts the navigation slot between the arrows and shows the today button only when it does something", () => {
    // Statistik: der Zeitraum-Chip und die Pfeile, die ihn verschieben, sind
    // EIN Bedienelement. „Heute" steht dann außerhalb der Gruppe und nur,
    // wenn es etwas zurückzusetzen gibt.
    const { rerender } = render(
      <PlanningContextBar
        navigationInGroup
        navigationSlot={<div data-testid="range-chip">1.–30. Aug. 2026</div>}
        previousLabel="Vorheriger Zeitraum"
        nextLabel="Nächster Zeitraum"
        todayLabel="Letzte 30 Tage"
      />,
    );

    const chip = screen.getByTestId("range-chip");
    const previous = screen.getByRole("button", {
      name: "Vorheriger Zeitraum",
    });
    const next = screen.getByRole("button", { name: "Nächster Zeitraum" });
    expect(previous.parentElement).toBe(chip.parentElement?.parentElement);
    expect(next.parentElement).toBe(chip.parentElement?.parentElement);
    expect(
      screen.queryByRole("button", { name: "Letzte 30 Tage" }),
    ).not.toBeInTheDocument();

    const onToday = vi.fn();
    rerender(
      <PlanningContextBar
        navigationInGroup
        navigationSlot={<div data-testid="range-chip">1.–30. Aug. 2026</div>}
        todayLabel="Letzte 30 Tage"
        onToday={onToday}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Letzte 30 Tage" }));
    expect(onToday).toHaveBeenCalledTimes(1);
  });

  it("renders the view-switcher and actions slots", () => {
    render(
      <PlanningContextBar
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
    const { rerender } = render(<PlanningContextBar />);
    expect(screen.queryByText("2 Lücken")).not.toBeInTheDocument();

    rerender(
      <PlanningContextBar>
        <span>2 Lücken</span>
      </PlanningContextBar>,
    );
    expect(screen.getByText("2 Lücken")).toBeInTheDocument();
  });

  it("shows that the mobile context row can scroll horizontally", () => {
    // Den Seitennamen trägt die Kopfkarte, die Bar hat keinen `title` mehr.
    const { container } = render(
      <PlanningContextBar>
        <span>Mo</span>
        <span>Di</span>
      </PlanningContextBar>,
    );

    const contextRow = container.querySelector(".overflow-x-auto");
    expect(contextRow).toHaveClass(
      "scrollbar-thin",
      "planning-context-scrollbar",
    );
    expect(contextRow).not.toHaveClass(
      "[scrollbar-width:none]",
      "[&::-webkit-scrollbar]:hidden",
    );
  });

  it("bricht die Kontextzeile mit `contextRowWrap` um, statt zu scrollen", () => {
    // Betreuungsplan: drei Text-Chips. Eine scrollende Zeile schneidet sie
    // auf dem Telefon ab, ohne dass eine Scrollbar das anzeigt.
    const { container } = render(
      <PlanningContextBar contextRowWrap>
        <span>Bedarf: keine Anmeldung verknüpft</span>
        <span>Keine Lücken</span>
      </PlanningContextBar>,
    );

    expect(container.querySelector(".overflow-x-auto")).toBeNull();
    const contextRow = screen.getByText("Keine Lücken").parentElement;
    expect(contextRow).toHaveClass("flex-wrap");
    expect(contextRow).not.toHaveClass("[&>*]:shrink-0");
  });

  it("nimmt den SegmentedControl-Umschalter der Planungsflächen auf", () => {
    // Die Ansicht ist eine Wertauswahl, kein Inhaltsreiter: alle drei
    // Planungsflächen nutzen dafür dasselbe Bauteil (ui/Tabs als
    // Wertumschalter ist per Ratsche gesperrt).
    const onChange = vi.fn();
    render(
      <PlanningContextBar
        viewSwitcher={
          <SegmentedControl
            ariaLabel="Ansicht"
            value="stoerungen"
            onChange={onChange}
            items={[
              { value: "stoerungen", label: "Nur Störungen" },
              { value: "ganzerTag", label: "Ganzer Tag" },
            ]}
          />
        }
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ganzer Tag" }));
    expect(onChange).toHaveBeenCalledWith("ganzerTag");
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
    expect(screen.getByRole("button")).toHaveClass("scroll-mx-1");
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

  it("scrolls the context row horizontally when a selected day is clipped", () => {
    const { rerender } = render(
      <PlanningDayChip weekdayLabel="Do" dateLabel="16.07." />,
    );
    const chip = screen.getByRole("button");
    const contextRow = chip.parentElement!;
    const scrollTo = vi.fn();
    Object.defineProperty(contextRow, "scrollTo", { value: scrollTo });
    vi.spyOn(chip, "getBoundingClientRect").mockReturnValue({
      left: 282,
      right: 355,
    } as DOMRect);
    vi.spyOn(contextRow, "getBoundingClientRect").mockReturnValue({
      left: 29,
      right: 338,
    } as DOMRect);

    rerender(<PlanningDayChip weekdayLabel="Do" dateLabel="16.07." selected />);
    expect(scrollTo).toHaveBeenCalledWith({
      left: 21,
    });
  });
});
