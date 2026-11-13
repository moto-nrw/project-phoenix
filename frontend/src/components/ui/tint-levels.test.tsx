import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Alert, type AlertType } from "./alert";
import { IconTile } from "./icon-tile";
import { InsetPanel } from "./inset-panel";

// One meaning, one tint (#6564): a tinted icon tile, a notice and an inner
// surface of the same meaning share the fill step of the Alert.
function fillOf(element: HTMLElement): string[] {
  return element.className.split(/\s+/).filter((c) => c.startsWith("bg-"));
}

function alertFill(type: AlertType): string[] {
  render(<Alert type={type} message={`Hinweis ${type}`} />);
  const notice = screen.getByText(`Hinweis ${type}`).closest("[role]");
  if (!(notice instanceof HTMLElement)) throw new Error("no alert surface");
  return fillOf(notice);
}

describe("tint step per meaning", () => {
  it.each([
    ["success", "green"],
    ["warning", "orange"],
    ["error", "red"],
    ["info", "blue"],
  ] as const)("Alert %s and IconTile %s share one fill", (type, tone) => {
    const expected = alertFill(type);
    render(
      <IconTile
        icon={<svg aria-hidden="true" />}
        tone={tone}
        data-testid="tile"
      />,
    );

    expect(fillOf(screen.getByTestId("tile"))).toEqual(expected);
  });

  it.each([
    ["warning", "warning"],
    ["error", "danger"],
  ] as const)("Alert %s and InsetPanel %s share one fill", (type, tone) => {
    const expected = alertFill(type);
    render(<InsetPanel tone={tone} data-testid="panel" />);

    expect(fillOf(screen.getByTestId("panel"))).toEqual(expected);
  });
});
