import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { moreConcept } from "~/lib/concepts/more";
import { settingsConcept } from "~/lib/concepts/settings";
import { PortalBottomNavLink } from "./portal-bottom-nav";
import { PortalMoreBottomNav } from "./portal-more-bottom-nav";

// Die eine Leiste mit „Mehr“ für Tenant-, Eltern- und Träger-Portal (#6580).

const ITEM = { paddingClassName: "px-3", labelClassName: "text-sm" };
const IGNORE_OPEN_CHANGE = () => undefined;

function Bar({
  open = false,
  moreItemActive = false,
  sheetMounted,
  onOpenChange = IGNORE_OPEN_CHANGE,
}: {
  readonly open?: boolean;
  readonly moreItemActive?: boolean;
  readonly sheetMounted?: boolean;
  readonly onOpenChange?: (open: boolean) => void;
}) {
  return (
    <PortalMoreBottomNav
      listClassName="gap-1"
      open={open}
      onOpenChange={onOpenChange}
      moreItemActive={moreItemActive}
      more={{ ...ITEM, label: "Mehr", concept: moreConcept }}
      sheetTitle="Weitere Bereiche"
      sheetDescription="Alle übrigen Seiten"
      sheetMounted={sheetMounted}
      sheet={<p>Inhalt des Blatts</p>}
    >
      <PortalBottomNavLink
        {...ITEM}
        href="/a"
        label="Start"
        concept={settingsConcept}
        active
      />
    </PortalMoreBottomNav>
  );
}

// Ein offenes Blatt verbirgt die übrige Seite vor Vorleseprogrammen, daher
// `hidden`.
function moreButton() {
  return screen.getByRole("button", { name: "Mehr", hidden: true });
}

describe("PortalMoreBottomNav", () => {
  it("opens the sheet from „Mehr“ and announces it as a dialog", () => {
    const onOpenChange = vi.fn();
    render(<Bar onOpenChange={onOpenChange} />);

    expect(moreButton()).toHaveAttribute("aria-haspopup", "dialog");
    expect(moreButton()).toHaveAttribute("aria-expanded", "false");
    expect(moreButton()).toHaveAttribute("data-active", "false");

    fireEvent.click(moreButton());
    expect(onOpenChange).toHaveBeenCalledWith(true);
  });

  it("shows the sheet with its name while open and marks „Mehr“", async () => {
    render(<Bar open />);

    expect(
      await screen.findByRole("dialog", { name: "Weitere Bereiche" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Inhalt des Blatts")).toBeInTheDocument();
    expect(moreButton()).toHaveAttribute("aria-expanded", "true");
    expect(moreButton()).toHaveAttribute("data-active", "true");
  });

  it("marks „Mehr“ when a page behind it is the current one", () => {
    render(<Bar moreItemActive />);
    expect(moreButton()).toHaveAttribute("data-active", "true");
  });

  it("takes the sheet out of the tree while a dialog from it is open", () => {
    render(<Bar open sheetMounted={false} />);

    // Anker: die Leiste ist da.
    expect(moreButton()).toBeInTheDocument();
    expect(screen.queryByText("Inhalt des Blatts")).not.toBeInTheDocument();
  });
});
