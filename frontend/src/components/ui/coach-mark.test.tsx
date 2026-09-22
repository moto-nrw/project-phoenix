import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { CoachMark } from "./coach-mark";
import { findVisibleTarget } from "~/components/school-setup/use-setup-tour";

function targetAt(rect: {
  top: number;
  left: number;
  width: number;
  height: number;
}): HTMLElement {
  const element = document.createElement("div");
  element.getClientRects = () => [{}] as unknown as DOMRectList;
  element.getBoundingClientRect = () =>
    ({
      ...rect,
      bottom: rect.top + rect.height,
      right: rect.left + rect.width,
    }) as DOMRect;
  document.body.appendChild(element);
  created.push(element);
  return element;
}

/** Nur die selbst angelegten Ziele entfernen; die Portale baut RTL ab. */
const created: Element[] = [];

describe("CoachMark", () => {
  afterEach(() => {
    cleanup();
    for (const element of created.splice(0)) element.remove();
  });

  it("stands beside a spot that fills the screen height, pointing at it", () => {
    // Ein ganzes Formular rechts (Slide-over): weder darüber noch darunter
    // ist Platz, links schon.
    const form = targetAt({
      top: 0,
      left: 600,
      width: 400,
      height: window.innerHeight,
    });
    render(
      <CoachMark
        target={form}
        title="Angaben eintragen"
        text="Tragen Sie die Angaben ein."
        onNext={() => undefined}
        onClose={() => undefined}
      />,
    );

    const bubble = screen.getByRole("dialog", { name: "Angaben eintragen" });
    const left = Number.parseFloat(bubble.style.left);
    expect(left + Number.parseFloat(bubble.style.width)).toBeLessThanOrEqual(
      594,
    );
    expect(bubble.querySelector(".-right-1\\.5")).not.toBeNull();
  });

  it("ends the highlight above a button that is the next station", () => {
    const card = targetAt({ top: 100, left: 100, width: 400, height: 300 });
    const button = targetAt({ top: 350, left: 100, width: 400, height: 40 });
    render(
      <CoachMark
        target={card}
        endBefore={button}
        title="Angaben eintragen"
        text="Text"
        onNext={() => undefined}
        onClose={() => undefined}
      />,
    );

    const ring = document.querySelector<HTMLElement>(".ring-moto-green");
    expect(ring).not.toBeNull();
    const bottom =
      Number.parseFloat(ring!.style.top) +
      Number.parseFloat(ring!.style.height);
    // Bis knapp über den Knopf (350), nicht bis zum Kartenende (400).
    expect(bottom).toBeLessThan(350);
    expect(bottom).toBeGreaterThan(330);
  });

  it("keeps the page dimmed while it looks for the next spot", () => {
    const { rerender } = render(
      <CoachMark
        target={null}
        searching
        title="Station"
        text="Text"
        onNext={() => undefined}
        onClose={() => undefined}
      />,
    );
    // Abdunklung da, Sprechblase noch nicht: kein helles Aufblitzen.
    expect(document.querySelector(".bg-gray-900\\/45")).not.toBeNull();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Die Stelle ist gefunden: dieselbe Tour zeigt jetzt Sprechblase und
    // Klick-Schutz, obwohl sie mit der Suche begann.
    const outside = vi.fn();
    document.addEventListener("pointerdown", outside);
    rerender(
      <CoachMark
        target={targetAt({ top: 10, left: 10, width: 120, height: 40 })}
        title="Station"
        text="Text"
        onNext={() => undefined}
        onClose={() => undefined}
      />,
    );
    fireEvent.pointerDown(screen.getByRole("button", { name: "Weiter" }));
    expect(outside).not.toHaveBeenCalled();
    document.removeEventListener("pointerdown", outside);
  });

  it("keeps a press on the bubble away from the page underneath", () => {
    const outside = vi.fn();
    document.addEventListener("pointerdown", outside);
    render(
      <CoachMark
        target={targetAt({ top: 10, left: 10, width: 120, height: 40 })}
        title="Station"
        text="Text"
        onNext={() => undefined}
        onClose={() => undefined}
      />,
    );

    fireEvent.pointerDown(screen.getByRole("button", { name: "Weiter" }));

    expect(outside).not.toHaveBeenCalled();
    document.removeEventListener("pointerdown", outside);
  });

  it("finds the form body next to the create button", () => {
    const form = document.createElement("form");
    const body = document.createElement("div");
    const footer = document.createElement("div");
    const submit = document.createElement("button");
    submit.dataset.setupTour = "student-submit";
    footer.appendChild(submit);
    form.append(body, footer);
    document.body.appendChild(form);
    created.push(form);
    body.getClientRects = () => [{}] as unknown as DOMRectList;
    body.getBoundingClientRect = () =>
      ({
        top: 0,
        left: 0,
        width: 300,
        height: 400,
        bottom: 400,
        right: 300,
      }) as DOMRect;

    expect(
      findVisibleTarget(
        'form:has([data-setup-tour="student-submit"]) > div:first-child',
      ),
    ).toBe(body);
  });
});
