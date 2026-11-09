import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { act, render, renderHook, screen } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TenantPage } from "~/components/ui/tenant-page";
import { ToastProvider, useToast } from "~/contexts/ToastContext";
import { DESKTOP_WIDTH, setViewportWidth } from "~/test/helpers/viewport";
import {
  BELOW_LG,
  BELOW_MD,
  BELOW_SM,
  BELOW_XL,
  TAILWIND_BREAKPOINTS,
  type TailwindBreakpoint,
  atLeast,
  below,
  between,
  useMediaQuery,
  useResolvedMediaQuery,
} from "./use-media-query";

/*
 * Happy DOM rechnet ein `rem` in einer Media Query mit der Schriftgröße der
 * Wurzel. Im Browser ist es die Standardschrift, die auch die Wurzel erbt;
 * eine andere Wurzel-Schriftgröße bildet also eine andere
 * Browser-Standardschrift nach.
 */
function setDefaultFontSize(px: number): void {
  document.documentElement.style.fontSize = `${px}px`;
}

afterEach(() => {
  document.documentElement.style.fontSize = "";
  setViewportWidth(DESKTOP_WIDTH);
});

/** So kompiliert Tailwind v4 `<bp>:`; das ist die Entscheidung des CSS. */
function cssVariantApplies(breakpoint: TailwindBreakpoint): boolean {
  return window.matchMedia(`(width >= ${TAILWIND_BREAKPOINTS[breakpoint]})`)
    .matches;
}

const BREAKPOINTS = Object.keys(TAILWIND_BREAKPOINTS) as TailwindBreakpoint[];
const FONT_SIZES = [16, 20, 12];

function boundaryPx(breakpoint: TailwindBreakpoint, fontSize: number): number {
  return Number.parseFloat(TAILWIND_BREAKPOINTS[breakpoint]) * fontSize;
}

/** Knapp unter, genau auf und knapp über jeder Grenze, je Schriftgröße. */
const BOUNDARY_CASES = BREAKPOINTS.flatMap((breakpoint) =>
  FONT_SIZES.flatMap((fontSize) => {
    const boundary = boundaryPx(breakpoint, fontSize);
    return [boundary - 0.5, boundary, boundary + 0.5].map((width) => ({
      breakpoint,
      fontSize,
      width,
      isBelow: width < boundary,
    }));
  }),
);

describe("Tailwind-Breakpoints", () => {
  it("stimmen mit dem Theme überein, aus dem Tailwind die Varianten baut", () => {
    const themeCss = readFileSync(
      join(process.cwd(), "node_modules/tailwindcss/theme.css"),
      "utf8",
    );
    const globalsCss = readFileSync(
      join(process.cwd(), "src/styles/globals.css"),
      "utf8",
    );
    const breakpoint = /--breakpoint-([a-z0-9]+):\s*([^;]+);/g;
    const theme = new Map<string, string>();
    // globals.css kommt nach dem Tailwind-Theme und würde es überschreiben.
    for (const css of [themeCss, globalsCss]) {
      for (const [, name, value] of css.matchAll(breakpoint)) {
        theme.set(name!, value!.trim());
      }
    }

    for (const [name, value] of Object.entries(TAILWIND_BREAKPOINTS)) {
      expect(theme.get(name), `--breakpoint-${name}`).toBe(value);
    }
  });

  it("stehen im Code nirgends als px-Breite in einer Media Query", () => {
    const pxWidthQuery = /\((?:min|max)-width:\s*\d+(?:\.\d+)?px\)/;
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) walk(path);
        else if (
          /\.(tsx?|css)$/.test(entry.name) &&
          !/\.(test|stories)\.tsx?$/.test(entry.name) &&
          pxWidthQuery.test(readFileSync(path, "utf8"))
        ) {
          offenders.push(path);
        }
      }
    };
    walk(join(process.cwd(), "src"));

    expect(offenders).toEqual([]);
  });
});

describe("below / atLeast / between", () => {
  it("bauen die Abfragen aus den rem-Grenzen", () => {
    expect(BELOW_SM).toBe("not all and (min-width: 40rem)");
    expect(BELOW_MD).toBe("not all and (min-width: 48rem)");
    expect(BELOW_LG).toBe("not all and (min-width: 64rem)");
    expect(BELOW_XL).toBe("not all and (min-width: 80rem)");
    expect(between("md", "lg")).toBe("(48rem <= width < 64rem)");
  });

  it.each(BOUNDARY_CASES)(
    "$breakpoint bei $width px und $fontSize px Standardschrift: unten=$isBelow",
    ({ breakpoint, fontSize, width, isBelow }) => {
      setDefaultFontSize(fontSize);
      setViewportWidth(width);

      expect(window.matchMedia(below(breakpoint)).matches).toBe(isBelow);
      expect(window.matchMedia(atLeast(breakpoint)).matches).toBe(!isBelow);
      expect(cssVariantApplies(breakpoint)).toBe(!isBelow);
    },
  );

  it.each(
    FONT_SIZES.flatMap((fontSize) => {
      const from = boundaryPx("md", fontSize);
      const until = boundaryPx("lg", fontSize);
      return [
        { fontSize, width: from - 0.5, inside: false },
        { fontSize, width: from, inside: true },
        { fontSize, width: until - 0.5, inside: true },
        { fontSize, width: until, inside: false },
      ];
    }),
  )(
    "between(md, lg) bei $width px und $fontSize px Standardschrift: $inside",
    ({ fontSize, width, inside }) => {
      setDefaultFontSize(fontSize);
      setViewportWidth(width);

      expect(window.matchMedia(between("md", "lg")).matches).toBe(inside);
      expect(cssVariantApplies("md") && !cssVariantApplies("lg")).toBe(inside);
    },
  );
});

