import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Button, ButtonLink, buttonClassName } from "./button";
import type { ButtonVariant } from "./button-styles";

describe("Button", () => {
  it("renders children correctly", () => {
    render(<Button>Click me</Button>);

    expect(
      screen.getByRole("button", { name: "Click me" }),
    ).toBeInTheDocument();
  });

  it("renders with default type submit", () => {
    render(<Button>Submit</Button>);

    expect(screen.getByRole("button")).toHaveAttribute("type", "submit");
  });

  it("shows loading text when isLoading is true", () => {
    render(<Button isLoading>Submit</Button>);

    expect(screen.getByRole("button")).toHaveTextContent("Wird geladen …");
    expect(screen.queryByText("Submit")).not.toBeInTheDocument();
  });

  it("shows custom loading text when provided", () => {
    render(
      <Button isLoading loadingText="Saving...">
        Submit
      </Button>,
    );

    expect(screen.getByRole("button")).toHaveTextContent("Saving...");
  });

  it("is disabled when isLoading is true", () => {
    render(<Button isLoading>Submit</Button>);

    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("stays locked while loading even when the caller passes disabled={false}", () => {
    const handleClick = vi.fn();
    render(
      <Button isLoading disabled={false} onClick={handleClick}>
        Submit
      </Button>,
    );

    const button = screen.getByRole("button");
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(handleClick).not.toHaveBeenCalled();
  });

  it("calls onClick handler when clicked", () => {
    const handleClick = vi.fn();
    render(<Button onClick={handleClick}>Click</Button>);

    fireEvent.click(screen.getByRole("button"));

    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it("applies primary variant styles by default", () => {
    render(<Button>Primary</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-gray-900");
    expect(button.className).toContain("text-white");
  });

  it("applies outline variant styles", () => {
    render(<Button variant="outline">Outline</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-transparent");
  });

  it("applies outline_danger variant styles", () => {
    render(<Button variant="outline_danger">Danger Outline</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-moto-red-soft");
    expect(button.className).toContain("text-moto-red-strong");
  });

  it("applies danger variant styles", () => {
    render(<Button variant="danger">Danger</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-moto-red");
    // Weiss, nicht gray-950: gray-950 auf moto-red ergibt 4,17:1 und verfehlt
    // AA fuer normalen Text. Weiss erreicht 4,83:1 und behaelt es auch im
    // Hover ueber moto-red-strong (6,47:1).
    expect(button.className).toContain("text-white");
    expect(button.className).toContain("hover:bg-moto-red-strong");
  });

  it("applies warning variant styles", () => {
    render(<Button variant="warning">Warning</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-moto-orange");
    // Dunkler Text: Weiss auf moto-orange erreicht nur 2,4:1.
    expect(button.className).toContain("text-gray-950");
    expect(button.className).toContain("hover:bg-moto-orange-hover");
  });

  it("applies success variant styles", () => {
    render(<Button variant="success">Success</Button>);

    const button = screen.getByRole("button");
    // #5141: weiße Schrift auf dem dunklen Füllton (4,84:1), nicht mehr
    // dunkle Schrift auf moto-green. Hover und Active bleiben über AA.
    expect(button.className).toContain("bg-moto-green-fill");
    expect(button.className).toContain("text-white");
    expect(button.className).toContain("hover:bg-moto-green-fill-hover");
    expect(button.className).toContain("active:bg-moto-green-fill-active");
    expect(button.className).not.toContain("text-gray-950");
  });

  it("applies login-style surface variant styles", () => {
    render(<Button variant="surface">Surface</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-white");
    expect(button.className).toContain("ring-gray-200");
    expect(button.className).toContain("shadow-sm");
    expect(button.className).toContain("hover:ring-gray-300");
    expect(button.className).toContain("hover:bg-gray-50");
  });

  it("renders the link variant as a text button with a visible focus ring", () => {
    render(
      <Button variant="link" size="md">
        Passwort vergessen?
      </Button>,
    );

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "bg-transparent",
        "hover:underline",
        "focus-visible:ring-2",
        "focus-visible:ring-gray-600",
      ]),
    );
    // Ein Textknopf hat kein Polster und keinen Schatten der Größe.
    expect(classes).toEqual(expect.arrayContaining(["px-0", "py-0"]));
    expect(classes).not.toContain("px-4");
    expect(classes.some((c) => /^shadow-(?:sm|md|lg)$/.test(c))).toBe(false);
  });

  it("renders ghost_tone in the text colour of the surface it sits on", () => {
    render(<Button variant="ghost_tone">Wiederholen</Button>);

    const classes = screen.getByRole("button").className.split(" ");
    // Auf einer roten Fehlerfläche rot, auf einer grünen grün (#5531).
    expect(classes).toEqual(
      expect.arrayContaining([
        "bg-transparent",
        "text-current",
        "hover:bg-black/5",
        "focus-visible:ring-2",
      ]),
    );
    expect(classes).not.toContain("text-gray-600");
  });

  it("renders ghost_danger as a quiet red button", () => {
    render(<Button variant="ghost_danger">Entfernen</Button>);

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "bg-transparent",
        "text-moto-red-strong",
        "hover:bg-moto-red-soft",
      ]),
    );
    expect(classes).not.toContain("shadow-md");
  });

  it("renders fab as a round green floating button, whatever the size", () => {
    render(
      <Button variant="fab" size="icon" aria-label="Aktivität erstellen">
        +
      </Button>,
    );

    const classes = screen
      .getByRole("button", { name: "Aktivität erstellen" })
      .className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "size-14",
        "rounded-full",
        "bg-moto-green-fill",
        "text-white",
        "shadow-lg",
      ]),
    );
    expect(classes).not.toContain("h-8");
    expect(classes).not.toContain("rounded-md");
  });

  it("renders dashed as the add surface", () => {
    render(<Button variant="dashed">Weitere Nummer hinzufügen</Button>);

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "border",
        "border-dashed",
        "border-gray-300",
        "bg-transparent",
        "hover:bg-gray-50",
      ]),
    );
  });

  // #6182: Varianten für Stellen, die ihr Aussehen bisher per className am
  // Kit-Knopf oder als eigenen Knopf bauten.
  it("renders link_tone as an underlined text button in the notice colour that may wrap", () => {
    render(
      <Button variant="link_tone" size="compact">
        Wiederholen
      </Button>,
    );

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "text-current",
        "underline",
        "h-auto",
        "min-h-8",
        "hover:bg-black/5",
      ]),
    );
    expect(classes).not.toContain("h-8");
  });

  it("renders filter as the round 40 px filter toggle, blue while expanded", () => {
    render(
      <Button variant="filter" size="icon" aria-label="Filter" aria-expanded>
        F
      </Button>,
    );

    const button = screen.getByRole("button", {
      name: "Filter",
      expanded: true,
    });
    const classes = button.className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "h-10",
        "rounded-2xl",
        "bg-white",
        "aria-expanded:bg-moto-blue",
        "data-filters-active:ring-moto-blue",
      ]),
    );
    expect(classes).not.toContain("h-8");
    expect(classes).not.toContain("rounded-md");
  });

  it("renders segment without its own radius, filling its group", () => {
    render(
      <Button variant="segment" size="compact">
        Heute
      </Button>,
    );

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining(["h-full", "rounded-none", "px-3", "text-xs"]),
    );
    expect(classes).not.toContain("h-8");
    expect(classes).not.toContain("rounded-md");
  });

  it("keeps an icon segment square: no side padding beside the divider", () => {
    render(
      <Button variant="segment" size="icon" aria-label="Zurück">
        ‹
      </Button>,
    );

    const button = screen.getByRole("button", { name: "Zurück" });
    expect(button).toHaveAttribute("data-icon-only");
    expect(button.className.split(" ")).toContain("data-icon-only:px-0");
  });

  it("gives a text button only the 44 px height as touch target, not a square", () => {
    render(
      <Button variant="link_tone" size="md" touchTarget>
        Wiederholen
      </Button>,
    );

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toContain("h-11");
    expect(classes).not.toContain("w-11");
    expect(classes).not.toContain("h-auto");
  });

  it("renders disclosure as ghost that also tints on hover of its card head", () => {
    render(
      <Button variant="disclosure" size="icon" aria-label="Einklappen">
        v
      </Button>,
    );

    const classes = screen.getByRole("button").className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "hover:bg-gray-100",
        "group-hover/collapse-head:bg-gray-100",
        "group-hover/collapse-head:text-gray-900",
      ]),
    );
  });

  it("renders day_chip as a 36 px day with a dark pressed state, whatever the size", () => {
    render(
      <Button variant="day_chip" size="md" aria-pressed>
        Mo 14.07.
      </Button>,
    );

    const button = screen.getByRole("button", { pressed: true });
    const classes = button.className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "h-9",
        "rounded-md",
        "px-2.5",
        "text-sm",
        "font-normal",
        "aria-pressed:bg-gray-900",
        "aria-pressed:text-white",
      ]),
    );
    expect(classes).not.toContain("px-4");
    expect(classes).not.toContain("py-2");
    expect(classes).not.toContain("font-medium");
  });

  it("renders day_chip_stacked as a two-line day for the phone strip", () => {
    render(
      <Button variant="day_chip_stacked" size="md" aria-pressed={false}>
        Mo
      </Button>,
    );

    const classes = screen
      .getByRole("button", { pressed: false })
      .className.split(" ");
    expect(classes).toEqual(
      expect.arrayContaining([
        "flex-col",
        "justify-start",
        "rounded-lg",
        "px-1",
        "py-1.5",
        "aria-pressed:bg-gray-900",
      ]),
    );
    expect(classes).not.toContain("justify-center");
  });

  // #5574: Knöpfe sind weiß, schwarz oder farbig. Ein hellgrau gefüllter
  // Ruhezustand passt in keine Rangstufe; Zustände (hover:, active:) dürfen
  // grau tönen.
  it("rests no variant on a grey fill", () => {
    // Record statt Liste: eine neue Variante fehlt hier sonst still.
    const allVariants: Record<ButtonVariant, true> = {
      primary: true,
      outline: true,
      outline_danger: true,
      danger: true,
      warning: true,
      success: true,
      surface: true,
      surface_success: true,
      ghost: true,
      ghost_danger: true,
      ghost_tone: true,
      link: true,
      fab: true,
      send: true,
      ghost_round: true,
      dashed: true,
      link_tone: true,
      filter: true,
      segment: true,
      disclosure: true,
      day_chip: true,
      day_chip_stacked: true,
    };
    for (const variant of Object.keys(allVariants) as ButtonVariant[]) {
      const greyRest = buttonClassName({ variant, size: "md", className: "" })
        .split(" ")
        .filter((token) => /^bg-gray-[1-4]00$/.test(token));
      expect({ variant, greyRest }).toEqual({ variant, greyRest: [] });
    }
  });

  it("drops every scale animation from the kit", () => {
    const variants = [
      "primary",
      "outline",
      "outline_danger",
      "danger",
      "warning",
      "success",
      "surface",
      "ghost",
      "ghost_danger",
      "ghost_tone",
      "link",
      "fab",
      "send",
      "ghost_round",
      "dashed",
      "link_tone",
      "filter",
      "segment",
      "disclosure",
      "day_chip",
      "day_chip_stacked",
    ] as const;
    for (const variant of variants) {
      expect(
        buttonClassName({ variant, size: "md", className: "" }),
      ).not.toMatch(/scale-/);
    }
  });

  it("applies sm size styles", () => {
    render(<Button size="sm">Small</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-sm");
  });

  it("applies md size styles", () => {
    render(<Button size="md">Medium</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("px-4");
    expect(button.className).toContain("py-2");
    expect(button.className).toContain("text-sm");
  });

  it("applies base size styles by default", () => {
    render(<Button>Base</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-base");
  });

  it("applies lg size styles", () => {
    render(<Button size="lg">Large</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-lg");
  });

  it("applies xl size styles", () => {
    render(<Button size="xl">Extra Large</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-xl");
  });

  it("applies the 40 px field size, flush with Input controlSize compact", () => {
    render(<Button size="field">Code senden</Button>);

    const classes = screen.getByRole("button").className.split(" ");
    // Input controlSize="compact" ist `h-10 rounded-lg … text-sm`.
    expect(classes).toEqual(
      expect.arrayContaining(["h-10", "rounded-lg", "px-4", "text-sm"]),
    );
    expect(classes).toContain("font-medium");
    expect(classes.some((c) => /^py-/.test(c))).toBe(false);
  });

  it("provides an explicit 44 px target when a viewport requires it", () => {
    render(
      <Button type="button" size="icon" touchTarget aria-label="Schließen">
        ×
      </Button>,
    );

    expect(screen.getByRole("button", { name: "Schließen" })).toHaveClass(
      "h-11",
      "w-11",
    );
  });

  describe("finger rule (pointer: coarse)", () => {
    const sizes = [
      "sm",
      "md",
      "field",
      "base",
      "lg",
      "xl",
      "compact",
      "icon",
      "card",
    ] as const;

    it.each(sizes)(
      "keeps size %s at least 44 px high under a finger",
      (size) => {
        const classes = buttonClassName({
          variant: "primary",
          size,
          className: "",
        }).split(" ");
        expect(classes).toContain("pointer-coarse:min-h-11");
      },
    );

    it("keeps the icon button at least 44 x 44 px under a finger", () => {
      const classes = buttonClassName({
        variant: "ghost",
        size: "icon",
        className: "",
      }).split(" ");
      expect(classes).toEqual(
        expect.arrayContaining([
          "pointer-coarse:min-h-11",
          "pointer-coarse:min-w-11",
        ]),
      );
    });

    it("leaves the mouse sizes untouched: every new class is pointer-coarse", () => {
      // Mausmaße der Bestandsgrößen (vor #5531) — unabhängige Quelle ist die
      // alte Klassenkette, nicht die Implementierung.
      const mouse: Record<string, string> = {
        md: "rounded-lg px-4 py-2 text-sm",
        compact: "h-8 gap-1.5 rounded-md px-2.5 text-xs",
        icon: "aspect-square h-8 rounded-md text-xs",
        base: "rounded-lg px-5 py-3 text-base",
      };
      for (const [size, expected] of Object.entries(mouse)) {
        const classes = buttonClassName({
          variant: "primary",
          size: size as (typeof sizes)[number],
          className: "",
        }).split(" ");
        expect(classes).toEqual(expect.arrayContaining(expected.split(" ")));
        const sizing = classes.filter((c) =>
          /^(?:min-)?[hw]-|^size-|^p[xy]?-/.test(c),
        );
        expect(sizing.every((c) => expected.split(" ").includes(c))).toBe(true);
      }
    });
  });

  it("applies card size styles", () => {
    render(<Button size="card">Klasse 1a</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("w-full");
    expect(button.className).toContain("rounded-2xl");
    expect(button.className).toContain("p-4");
  });

  it("passes custom className", () => {
    render(<Button className="custom-class">Custom</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("custom-class");
  });

  it("passes additional props", () => {
    render(
      <Button data-testid="custom-button" aria-label="Custom action">
        Custom
      </Button>,
    );

    expect(screen.getByTestId("custom-button")).toBeInTheDocument();
    expect(screen.getByLabelText("Custom action")).toBeInTheDocument();
  });

  it("can be disabled via disabled prop", () => {
    render(<Button disabled>Disabled</Button>);

    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("renders links with the shared button styles", () => {
    render(
      <ButtonLink href="/parents/children" variant="outline" size="md">
        Meine Kinder
      </ButtonLink>,
    );

    const link = screen.getByRole("link", { name: "Meine Kinder" });
    expect(link).toHaveAttribute("href", "/parents/children");
    expect(link.className).toContain("px-4");
    expect(link.className).toContain("ring-gray-300");
  });
});

describe("noTooltip", () => {
  const reason = "Abbrechen schließt nur den Dialog.";

  it("keeps the reason out of the DOM and the accessible description", () => {
    const { container } = render(
      <>
        <Button type="button" noTooltip={reason}>
          Abbrechen
        </Button>
        <ButtonLink href="/zurueck" noTooltip={reason}>
          Zurück
        </ButtonLink>
      </>,
    );

    const button = screen.getByRole("button", { name: "Abbrechen" });
    const link = screen.getByRole("link", { name: "Zurück" });
    for (const element of [button, link]) {
      expect(element).not.toHaveAttribute("notooltip");
      expect(element).not.toHaveAttribute("noTooltip");
      expect(element).not.toHaveAccessibleDescription();
    }
    expect(container).toHaveTextContent("AbbrechenZurück");
    expect(container.innerHTML).not.toContain(reason);
  });

  it("rejects tooltip and noTooltip together at type level", () => {
    // Der Nachweis ist `@ts-expect-error`: tsc schlägt fehl, wenn die
    // Kombination erlaubt wäre.
    const invalid = (
      // @ts-expect-error: a button either explains itself or names a reason
      <Button type="button" tooltip="Schließt den Dialog." noTooltip={reason}>
        Abbrechen
      </Button>
    );
    expect(invalid.props).toMatchObject({ noTooltip: reason });
  });
});
