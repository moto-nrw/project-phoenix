import { act, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { settingsConcept } from "~/lib/concepts/settings";
import {
  PortalSidebar,
  PortalSidebarGroup,
  PortalSidebarLink,
} from "./portal-sidebar";

/** Stellt ein, wo die Leiste im Fenster beginnt, wie es der Browser misst. */
function placeSidebarAt(top: number) {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    top,
  } as DOMRect);
}

async function scrollPage() {
  await act(async () => {
    window.dispatchEvent(new Event("scroll"));
    await new Promise((resolve) => requestAnimationFrame(resolve));
  });
}

function renderSidebar() {
  const { container } = render(
    <PortalSidebar secondary={<li>Hilfe</li>}>
      <li>Schulen</li>
    </PortalSidebar>,
  );
  return container.querySelector<HTMLElement>("aside > div")!;
}

describe("PortalSidebar (#5680)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("endet am Fensterrand, wenn über der Kopfzeile ein Hinweis steht", () => {
    placeSidebarAt(108);

    expect(renderSidebar().style.height).toBe("calc(100vh - 108px)");
  });

  it("wächst mit, sobald der Hinweis aus dem Fenster gescrollt wird", async () => {
    placeSidebarAt(108);
    const inner = renderSidebar();

    placeSidebarAt(90);
    await scrollPage();
    expect(inner.style.height).toBe("calc(100vh - 90px)");

    placeSidebarAt(50);
    await scrollPage();
    expect(inner.style.height).toBe("");
  });

  it("passt sich an, wenn der Hinweis erst nach dem Laden erscheint", async () => {
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
    placeSidebarAt(65);
    const inner = renderSidebar();
    expect(inner.style.height).toBe("");

    placeSidebarAt(108);
    await act(async () => {
      notifyResize();
      await new Promise((resolve) => requestAnimationFrame(resolve));
    });

    expect(inner.style.height).toBe("calc(100vh - 108px)");
  });

  it("behält ohne Hinweis die Höhe aus der Klasse", () => {
    placeSidebarAt(65);
    const inner = renderSidebar();

    expect(inner.style.height).toBe("");
    expect(inner).toHaveClass("top-[73px]", "h-[calc(100vh-73px)]");
  });

  it("erzwingt keine eigene Mindesthöhe, die kurze Seiten scrollbar macht", () => {
    placeSidebarAt(65);
    renderSidebar();

    expect(screen.getByRole("complementary")).not.toHaveClass("min-h-screen");
  });

  it("names a group for screen readers and leaves out an empty lower block (#5685)", () => {
    render(
      <PortalSidebar navLabel="Einstellungen">
        <PortalSidebarGroup label="Bereich">
          <PortalSidebarLink
            href="/a"
            label="Ziel"
            concept={settingsConcept}
            active={false}
          />
        </PortalSidebarGroup>
      </PortalSidebar>,
    );

    const nav = screen.getByRole("navigation", { name: "Einstellungen" });
    expect(
      within(within(nav).getByRole("list", { name: "Bereich" })).getByRole(
        "link",
        { name: "Ziel" },
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("navigation")).toHaveLength(1);
  });
});
