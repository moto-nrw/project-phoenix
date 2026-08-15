import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { NewsCard, NewsDetailModal, isOpenPoll } from "./news-components";
import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";

// Poll (Umfrage, #1371) behaviour in the parent portal: the feed card only
// flags that an answer is due, the detail view is where it is given — one row
// per child, saved explicitly, and refused once the poll is closed.

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

describe("Umfrage answering in the detail view", () => {
  it("selects locally and only writes once the answer is saved", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);
    const onUpdated = vi.fn();
    const onClose = vi.fn();

    render(
      <NewsDetailModal item={poll()} onClose={onClose} onUpdated={onUpdated} />,
    );

    // Picking an option must not write anything yet.
    fireEvent.click(screen.getByRole("button", { name: "Ja" }));
    expect(respond).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Ja" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Noch nicht gespeichert")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

    await waitFor(() => {
      expect(respond).toHaveBeenCalledWith(
        "42",
        "10",
        ["1"],
        "2026-07-01T08:00:00Z",
      );
    });
    expect(onUpdated).toHaveBeenCalledWith("42", {
      children: [
        expect.objectContaining({ student_id: "10", selected_options: ["1"] }),
      ],
    });
    // A successful write closes the dialog, like every other parent modal.
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("keeps the save button disabled while nothing changed", () => {
    render(
      <NewsDetailModal item={poll()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );
    expect(
      screen.getByRole("button", { name: "Antwort speichern" }),
    ).toBeDisabled();
  });

  it("withdraws the answer when the selected option is tapped again", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);

    render(
      <NewsDetailModal
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
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

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
      <NewsDetailModal
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
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Nein" }));
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

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
      <NewsDetailModal
        item={poll({ response_deadline: "2020-01-01T00:00:00Z" })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByText("Umfrage geschlossen")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ja" })).toBeDisabled();
    // Nothing left to save, so the action is gone entirely.
    expect(
      screen.queryByRole("button", { name: "Antwort speichern" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the selection and reports the error when saving fails", async () => {
    vi.spyOn(parentApi, "respondToAnnouncement").mockRejectedValue(
      new Error("boom"),
    );
    const onUpdated = vi.fn();
    const onClose = vi.fn();

    render(
      <NewsDetailModal item={poll()} onClose={onClose} onUpdated={onUpdated} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

    expect(
      await screen.findByText("Aktion fehlgeschlagen. Bitte erneut versuchen."),
    ).toBeInTheDocument();
    // Nothing was committed, the dialog stays open, and the choice stays on
    // screen so it can be retried.
    expect(onUpdated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Ja" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("reconciles earlier child responses when a later save fails", async () => {
    vi.spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("boom"));
    const onClose = vi.fn();
    const initial = poll({
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
          selected_options: [],
        },
      ],
    });
    function StatefulModal() {
      const [item, setItem] = useState(initial);
      return (
        <NewsDetailModal
          item={item}
          onClose={onClose}
          onUpdated={(_id, patch) =>
            setItem((current) => ({ ...current, ...patch }))
          }
        />
      );
    }

    render(<StatefulModal />);

    const yesButtons = screen.getAllByRole("button", { name: "Ja" });
    const noButtons = screen.getAllByRole("button", { name: "Nein" });
    expect(yesButtons[0]).toBeDefined();
    expect(noButtons[1]).toBeDefined();
    fireEvent.click(yesButtons[0]!);
    fireEvent.click(noButtons[1]!);
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

    await waitFor(() => {
      expect(
        screen.getAllByRole("button", { name: "Nein" })[1],
      ).toHaveAttribute("aria-pressed", "true");
    });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("resets drafts when a corrected poll has different options", () => {
    const { rerender } = render(
      <NewsDetailModal item={poll()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ja" }));
    expect(
      screen.getByRole("button", { name: "Antwort speichern" }),
    ).toBeEnabled();

    rerender(
      <NewsDetailModal
        item={poll({
          published_at: "2026-07-02T08:00:00Z",
          options: [
            { id: "3", label: "Vielleicht" },
            { id: "4", label: "Nein" },
          ],
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Ja" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Antwort speichern" }),
    ).toBeDisabled();
  });

  it("flags an open poll on the feed card without answering there", () => {
    render(<NewsCard item={poll()} onOpen={vi.fn()} />);
    expect(screen.getByText("Antwort nötig")).toBeInTheDocument();
    expect(screen.getByText("Antworten")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Ja" }),
    ).not.toBeInTheDocument();
  });

  it("names each child when the guardian has more than one", () => {
    render(
      <NewsDetailModal
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
        onClose={vi.fn()}
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
