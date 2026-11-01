import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  DataPrefetchContext,
  useDataPrefetchIntent,
} from "./data-prefetch-intent";

function Card() {
  const intent = useDataPrefetchIntent("/students/42");
  return <button {...intent}>Kind öffnen</button>;
}

function renderCard() {
  const preload = vi.fn();
  render(
    <DataPrefetchContext.Provider value={preload}>
      <Card />
    </DataPrefetchContext.Provider>,
  );
  return { preload, card: screen.getByRole("button", { name: "Kind öffnen" }) };
}

describe("data prefetch intent", () => {
  it("preloads only once when a tap focuses the card after touchend", () => {
    vi.useFakeTimers();
    const { card, preload } = renderCard();
    const point = { identifier: 1, clientX: 10, clientY: 20 };
    fireEvent.touchStart(card, { touches: [point], changedTouches: [point] });
    fireEvent.touchEnd(card, { touches: [], changedTouches: [point] });
    expect(preload).toHaveBeenCalledExactlyOnceWith("/students/42", undefined);
    fireEvent.mouseDown(card);
    fireEvent.focus(card);
    act(() => vi.advanceTimersByTime(300));
    expect(preload).toHaveBeenCalledTimes(1);
  });

  it("preloads after 300 ms of keyboard focus without touch", () => {
    vi.useFakeTimers();
    const { card, preload } = renderCard();
    expect(card).toBeInTheDocument();
    fireEvent.focus(card);
    act(() => vi.advanceTimersByTime(299));
    expect(preload).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(1));
    expect(preload).toHaveBeenCalledExactlyOnceWith("/students/42", undefined);
  });
});
