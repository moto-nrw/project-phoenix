import { act, render, screen, waitFor } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { logoutConcept } from "~/lib/concepts/logout";
import { moreConcept } from "~/lib/concepts/more";
import { settingsConcept } from "~/lib/concepts/settings";
import {
  PortalBottomNav,
  PortalBottomNavButton,
  PortalBottomNavLink,
  PortalNavSheetButton,
  PortalNavSheetLink,
  usePortalBottomNavIndicator,
  accessibleBottomNavName,
} from "./portal-bottom-nav";

const ITEM = { paddingClassName: "px-3", labelClassName: "text-sm" };

function BarWithIndicator({ activeKey }: { readonly activeKey: string }) {
  const refs = useRef<Record<string, HTMLElement | null>>({});
  const indicator = usePortalBottomNavIndicator(
    () => refs.current[activeKey] ?? null,
    activeKey,
  );
  return (
    <PortalBottomNav label="Menü" listClassName="gap-1" indicator={indicator}>
      <PortalBottomNavLink
        {...ITEM}
        href="/a"
        label="Start"
        concept={settingsConcept}
        active={activeKey === "start"}
        ref={(element: HTMLAnchorElement | null) => {
          refs.current.start = element;
        }}
      />
      <PortalBottomNavButton
        {...ITEM}
        label="Mehr"
        concept={moreConcept}
        active={activeKey === "more"}
        ref={(element: HTMLButtonElement | null) => {
          refs.current.more = element;
        }}
      />
    </PortalBottomNav>
  );
}

function indicatorElement() {
  return screen
    .getByRole("navigation", { name: "Menü" })
    .querySelector<HTMLElement>('li[aria-hidden="true"]');
}

describe("PortalBottomNav (#5603)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("trägt den übergebenen Namen und ohne Angabe „Hauptnavigation“", () => {
    const { unmount } = render(
      <PortalBottomNav listClassName="gap-1">
        <li>Feld</li>
      </PortalBottomNav>,
    );
    expect(
      screen.getByRole("navigation", { name: "Hauptnavigation" }),
    ).toBeInTheDocument();
    unmount();

    render(
      <PortalBottomNav label="Main menu" listClassName="gap-1">
        <li>Feld</li>
      </PortalBottomNav>,
    );
    expect(
      screen.getByRole("navigation", { name: "Main menu" }),
    ).toBeInTheDocument();
  });

  it("zeigt den Namen nur am aktiven Feld, das aria-label an jedem", () => {
    render(
      <PortalBottomNav listClassName="gap-1">
        <PortalBottomNavLink
          {...ITEM}
          href="/a"
          label="Start"
          concept={settingsConcept}
          active
        />
        <PortalBottomNavButton
          {...ITEM}
          label="Mehr"
          icon={<svg data-testid="eigenes-icon" />}
          active={false}
        />
      </PortalBottomNav>,
    );

    expect(screen.getByRole("link", { name: "Start" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("Start")).toBeVisible();
    expect(screen.getByRole("button", { name: "Mehr" })).toHaveAttribute(
      "data-active",
      "false",
    );
    expect(screen.getByTestId("eigenes-icon")).toBeInTheDocument();
    // Anker: Das aktive Feld zeigt seinen Namen, die Leiste ist also gerendert.
    expect(screen.queryByText("Mehr")).not.toBeInTheDocument();
  });

  it("schiebt die gleitende Fläche unter das Ziel und nimmt sie ohne Ziel weg", async () => {
    vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockReturnValue(12);
    vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(80);

    const { rerender } = render(<BarWithIndicator activeKey="start" />);
    await waitFor(() => {
      expect(indicatorElement()?.style.left).toBe("12px");
    });
    expect(indicatorElement()?.style.width).toBe("80px");

    rerender(<BarWithIndicator activeKey="nichts" />);
    await waitFor(() => {
      expect(indicatorElement()).toBeNull();
    });
    // Anker: Die Leiste selbst bleibt stehen.
    expect(screen.getByRole("link", { name: "Start" })).toBeVisible();
  });

  it("misst neu, wenn das Ziel seine Breite ändert", async () => {
    let notifyResize = () => {};
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          notifyResize = callback;
        }
        observe() {}
        disconnect() {}
      },
    );
    const width = vi
      .spyOn(HTMLElement.prototype, "offsetWidth", "get")
      .mockReturnValue(40);
    render(<BarWithIndicator activeKey="start" />);
    await waitFor(() => {
      expect(indicatorElement()?.style.width).toBe("40px");
    });

    width.mockReturnValue(96);
    act(() => notifyResize());

    expect(indicatorElement()?.style.width).toBe("96px");
  });
});

describe("Zeilen im „Mehr“-Blatt (#5603)", () => {
  it("führt mit Pfeil zum Ziel, `plain` lässt den Pfeil weg", () => {
    const { unmount } = render(
      <PortalNavSheetLink
        href="/a"
        label="Dokumente"
        concept={settingsConcept}
        active={false}
      />,
    );
    expect(
      screen.getByRole("link", { name: "Dokumente" }).querySelectorAll("svg"),
    ).toHaveLength(2);
    unmount();

    render(
      <PortalNavSheetLink
        href="/a"
        label="Dokumente"
        concept={settingsConcept}
        active={false}
        plain
      />,
    );
    expect(
      screen.getByRole("link", { name: "Dokumente" }).querySelectorAll("svg"),
    ).toHaveLength(1);
  });

  it("löst mit der Aktionszeile die Aktion aus", () => {
    const onClick = vi.fn();
    render(
      <PortalNavSheetButton
        label="Abmelden"
        concept={logoutConcept}
        onClick={onClick}
      />,
    );
    screen.getByRole("button", { name: "Abmelden" }).click();
    expect(onClick).toHaveBeenCalledOnce();
  });
});

// WCAG 2.5.3: the name of the active field starts with the word it shows.
describe("accessibleBottomNavName", () => {
  it("keeps the full name when the short word is part of it", () => {
    expect(
      accessibleBottomNavName("Familien und Zahlung", "Familien", true),
    ).toBe("Familien und Zahlung");
    expect(accessibleBottomNavName("Ferienbetreuung", "Ferien", false)).toBe(
      "Ferienbetreuung",
    );
  });

  it("starts with the shown word when it is no word of the name", () => {
    expect(accessibleBottomNavName("Ferienbetreuung", "Ferien", true)).toBe(
      "Ferien, Ferienbetreuung",
    );
  });
});
