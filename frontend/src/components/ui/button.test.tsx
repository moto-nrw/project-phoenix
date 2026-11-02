import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Button, ButtonLink, buttonClassName } from "./button";

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

    expect(screen.getByRole("button")).toHaveTextContent("Loading...");
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

  it("applies secondary variant styles", () => {
    render(<Button variant="secondary">Secondary</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-gray-200");
    expect(button.className).toContain("text-gray-800");
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
        "focus-visible:ring-gray-400",
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
        "bg-gray-50",
        "hover:bg-gray-100",
      ]),
    );
  });

  it("drops every scale animation from the kit", () => {
    const variants = [
      "primary",
      "secondary",
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
      "dashed",
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
