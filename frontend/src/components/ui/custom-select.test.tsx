import { act, fireEvent, render, screen } from "@testing-library/react";
import type { FormEvent } from "react";
import { flushSync } from "react-dom";
import { describe, expect, it, vi } from "vitest";
import { releaseFakeTimers } from "~/test/clock";
import { CustomSelect } from "./custom-select";

const options = [
  { value: "", label: "Bitte wählen" },
  { value: "first", label: "Erste Option" },
  { value: "second", label: "Zweite Option" },
  { value: "disabled", label: "Gesperrt", disabled: true },
] as const;

describe("CustomSelect", () => {
  it("renders the selected label", () => {
    render(
      <CustomSelect
        value="first"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    expect(screen.getByRole("combobox", { name: "Auswahl" })).toHaveTextContent(
      "Erste Option",
    );
  });

  it("forwards ariaDescribedBy to the trigger", () => {
    render(
      <CustomSelect
        value="first"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
        ariaDescribedBy="auswahl-error auswahl-hint"
      />,
    );

    expect(screen.getByRole("combobox", { name: "Auswahl" })).toHaveAttribute(
      "aria-describedby",
      "auswahl-error auswahl-hint",
    );
  });

  it("names the trigger and the popup listbox from an external label via ariaLabelledBy", () => {
    render(
      <>
        <span id="rolle-label">Rolle</span>
        <CustomSelect
          value="first"
          options={options}
          onChange={vi.fn()}
          id="rolle"
          ariaLabelledBy="rolle-label"
        />
      </>,
    );

    const trigger = screen.getByRole("combobox", { name: "Rolle" });
    fireEvent.click(trigger);

    expect(screen.getByRole("listbox", { name: "Rolle" })).toBeInTheDocument();
  });

  it("opens and selects an option", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
    fireEvent.click(screen.getByRole("option", { name: "Zweite Option" }));

    expect(onChange).toHaveBeenCalledWith("second");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("portals the listbox out of clipping containers", () => {
    // The menu must escape overflow-hidden/scrollable ancestors (modal
    // bodies, cards) — it is portaled to document.body and positioned
    // against the viewport, so no ancestor can clip it.
    render(
      <div data-testid="surface" className="overflow-hidden">
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />
      </div>,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));

    const listbox = screen.getByRole("listbox");
    expect(screen.getByTestId("surface")).not.toContainElement(listbox);
    expect(document.body).toContainElement(listbox);
    expect(listbox.parentElement).toBe(document.body);
  });

  it("closes with Escape", () => {
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Auswahl" });
    fireEvent.click(trigger);
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.keyDown(trigger, { key: "Escape" });

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("closes an open menu when the field becomes disabled", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Auswahl" });
    fireEvent.click(trigger);
    // Anker: Das Menü war vor der Deaktivierung geöffnet.
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    rerender(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
        disabled
      />,
    );

    expect(trigger).toBeDisabled();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("keeps Escape from reaching document listeners while the trigger owns an open listbox", () => {
    const onDocumentKeyDown = vi.fn();
    document.addEventListener("keydown", onDocumentKeyDown);

    try {
      render(
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Auswahl" });
      fireEvent.click(trigger);
      fireEvent.keyDown(trigger, { key: "Escape" });

      expect(onDocumentKeyDown).not.toHaveBeenCalled();
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    } finally {
      document.removeEventListener("keydown", onDocumentKeyDown);
    }
  });

  it("keeps Escape from reaching document listeners while an option owns focus", () => {
    const onDocumentKeyDown = vi.fn();
    document.addEventListener("keydown", onDocumentKeyDown);

    try {
      render(
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />,
      );

      fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
      fireEvent.keyDown(screen.getByRole("option", { name: "Erste Option" }), {
        key: "Escape",
      });

      expect(onDocumentKeyDown).not.toHaveBeenCalled();
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    } finally {
      document.removeEventListener("keydown", onDocumentKeyDown);
    }
  });

  it("ignores disabled options", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
    fireEvent.click(screen.getByRole("option", { name: "Gesperrt" }));

    expect(onChange).not.toHaveBeenCalled();
  });

  it("supports keyboard selection", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Auswahl" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith("first");
  });

  describe("type-ahead", () => {
    const nameOptions = [
      { value: "", label: "Bitte wählen" },
      { value: "anna", label: "Anna Becker" },
      { value: "anton", label: "Anton Maler" },
      { value: "bernd", label: "Bernd Castor" },
      { value: "gesperrt", label: "Gesperrt", disabled: true },
    ] as const;

    const renderNameSelect = (onChange = vi.fn()) => {
      render(
        <CustomSelect
          value=""
          options={nameOptions}
          onChange={onChange}
          ariaLabel="Betreuungsperson"
        />,
      );
      return onChange;
    };

    const activeElement = () => document.activeElement as HTMLElement;

    it("opens and focuses the matching option when typing on the closed trigger", () => {
      renderNameSelect();

      fireEvent.keyDown(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
        { key: "b" },
      );

      expect(screen.getByRole("listbox")).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Bernd Castor" }),
      ).toHaveFocus();
    });

    it("accumulates typed characters into a multi-character match", () => {
      const onChange = renderNameSelect();

      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
      );
      fireEvent.keyDown(activeElement(), { key: "a" });
      fireEvent.keyDown(activeElement(), { key: "n" });
      fireEvent.keyDown(activeElement(), { key: "t" });

      expect(screen.getByRole("option", { name: "Anton Maler" })).toHaveFocus();
      expect(onChange).not.toHaveBeenCalled();
    });

    it("cycles through options sharing a first letter on repeated presses", () => {
      renderNameSelect();

      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
      );
      fireEvent.keyDown(activeElement(), { key: "a" });
      expect(screen.getByRole("option", { name: "Anna Becker" })).toHaveFocus();

      fireEvent.keyDown(activeElement(), { key: "a" });
      expect(screen.getByRole("option", { name: "Anton Maler" })).toHaveFocus();

      fireEvent.keyDown(activeElement(), { key: "a" });
      expect(screen.getByRole("option", { name: "Anna Becker" })).toHaveFocus();
    });

    it("treats Space as part of an active search instead of selecting", () => {
      const onChange = renderNameSelect();

      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
      );
      fireEvent.keyDown(activeElement(), { key: "a" });
      fireEvent.keyDown(activeElement(), { key: "n" });
      fireEvent.keyDown(activeElement(), { key: "n" });
      fireEvent.keyDown(activeElement(), { key: "a" });
      fireEvent.keyDown(activeElement(), { key: " " });
      fireEvent.keyDown(activeElement(), { key: "b" });

      expect(screen.getByRole("option", { name: "Anna Becker" })).toHaveFocus();
      expect(screen.getByRole("listbox")).toBeInTheDocument();
      expect(onChange).not.toHaveBeenCalled();
    });

    it("still selects with Space when no search is in progress", () => {
      const onChange = renderNameSelect();

      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
      );
      fireEvent.keyDown(activeElement(), { key: "ArrowDown" });
      fireEvent.keyDown(activeElement(), { key: " " });

      expect(onChange).toHaveBeenCalledWith("anna");
    });

    it("resets the search buffer after the type-ahead timeout", () => {
      vi.useFakeTimers();
      try {
        renderNameSelect();

        fireEvent.click(
          screen.getByRole("combobox", { name: "Betreuungsperson" }),
        );
        fireEvent.keyDown(activeElement(), { key: "b" });
        expect(
          screen.getByRole("option", { name: "Bernd Castor" }),
        ).toHaveFocus();

        act(() => {
          vi.advanceTimersByTime(600);
        });

        fireEvent.keyDown(activeElement(), { key: "a" });
        expect(
          screen.getByRole("option", { name: "Anna Becker" }),
        ).toHaveFocus();
      } finally {
        releaseFakeTimers();
      }
    });

    it("never focuses a disabled option via type-ahead", () => {
      renderNameSelect();

      fireEvent.click(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
      );
      fireEvent.keyDown(activeElement(), { key: "g" });

      expect(
        screen.getByRole("option", { name: "Gesperrt" }),
      ).not.toHaveFocus();
    });

    it("focuses the first match when typing on a placeholder-only select with no empty option", () => {
      // No empty option and an empty value: selectedIndex is synthesized to the
      // first option, so single-char typeahead must NOT skip past it.
      render(
        <CustomSelect
          value=""
          options={[
            { value: "anna", label: "Anna Becker" },
            { value: "anton", label: "Anton Maler" },
          ]}
          onChange={vi.fn()}
          ariaLabel="Betreuungsperson"
        />,
      );

      fireEvent.keyDown(
        screen.getByRole("combobox", { name: "Betreuungsperson" }),
        { key: "a" },
      );

      expect(screen.getByRole("listbox")).toBeInTheDocument();
      expect(screen.getByRole("option", { name: "Anna Becker" })).toHaveFocus();
    });
  });

  it("keeps a hidden form value", () => {
    render(
      <CustomSelect
        name="phase"
        value="second"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    expect(document.querySelector('input[name="phase"]')).toHaveValue("second");
  });

  it("omits a disabled select from form submission", () => {
    render(
      <CustomSelect
        name="phase"
        value="second"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
        disabled
      />,
    );

    expect(document.querySelector('input[name="phase"]')).toBeDisabled();
  });

  it("blocks submitting a required select with an empty value", () => {
    const onSubmit = vi.fn((event: FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <CustomSelect
          name="phase"
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
          required
        />
        <button type="submit">Speichern</button>
      </form>,
    );

    const form = document.querySelector("form")!;
    expect(form.checkValidity()).toBe(false);
    expect(
      document.querySelector<HTMLSelectElement>("[data-validation-mirror]")
        ?.validity.valueMissing,
    ).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("allows submitting a required select once a value is chosen", () => {
    const onSubmit = vi.fn((event: FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <CustomSelect
          name="phase"
          value="second"
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
          required
        />
        <button type="submit">Speichern</button>
      </form>,
    );

    expect(document.querySelector("form")!.checkValidity()).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("keeps a menu wider than its trigger inside the right viewport edge", () => {
    // A start-aligned menu anchored at a trigger near the right edge would run
    // off screen whenever its content is wider than the trigger, leaving the
    // last options unreachable on narrow layouts.
    const viewportWidth = 360;
    const menuWidth = 240;
    window.innerWidth = viewportWidth;

    const triggerRect = {
      top: 100,
      bottom: 140,
      left: 260,
      right: 340,
      width: 80,
      height: 40,
      x: 260,
      y: 100,
      toJSON: () => ({}),
    } as DOMRect;
    const rectSpy = vi
      .spyOn(HTMLButtonElement.prototype, "getBoundingClientRect")
      .mockReturnValue(triggerRect);
    const widthSpy = vi
      .spyOn(HTMLUListElement.prototype, "offsetWidth", "get")
      .mockReturnValue(menuWidth);

    try {
      render(
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />,
      );

      fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));

      const listbox = screen.getByRole("listbox");
      const left = Number.parseFloat(listbox.style.left);
      expect(left).toBeLessThan(triggerRect.left);
      expect(left + menuWidth).toBeLessThanOrEqual(viewportWidth);
    } finally {
      rectSpy.mockRestore();
      widthSpy.mockRestore();
    }
  });

  it("brings the trigger and its open menu back when content above pushes them out of view", () => {
    // #6356: recipients that load after the calendar's Wiederholung menu
    // opened grew the lists above the field. The trigger slid below the
    // drawer's visible area without a scroll event, the menu followed it, and
    // its options sat outside the viewport where no click could reach them.
    const observers: {
      callback: ResizeObserverCallback;
      targets: Element[];
    }[] = [];
    class RecordingResizeObserver {
      private readonly entry: (typeof observers)[number];
      constructor(callback: ResizeObserverCallback) {
        this.entry = { callback, targets: [] };
        observers.push(this.entry);
      }
      observe(target: Element) {
        this.entry.targets.push(target);
      }
      unobserve() {}
      disconnect() {
        this.entry.targets = [];
      }
    }
    vi.stubGlobal("ResizeObserver", RecordingResizeObserver);
    window.innerHeight = 720;
    let triggerTop = 500;
    const rectSpy = vi
      .spyOn(HTMLButtonElement.prototype, "getBoundingClientRect")
      .mockImplementation(
        () =>
          ({
            top: triggerTop,
            bottom: triggerTop + 40,
            left: 40,
            right: 300,
            width: 260,
            height: 40,
            x: 40,
            y: triggerTop,
            toJSON: () => ({}),
          }) as DOMRect,
      );
    const scrollIntoViewMock = vi.fn(function (this: Element) {
      // The browser scrolls the drawer body so the trigger is visible again.
      if (this.getAttribute("role") === "combobox") triggerTop = 600;
    });
    Element.prototype.scrollIntoView = scrollIntoViewMock;

    try {
      render(
        <div data-testid="drawer-body" style={{ overflowY: "auto" }}>
          <div data-testid="drawer-content">
            <CustomSelect
              value=""
              options={options}
              onChange={vi.fn()}
              ariaLabel="Wiederholung"
            />
          </div>
        </div>,
      );
      const trigger = screen.getByRole("combobox", { name: "Wiederholung" });
      fireEvent.click(trigger);
      const listbox = screen.getByRole("listbox");
      expect(listbox.style.top).toBe("544px");

      const watching = observers.filter((entry) => entry.targets.length > 0);
      expect(watching.flatMap((entry) => entry.targets)).toEqual([
        screen.getByTestId("drawer-content"),
      ]);

      // Content above grows: the trigger is now below the 720 px viewport.
      triggerTop = 1060;
      scrollIntoViewMock.mockClear();
      act(() => {
        for (const entry of watching) entry.callback([], {} as ResizeObserver);
      });

      expect(scrollIntoViewMock).toHaveBeenCalledWith({
        block: "nearest",
        inline: "nearest",
      });
      expect(scrollIntoViewMock.mock.contexts).toContain(trigger);
      // The menu sits at the trigger's restored place, inside the viewport.
      expect(listbox.style.top).toBe(`${600 + 40 + 4}px`);
    } finally {
      rectSpy.mockRestore();
      Element.prototype.scrollIntoView = vi.fn();
      vi.unstubAllGlobals();
    }
  });

  describe("scroll behaviour in long lists", () => {
    const longOptions = Array.from({ length: 40 }, (_, index) => ({
      value: `opt-${index}`,
      label: `Option ${index + 1}`,
    }));

    it("scrolls the selected option into view once when the menu opens", () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      try {
        render(
          <CustomSelect
            value="opt-30"
            options={longOptions}
            onChange={vi.fn()}
            ariaLabel="Lange Liste"
          />,
        );

        fireEvent.click(screen.getByRole("combobox", { name: "Lange Liste" }));

        expect(screen.getByRole("listbox")).toBeInTheDocument();
        expect(scrollIntoViewMock).toHaveBeenCalledTimes(1);
        expect(scrollIntoViewMock).toHaveBeenCalledWith({ block: "nearest" });
        expect(screen.getByRole("option", { name: "Option 31" })).toHaveFocus();
      } finally {
        Element.prototype.scrollIntoView = vi.fn();
      }
    });

    it("does not scroll when the pointer enters an already-visible option", () => {
      // Hover used to set focusIndex and scrollIntoView({ block: "center" }),
      // which re-centred the list under the cursor and made options jump (#2212).
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      try {
        render(
          <CustomSelect
            value="opt-0"
            options={longOptions}
            onChange={vi.fn()}
            ariaLabel="Lange Liste"
          />,
        );

        fireEvent.click(screen.getByRole("combobox", { name: "Lange Liste" }));
        expect(screen.getByRole("listbox")).toBeInTheDocument();

        // Opening may scroll the selected value into view once; that is fine.
        scrollIntoViewMock.mockClear();

        fireEvent.mouseEnter(screen.getByRole("option", { name: "Option 8" }));

        expect(scrollIntoViewMock).not.toHaveBeenCalled();
        expect(screen.getByRole("option", { name: "Option 8" })).toHaveFocus();
      } finally {
        Element.prototype.scrollIntoView = vi.fn();
      }
    });

    it("scrolls the focused option into view on keyboard navigation", () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      try {
        render(
          <CustomSelect
            value="opt-0"
            options={longOptions}
            onChange={vi.fn()}
            ariaLabel="Lange Liste"
          />,
        );

        const trigger = screen.getByRole("combobox", { name: "Lange Liste" });
        fireEvent.click(trigger);
        scrollIntoViewMock.mockClear();

        fireEvent.keyDown(trigger, { key: "ArrowDown" });

        expect(scrollIntoViewMock).toHaveBeenCalled();
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          block: "nearest",
        });
        expect(screen.getByRole("option", { name: "Option 2" })).toHaveFocus();
      } finally {
        Element.prototype.scrollIntoView = vi.fn();
      }
    });

    it("scrolls the selected option on open even when hover moves focus first", () => {
      // Open scroll must use the index captured at open time. If mouseEnter
      // changes focusIndex before the open effect runs, scrolling focusIndex
      // would reveal the hovered row and leave the selection off-screen.
      const scrolledLabels: string[] = [];
      const scrollIntoViewMock = vi.fn(function scrollIntoView(this: Element) {
        scrolledLabels.push(this.textContent ?? "");
        // scrollIntoView on a long list scrolls the listbox; that fires a
        // capture-phase window scroll listener. Without ignoring menu-local
        // scrolls, a new menuStyle re-scrolls the hovered focusIndex.
        const menu = this.closest('[role="listbox"]');
        menu?.dispatchEvent(new Event("scroll", { bubbles: true }));
      });
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      try {
        render(
          <CustomSelect
            value="opt-30"
            options={longOptions}
            onChange={vi.fn()}
            ariaLabel="Lange Liste"
          />,
        );

        const trigger = screen.getByRole("combobox", { name: "Lange Liste" });
        // flushSync opens the menu (layout) without running passive effects;
        // hover then moves focusIndex before the open-scroll effect. Native
        // dispatch (not RTL fireEvent) avoids a per-event act flush that would
        // run effects between the two steps.
        act(() => {
          flushSync(() => {
            trigger.dispatchEvent(
              new MouseEvent("click", { bubbles: true, cancelable: true }),
            );
          });
          const hovered = screen.getByRole("option", { name: "Option 1" });
          // React wires onMouseEnter from mouseover; fire both.
          hovered.dispatchEvent(
            new MouseEvent("mouseover", { bubbles: true, cancelable: true }),
          );
          hovered.dispatchEvent(
            new MouseEvent("mouseenter", { bubbles: true, cancelable: true }),
          );
        });

        expect(scrolledLabels).toContain("Option 31");
        // Hover must not get a follow-up layout re-scroll after open reveal.
        expect(scrolledLabels).not.toContain("Option 1");
        expect(screen.getByRole("option", { name: "Option 1" })).toHaveFocus();
      } finally {
        Element.prototype.scrollIntoView = vi.fn();
      }
    });

    it("scrolls the focused option after layout updates even when hover suppressed scroll", () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      try {
        render(
          <CustomSelect
            value="opt-0"
            options={longOptions}
            onChange={vi.fn()}
            ariaLabel="Lange Liste"
          />,
        );

        fireEvent.click(screen.getByRole("combobox", { name: "Lange Liste" }));
        fireEvent.mouseEnter(screen.getByRole("option", { name: "Option 8" }));
        scrollIntoViewMock.mockClear();

        // Resize repositions the menu (new menuStyle). Hover suppress must not
        // leave the focused option off-screen after the layout change.
        fireEvent(window, new Event("resize"));

        expect(scrollIntoViewMock).toHaveBeenCalledWith({ block: "nearest" });
        expect(screen.getByRole("option", { name: "Option 8" })).toHaveFocus();
      } finally {
        Element.prototype.scrollIntoView = vi.fn();
      }
    });
  });

  it("shows an invalid trigger in red instead of the neutral surface", () => {
    render(
      <CustomSelect
        value="a"
        options={[{ value: "a", label: "Anna" }]}
        onChange={() => undefined}
        ariaLabel="Ersatzperson"
        invalid
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Ersatzperson" });
    expect(trigger).toHaveAttribute("aria-invalid", "true");
    // moto-content-surface is unlayered and would paint over the red border.
    expect(trigger.className).not.toContain("moto-content-surface");
    expect(trigger.className).toContain("border-moto-red");
  });

  it("wraps long labels in the trigger and the menu with wrapLabels instead of cutting them", () => {
    const long = "Ganztagsbetreuung mit Mittagessen und Ausflug · 65,00 €";
    render(
      <CustomSelect
        value="long"
        options={[
          { value: "", label: "Nicht angemeldet" },
          { value: "long", label: long },
        ]}
        onChange={vi.fn()}
        ariaLabel="Woche"
        wrapLabels
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Woche" });
    const shown = screen.getByText(long);
    expect(shown.className).toContain("whitespace-normal");
    expect(shown.className).not.toContain("truncate");
    expect(trigger.className).toContain("min-h-10");

    fireEvent.click(trigger);
    const option = screen.getByRole("option", { name: long });
    expect(option.querySelector(".truncate")).toBeNull();
    expect(option.querySelector(".whitespace-normal")).toHaveTextContent(long);
  });

  describe("field API", () => {
    it("names the field by its visible label and marks it required", () => {
      render(
        <CustomSelect
          label="Raum"
          required
          value=""
          options={options}
          onChange={vi.fn()}
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Raum" });
      expect(trigger).toHaveAttribute("aria-required", "true");
      expect(screen.getByText("*")).toBeInTheDocument();
    });

    it("shows the error under the field and marks the trigger invalid", () => {
      render(
        <CustomSelect
          label="Raum"
          error="Bitte einen Raum wählen."
          value=""
          options={options}
          onChange={vi.fn()}
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Raum" });
      expect(trigger).toHaveAttribute("aria-invalid", "true");
      expect(trigger).toHaveAccessibleDescription("Bitte einen Raum wählen.");
    });

    it("describes the field with its hint", () => {
      render(
        <CustomSelect
          label="Raum"
          hint="Nur freie Räume."
          value=""
          options={options}
          onChange={vi.fn()}
        />,
      );

      expect(
        screen.getByRole("combobox", { name: "Raum" }),
      ).toHaveAccessibleDescription("Nur freie Räume.");
    });

    it("filters a searchable list and keeps the chosen label on the trigger", () => {
      const onChange = vi.fn();
      render(
        <CustomSelect
          ariaLabel="Auswahl"
          searchable
          value="first"
          options={options}
          onChange={onChange}
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Auswahl" });
      fireEvent.click(trigger);
      fireEvent.change(screen.getByRole("textbox", { name: "Suchen" }), {
        target: { value: "zweite" },
      });

      expect(screen.queryByRole("option", { name: "Erste Option" })).toBeNull();
      expect(trigger).toHaveTextContent("Erste Option");

      fireEvent.click(screen.getByRole("option", { name: "Zweite Option" }));
      expect(onChange).toHaveBeenCalledWith("second");
    });

    it("says so when the search finds nothing", () => {
      render(
        <CustomSelect
          ariaLabel="Auswahl"
          searchable
          value=""
          options={options}
          onChange={vi.fn()}
        />,
      );

      fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
      fireEvent.change(screen.getByRole("textbox", { name: "Suchen" }), {
        target: { value: "xyz" },
      });

      expect(screen.getByText("Keine Treffer.")).toBeInTheDocument();
    });

    it("closes the panel when Tab leaves the search field", () => {
      render(
        <CustomSelect
          ariaLabel="Auswahl"
          searchable
          value=""
          options={options}
          onChange={vi.fn()}
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Auswahl" });
      fireEvent.click(trigger);
      const search = screen.getByRole("textbox", { name: "Suchen" });
      // Anker: Die Suche liegt im geöffneten Listbox-Panel.
      expect(screen.getByRole("listbox")).toBeInTheDocument();

      fireEvent.keyDown(search, { key: "Tab" });

      expect(trigger).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
      expect(trigger).toHaveFocus();
    });

    it("steps from the search field into the list and selects with Enter", () => {
      const onChange = vi.fn();
      render(
        <CustomSelect
          ariaLabel="Auswahl"
          searchable
          value="first"
          options={options}
          onChange={onChange}
        />,
      );

      fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
      fireEvent.keyDown(screen.getByRole("textbox", { name: "Suchen" }), {
        key: "ArrowDown",
      });
      const focused = screen.getByRole("option", { name: "Zweite Option" });
      expect(focused).toHaveFocus();
      fireEvent.keyDown(focused, { key: "Enter" });

      expect(onChange).toHaveBeenCalledWith("second");
      expect(screen.queryByRole("listbox")).toBeNull();
    });
  });
});
