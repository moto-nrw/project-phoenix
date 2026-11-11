import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";
import { UsersIcon } from "@phosphor-icons/react";

import { SidebarAccordionSection } from "./sidebar-accordion-section";
import { SidebarGroup } from "./sidebar-group";
import { combineSidebarRowBadges, SidebarRowLink } from "./sidebar-row";

// Zähler und Punkt sehen in Zeile, Gruppe und Akkordeon-Bereich gleich aus
// (#6566). Die Zeile ist das Vorbild.

const ICON = <svg aria-hidden="true" />;
const LABEL = "3 ungelesene Anmeldungen";

type RailState = "expanded" | "rail";

function railProps(state: RailState) {
  return state === "rail"
    ? { collapsed: true, labelsMounted: false, labelsVisible: false }
    : { collapsed: false, labelsMounted: true, labelsVisible: true };
}

const SURFACES: ReadonlyArray<
  readonly [string, (state: RailState) => ReactElement]
> = [
  [
    "Zeile",
    (state) => (
      <SidebarRowLink
        href="/a"
        icon={ICON}
        label="Anmeldungen"
        active={false}
        badge={{ count: 3, tone: "orange", ariaLabel: LABEL }}
        {...railProps(state)}
      />
    ),
  ],
  [
    "Gruppe",
    (state) => (
      <SidebarGroup
        label="Eltern"
        icon={UsersIcon}
        isOpen={false}
        onToggle={() => undefined}
        containsActive={false}
        badge={{ count: 3, tone: "orange", ariaLabel: LABEL }}
        {...railProps(state)}
      >
        <span>Inhalt</span>
      </SidebarGroup>
    ),
  ],
  [
    "Bereich",
    (state) => (
      <SidebarAccordionSection
        label="Anmeldungen"
        isExpanded={false}
        onToggle={() => undefined}
        isActive={false}
        hasChildren={false}
        badgeCount={3}
        badgeTone="orange"
        badgeNoun="ungelesene Anmeldungen"
        {...railProps(state)}
      />
    ),
  ],
];

function visibleBadgeClass(element: ReactElement): string {
  const { unmount } = render(element);
  const badge = screen.getByLabelText(LABEL);
  const result = `${badge.className} | ${badge.parentElement?.className ?? ""}`;
  unmount();
  return result;
}

describe("Sidebar-Zähler (#6566)", () => {
  it.each(["expanded", "rail"] as const)(
    "sieht in Zeile, Gruppe und Bereich gleich aus (%s)",
    (state) => {
      const [row, ...others] = SURFACES.map(([name, build]) => ({
        name,
        classes: visibleBadgeClass(build(state)),
      }));
      for (const { name, classes } of others) {
        // Die Gruppen-Kopfzeile ist 32 statt 40 px hoch; ihr Punkt sitzt
        // deshalb 4 px höher und damit an derselben Stelle zum Icon.
        const normalized =
          name === "Gruppe" ? classes.replace("top-0", "top-1") : classes;
        expect({ name, classes: normalized }).toEqual({
          name,
          classes: row?.classes,
        });
      }
    },
  );

  it("zeigt am Bereich bei 0 keinen Zählerplatz", () => {
    render(
      <SidebarAccordionSection
        label="Anmeldungen"
        isExpanded={false}
        onToggle={() => undefined}
        isActive={false}
        hasChildren={false}
        badgeCount={0}
      />,
    );

    // Auf die Bezeichnung folgt direkt der Pfeil: kein leerer Platz, der die
    // Bezeichnung kürzt.
    expect(screen.getByText("Anmeldungen").nextElementSibling?.tagName).toBe(
      "svg",
    );
  });

  it("liest an der Gruppe vor, was gezählt wird", () => {
    render(
      <SidebarGroup
        label="Team"
        icon={UsersIcon}
        isOpen={false}
        onToggle={() => undefined}
        containsActive={false}
        badge={{
          count: 2,
          tone: "orange",
          ariaLabel: "2 offene Tagesinformationen",
        }}
      >
        <span>Inhalt</span>
      </SidebarGroup>,
    );

    expect(
      screen.getByLabelText("2 offene Tagesinformationen"),
    ).toBeInTheDocument();
  });

  it("nennt im Streifen die Gruppe und ihren Zähler", () => {
    render(
      <SidebarGroup
        label="Eltern"
        icon={UsersIcon}
        isOpen={false}
        onToggle={() => undefined}
        containsActive={false}
        badge={{
          count: 5,
          tone: "blue",
          ariaLabel: "2 ungelesene Nachrichten und 3 ungelesene Anmeldungen",
        }}
        {...railProps("rail")}
      >
        <span>Inhalt</span>
      </SidebarGroup>,
    );

    expect(
      screen.getByRole("button", {
        name: "Eltern, 2 ungelesene Nachrichten und 3 ungelesene Anmeldungen",
      }),
    ).toBeInTheDocument();
  });

  it("nennt im Streifen den Bereich und seinen Zähler", () => {
    render(
      <SidebarAccordionSection
        label="Anmeldungen"
        isExpanded={false}
        onToggle={() => undefined}
        isActive={false}
        hasChildren={false}
        badgeCount={3}
        badgeTone="orange"
        badgeNoun="ungelesene Anmeldungen"
        {...railProps("rail")}
      />,
    );

    expect(
      screen.getByRole("button", {
        name: "Anmeldungen, 3 ungelesene Anmeldungen",
      }),
    ).toBeInTheDocument();
  });
});

describe("combineSidebarRowBadges (#6566)", () => {
  it("nennt einen einzelnen Zähler unverändert", () => {
    expect(
      combineSidebarRowBadges(
        [null, { count: 1, tone: "orange", ariaLabel: "1 offene Anfrage" }],
        "orange",
      ),
    ).toEqual({ count: 1, tone: "orange", ariaLabel: "1 offene Anfrage" });
  });

  it("zählt zusammen und nennt jeden Teil", () => {
    expect(
      combineSidebarRowBadges(
        [
          { count: 2, tone: "blue", ariaLabel: "2 ungelesene Nachrichten" },
          { count: 3, tone: "orange", ariaLabel: "3 ungelesene Anmeldungen" },
          { count: 1, tone: "orange", ariaLabel: "1 offene Anfrage" },
        ],
        "blue",
      ),
    ).toEqual({
      count: 6,
      tone: "blue",
      ariaLabel:
        "2 ungelesene Nachrichten, 3 ungelesene Anmeldungen und 1 offene Anfrage",
    });
  });

  it("hat ohne Zähler keinen Zähler", () => {
    expect(
      combineSidebarRowBadges(
        [null, { count: 0, tone: "orange", ariaLabel: "0 offene Anfragen" }],
        "orange",
      ),
    ).toBeNull();
  });
});
