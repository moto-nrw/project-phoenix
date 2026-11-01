import { HouseLineIcon, AcornIcon } from "@phosphor-icons/react/ssr";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-icon";
import { NAV_SCENE_PLAY_MS, NavSceneIcon } from "./nav-scene-icon";

function renderInRow(ui: React.ReactNode) {
  render(
    <button type="button" data-testid="row">
      {ui}
      Home
    </button>,
  );
  const row = screen.getByTestId("row");
  const icon = row.querySelector("svg.ns-icon, span.ns-fallback");
  if (!icon) throw new Error("Icon fehlt");
  return { row, icon };
}

function hover(row: HTMLElement, pointerType = "mouse") {
  fireEvent.pointerEnter(row, { pointerType });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("NavSceneIcon", () => {
  it("zeichnet die Szene des Glyphs statt des Phosphor-Icons", () => {
    const { icon } = renderInRow(<NavSceneIcon icon={HouseLineIcon} />);
    expect(icon).toHaveAttribute("data-nav-scene", "HouseLineIcon");
    expect(icon.querySelector(".ns-roof")).not.toBeNull();
  });

  it("spielt die Szene beim Überfahren mit der Maus und lässt sie zu Ende laufen", () => {
    vi.useFakeTimers();
    const { row, icon } = renderInRow(<NavSceneIcon icon={HouseLineIcon} />);

    hover(row);
    expect(icon).toHaveClass("ns-play");

    fireEvent.pointerLeave(row, { pointerType: "mouse" });
    vi.advanceTimersByTime(NAV_SCENE_PLAY_MS - 1);
    expect(icon).toHaveClass("ns-play");

    vi.advanceTimersByTime(1);
    expect(icon).not.toHaveClass("ns-play");
  });

  it("startet eine laufende Szene nicht neu", () => {
    vi.useFakeTimers();
    const { row, icon } = renderInRow(<NavSceneIcon icon={HouseLineIcon} />);

    hover(row);
    vi.advanceTimersByTime(NAV_SCENE_PLAY_MS / 2);
    hover(row);
    vi.advanceTimersByTime(NAV_SCENE_PLAY_MS / 2);
    expect(icon).not.toHaveClass("ns-play");
  });

  it("spielt nichts bei einem Tipp auf dem Touchscreen", () => {
    const { row, icon } = renderInRow(<NavSceneIcon icon={HouseLineIcon} />);
    hover(row, "touch");
    expect(icon).toHaveClass("ns-icon");
    expect(icon).not.toHaveClass("ns-play");
  });

  it("spielt nichts, wenn die Person reduzierte Bewegung eingestellt hat", () => {
    vi.spyOn(window, "matchMedia").mockImplementation(
      (query: string) =>
        ({
          matches: query.includes("prefers-reduced-motion"),
          media: query,
        }) as MediaQueryList,
    );
    const { row, icon } = renderInRow(<NavSceneIcon icon={HouseLineIcon} />);
    hover(row);
    expect(icon).toHaveClass("ns-icon");
    expect(icon).not.toHaveClass("ns-play");
  });

  it("spielt nichts in einer gesperrten Zeile ohne Link oder Knopf", () => {
    render(
      <div data-testid="locked">
        <NavSceneIcon icon={HouseLineIcon} />
        Bald
      </div>,
    );
    const row = screen.getByTestId("locked");
    hover(row);
    expect(row.querySelector("svg")).toHaveClass("ns-icon");
    expect(row.querySelector("svg")).not.toHaveClass("ns-play");
  });

  it("trägt aktiv die Farbe des Konzepts", () => {
    const { icon } = renderInRow(
      <NavSceneIcon icon={HouseLineIcon} tone="blue" active />,
    );
    expect(icon).toHaveStyle({ color: MOTO_DUOTONE_TONES.blue.primary });
  });

  it("lässt ein Glyph ohne Szene als Ganzes hüpfen", () => {
    vi.useFakeTimers();
    const { row, icon } = renderInRow(<NavSceneIcon icon={AcornIcon} />);
    expect(icon).toHaveClass("ns-fallback");
    hover(row);
    expect(icon).toHaveClass("ns-play");
  });
});
