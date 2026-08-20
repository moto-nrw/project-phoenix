import { describe, expect, it, vi } from "vitest";

import {
  createFeedState,
  takeMergedPage,
  type FeedPage,
  type FeedSource,
} from "./request-feed";

interface Row {
  readonly id: string;
  readonly occurred_at: string;
}

function row(id: string, occurredAt: string): Row {
  return { id, occurred_at: occurredAt };
}

/** Eine Quelle, die die übergebenen Seiten der Reihe nach ausliefert. */
function pagedSource(key: string, pages: FeedPage<Row>[]): FeedSource<Row> {
  let index = 0;
  return {
    key,
    fetchPage: vi.fn(() => {
      const page = pages[index] ?? { items: [] };
      index += 1;
      return Promise.resolve(page);
    }),
  };
}

const ids = (items: Row[]) => items.map((item) => item.id);

describe("takeMergedPage", () => {
  it("gibt die Zeilen beider Quellen nach Zeitpunkt sortiert aus", async () => {
    const sources = [
      pagedSource("a", [
        {
          items: [
            row("a1", "2026-08-20T10:00:00Z"),
            row("a2", "2026-08-18T10:00:00Z"),
          ],
        },
      ]),
      pagedSource("b", [
        {
          items: [
            row("b1", "2026-08-19T10:00:00Z"),
            row("b2", "2026-08-17T10:00:00Z"),
          ],
        },
      ]),
    ];
    const state = createFeedState(sources);

    const page = await takeMergedPage(sources, state, 10);

    expect(ids(page.items)).toEqual(["a1", "b1", "a2", "b2"]);
    expect(page.hasMore).toBe(false);
  });

  it("setzt beim Nachladen je Quelle dort fort, wo sie aufgehört hat", async () => {
    const sources = [
      pagedSource("a", [
        { items: [row("a1", "2026-08-20T10:00:00Z")], next_cursor: "a-next" },
        { items: [row("a2", "2026-08-16T10:00:00Z")] },
      ]),
      pagedSource("b", [{ items: [row("b1", "2026-08-19T10:00:00Z")] }]),
    ];
    const state = createFeedState(sources);

    const first = await takeMergedPage(sources, state, 2);
    expect(ids(first.items)).toEqual(["a1", "b1"]);
    expect(first.hasMore).toBe(true);

    const second = await takeMergedPage(sources, state, 2);
    expect(ids(second.items)).toEqual(["a2"]);
    expect(second.hasMore).toBe(false);
    // Die erschöpfte Quelle wird nicht erneut abgefragt.
    expect(sources[1]!.fetchPage).toHaveBeenCalledTimes(1);
  });

  it("zieht leere Seiten mit Cursor selbst nach, statt eine leere Liste zu zeigen", async () => {
    const sources = [
      pagedSource("a", [
        { items: [], next_cursor: "a-1" },
        { items: [], next_cursor: "a-2" },
        { items: [row("a1", "2026-08-20T10:00:00Z")] },
      ]),
    ];
    const state = createFeedState(sources);

    const page = await takeMergedPage(sources, state, 5);

    expect(ids(page.items)).toEqual(["a1"]);
    expect(page.hasMore).toBe(false);
  });

  it("kommt mit einer einzigen Quelle aus, wenn die andere fehlt", async () => {
    const sources = [
      pagedSource("a", [{ items: [row("a1", "2026-08-20T10:00:00Z")] }]),
    ];
    const state = createFeedState(sources);

    const page = await takeMergedPage(sources, state, 5);

    expect(ids(page.items)).toEqual(["a1"]);
  });
});