describe("useMediaQuery", () => {
  it("ist im Server-Render false, auch auf einem Telefon", () => {
    setViewportWidth(390);
    function Probe() {
      return <span>{String(useMediaQuery(BELOW_MD))}</span>;
    }

    // Anker: der Server-Render zeigt den Wert.
    expect(renderToString(<Probe />)).toContain("false");
  });

  it("folgt einer Verengung über eine Bruchteil-Grenze bei 20 px Schrift", () => {
    setDefaultFontSize(20);
    setViewportWidth(960);
    const { result } = renderHook(() => useMediaQuery(BELOW_MD));
    expect(result.current).toBe(false);

    setViewportWidth(959.5);

    expect(result.current).toBe(true);
  });

  it("meldet sich beim Unmount von genau der abonnierten Liste ab", () => {
    const lists: MediaQueryList[] = [];
    const realMatchMedia = window.matchMedia.bind(window);
    vi.spyOn(window, "matchMedia").mockImplementation((query) => {
      const list = realMatchMedia(query);
      vi.spyOn(list, "addEventListener");
      vi.spyOn(list, "removeEventListener");
      lists.push(list);
      return list;
    });

    const { unmount } = renderHook(() => useMediaQuery(BELOW_SM));
    const [list] = lists;
    const listener = vi.mocked(list!.addEventListener).mock.calls[0]?.[1];
    expect(listener).toBeTypeOf("function");
    unmount();

    expect(list!.removeEventListener).toHaveBeenCalledWith("change", listener);
  });
});

describe("useResolvedMediaQuery", () => {
  it("ist im Server-Render unbekannt (null)", () => {
    function Probe() {
      return <span>{String(useResolvedMediaQuery(atLeast("md")))}</span>;
    }

    // Anker: der Server-Render zeigt den Wert.
    expect(renderToString(<Probe />)).toContain("null");
  });

  it("entscheidet bei 767,5 px wie CSS: noch kein md", () => {
    setViewportWidth(767.5);
    const { result } = renderHook(() => useResolvedMediaQuery(atLeast("md")));

    expect(result.current).toBe(false);
  });
});

/*
 * Die Verbraucher müssen dieselbe Darstellung wählen wie die Tailwind-Klassen,
 * die sie umgeben: ToastProvider rendert Toasts nur in den Container, den CSS
 * gerade zeigt (`md:hidden` bzw. `hidden md:flex`), TenantPage legt den einen
 * Tabulator-Stopp auf den Reiter, den CSS sichtbar lässt (`max-sm:hidden`
 * blendet „Mehr“ auf dem Telefon aus).
 */
describe("Verbraucher entscheiden wie CSS", () => {
  const CONSUMER_CASES = FONT_SIZES.flatMap((fontSize) =>
    (["sm", "md"] as const).flatMap((breakpoint) => {
      const boundary = boundaryPx(breakpoint, fontSize);
      return [boundary - 0.5, boundary, boundary + 0.5].map((width) => ({
        breakpoint,
        fontSize,
        width,
      }));
    }),
  );

  function ShowToast() {
    const toast = useToast();
    return (
      <button type="button" onClick={() => toast.info("Gespeichert")}>
        Auslösen
      </button>
    );
  }

  it.each(CONSUMER_CASES.filter((entry) => entry.breakpoint === "md"))(
    "ToastProvider bei $width px und $fontSize px Standardschrift",
    async ({ fontSize, width }) => {
      setDefaultFontSize(fontSize);
      setViewportWidth(width);
      render(
        <ToastProvider>
          <ShowToast />
        </ToastProvider>,
      );

      act(() => screen.getByRole("button", { name: "Auslösen" }).click());

      const toast = await screen.findByRole("status", {
        name: /Gespeichert/,
      });
      const container = toast.closest(".md\\:hidden, .md\\:flex");
      const cssShowsMobileContainer = !cssVariantApplies("md");
      expect(container).toHaveClass(
        cssShowsMobileContainer ? "md:hidden" : "md:flex",
      );
    },
  );

  it.each(CONSUMER_CASES.filter((entry) => entry.breakpoint === "sm"))(
    "TenantPage-Reiter bei $width px und $fontSize px Standardschrift",
    ({ fontSize, width }) => {
      setDefaultFontSize(fontSize);
      setViewportWidth(width);
      const clientWidth = vi
        .spyOn(HTMLElement.prototype, "clientWidth", "get")
        .mockReturnValue(120);
      const offsetWidth = vi
        .spyOn(HTMLElement.prototype, "offsetWidth", "get")
        .mockReturnValue(100);

      try {
        render(
          <TenantPage
            title="Datenverwaltung"
            tabs={{
              value: "personal",
              onChange: vi.fn(),
              items: [
                { value: "kinder", label: "Kinder" },
                { value: "personal", label: "Personal" },
              ],
            }}
          />,
        );

        // Anker: genau ein Reiter ist der Tabulator-Stopp.
        const stops = screen
          .getAllByRole("tab")
          .filter((tab) => tab.getAttribute("tabindex") === "0");
        expect(stops).toHaveLength(1);
        const cssIsPhone = !cssVariantApplies("sm");
        expect(stops[0]).toHaveAccessibleName(cssIsPhone ? "Personal" : "Mehr");
        // Der Tabulator-Stopp ist bei dieser Breite per CSS sichtbar.
        const hiddenByCss = cssIsPhone ? "max-sm:hidden" : "sm:hidden";
        expect(stops[0]).not.toHaveClass(hiddenByCss);
      } finally {
        clientWidth.mockRestore();
        offsetWidth.mockRestore();
      }
    },
  );
});
