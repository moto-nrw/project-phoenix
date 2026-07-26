import { act, fireEvent, render, screen } from "@testing-library/react";
import type { FormEvent } from "react";
import { describe, expect, it, vi } from "vitest";
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
        vi.useRealTimers();
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
});
