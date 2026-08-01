import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { NewsCard, isOpenPoll } from "./news-components";
import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";

// Poll (Umfrage, #1371) behaviour of the parent news card: the answer buttons
// live ON the card, one row per child, and a closed poll stops accepting taps.

function poll(overrides: Partial<ParentAnnouncement> = {}): ParentAnnouncement {
  return {
    id: "42",
    title: "Kommt Ihr Kind zur Murmelparty?",
    body: "Am Freitag ab 15 Uhr.",
    priority: "info",
    requires_acknowledgement: false,
    school_name: "OGS Am Berg",
    published_at: "2026-07-01T08:00:00Z",
    read: true,
    acknowledged: false,
    response_type: "single_choice",
    options: [
      { id: "1", label: "Ja" },
      { id: "2", label: "Nein" },
    ],
    children: [
      {
        student_id: "10",
        first_name: "Felix",
        last_name: "Schneider",
        selected_options: [],
      },
    ],
    ...overrides,
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("NewsCard poll answering", () => {
  it("submits the tapped option for the child and marks it answered", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);
    const onUpdated = vi.fn();

    render(<NewsCard item={poll()} onOpen={vi.fn()} onUpdated={onUpdated} />);

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));

    await waitFor(() => {
      expect(respond).toHaveBeenCalledWith(
        "42",
        "10",
        ["1"],
        "2026-07-01T08:00:00Z",
      );
    });
    // Optimistic patch: the card shows the new selection before the round trip.
    expect(onUpdated).toHaveBeenCalledWith("42", {
      children: [
        expect.objectContaining({ student_id: "10", selected_options: ["1"] }),
      ],
    });
  });

  it("withdraws the answer when the selected option is tapped again", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);

    render(
      <NewsCard
        item={poll({
          children: [
            {
              student_id: "10",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: ["1"],
            },
          ],
        })}
        onOpen={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));

    await waitFor(() => {
      expect(respond).toHaveBeenCalledWith(
        "42",
        "10",
        [],
        "2026-07-01T08:00:00Z",
      );
    });
  });

  it("adds to the selection instead of replacing it in multi choice", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);

    render(
      <NewsCard
        item={poll({
          response_type: "multi_choice",
          children: [
            {
              student_id: "10",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: ["1"],
            },
          ],
        })}
        onOpen={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Nein" }));

    await waitFor(() => {
      expect(respond).toHaveBeenCalledWith(
        "42",
        "10",
        ["1", "2"],
        "2026-07-01T08:00:00Z",
      );
    });
  });

  it("disables the options and shows the closed badge past the deadline", () => {
    render(
      <NewsCard
        item={poll({ response_deadline: "2020-01-01T00:00:00Z" })}
        onOpen={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByText("Umfrage geschlossen")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ja" })).toBeDisabled();
  });

  it("reverts the optimistic patch when the write fails", async () => {
    vi.spyOn(parentApi, "respondToAnnouncement").mockRejectedValue(
      new Error("boom"),
    );
    const onUpdated = vi.fn();

    render(<NewsCard item={poll()} onOpen={vi.fn()} onUpdated={onUpdated} />);

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));

    await waitFor(() => {
      // Second call restores the pre-tap children array.
      expect(onUpdated).toHaveBeenLastCalledWith("42", {
        children: [
          expect.objectContaining({ student_id: "10", selected_options: [] }),
        ],
      });
    });
    expect(
      await screen.findByText("Aktion fehlgeschlagen. Bitte erneut versuchen."),
    ).toBeInTheDocument();
  });

  it("names each child when the guardian has more than one", () => {
    render(
      <NewsCard
        item={poll({
          children: [
            {
              student_id: "10",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: [],
            },
            {
              student_id: "11",
              first_name: "Mila",
              last_name: "Schneider",
              selected_options: ["1"],
            },
          ],
        })}
        onOpen={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByText("Felix")).toBeInTheDocument();
    expect(screen.getByText("Mila")).toBeInTheDocument();
    // Two children, two option sets.
    expect(screen.getAllByRole("button", { name: "Ja" })).toHaveLength(2);
  });
});

describe("isOpenPoll", () => {
  it("is true only while an answer is still owed and possible", () => {
    expect(isOpenPoll(poll())).toBe(true);
    expect(
      isOpenPoll(
        poll({
          children: [
            {
              student_id: "10",
              first_name: "Felix",
              last_name: "Schneider",
              selected_options: ["1"],
            },
          ],
        }),
      ),
    ).toBe(false);
    expect(
      isOpenPoll(poll({ response_deadline: "2020-01-01T00:00:00Z" })),
    ).toBe(false);
    expect(isOpenPoll(poll({ response_type: "none" }))).toBe(false);
  });
});
