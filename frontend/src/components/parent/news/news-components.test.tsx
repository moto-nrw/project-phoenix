import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { NewsCard, NewsDetailModal, isOpenPoll } from "./news-components";
import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";

// Poll (Umfrage, #1371) behaviour in the parent portal: the feed card only
// flags that an answer is due. The detail view is where it is given, one row
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

function announcement(
  overrides: Partial<ParentAnnouncement> = {},
): ParentAnnouncement {
  return {
    ...poll({
      title: "Infos zum Sommerfest",
      body: "Das Sommerfest beginnt am Freitag um 15 Uhr.",
      response_type: "none",
      options: undefined,
      children: undefined,
    }),
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
    fireEvent.click(screen.getByRole("radio", { name: "Ja" }));
    expect(respond).not.toHaveBeenCalled();
    expect(screen.getByRole("radio", { name: "Ja" })).toBeChecked();
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

  it("withdraws a saved single-choice answer through an explicit action", async () => {
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

    fireEvent.click(screen.getByRole("button", { name: "Auswahl aufheben" }));
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

    fireEvent.click(screen.getByRole("checkbox", { name: "Nein" }));
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
    expect(screen.getByRole("radio", { name: "Ja" })).toBeDisabled();
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

    fireEvent.click(screen.getByRole("radio", { name: "Ja" }));
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

    expect(
      await screen.findByText("Aktion fehlgeschlagen. Bitte erneut versuchen."),
    ).toBeInTheDocument();
    // Nothing was committed, the dialog stays open, and the choice stays on
    // screen so it can be retried.
    expect(onUpdated).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("radio", { name: "Ja" })).toBeChecked();
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

    const yesButtons = screen.getAllByRole("radio", { name: "Ja" });
    const noButtons = screen.getAllByRole("radio", { name: "Nein" });
    expect(yesButtons[0]).toBeDefined();
    expect(noButtons[1]).toBeDefined();
    fireEvent.click(yesButtons[0]!);
    fireEvent.click(noButtons[1]!);
    fireEvent.click(screen.getByRole("button", { name: "Antwort speichern" }));

    await waitFor(() => {
      expect(screen.getAllByRole("radio", { name: "Nein" })[1]).toBeChecked();
    });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("resets drafts when a corrected poll has different options", () => {
    const { rerender } = render(
      <NewsDetailModal item={poll()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    fireEvent.click(screen.getByRole("radio", { name: "Ja" }));
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

    expect(screen.queryByRole("radio", { name: "Ja" })).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Antwort speichern" }),
    ).toBeDisabled();
  });

  it("shows one clear action for an unanswered poll", () => {
    render(<NewsCard item={poll()} onOpen={vi.fn()} />);
    expect(screen.getByText("Umfrage")).toBeInTheDocument();
    expect(screen.getByText("Antworten")).toBeInTheDocument();
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Ja" })).not.toBeInTheDocument();
  });

  it("shows the saved answer on an answered poll", () => {
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
      />,
    );

    expect(screen.getByText("Beantwortet")).toBeInTheDocument();
    expect(screen.getByText("Antwort: Ja")).toBeInTheDocument();
    expect(screen.queryByText("Antworten")).not.toBeInTheDocument();
  });

  it("shows partial progress and the saved answer for multiple children", () => {
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
              selected_options: ["2"],
            },
          ],
        })}
        onOpen={vi.fn()}
      />,
    );

    expect(screen.getByText("Antwort vervollständigen")).toBeInTheDocument();
    expect(screen.getByText("1 von 2 beantwortet")).toBeInTheDocument();
    expect(screen.getByText("Mila: Nein")).toBeInTheDocument();
  });

  it("shows read and confirmed states instead of another action", () => {
    const { rerender } = render(
      <NewsCard item={announcement({ read: true })} onOpen={vi.fn()} />,
    );
    expect(screen.getByText("Gelesen")).toBeInTheDocument();

    rerender(
      <NewsCard
        item={announcement({
          read: true,
          requires_acknowledgement: true,
          acknowledged: true,
        })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getByText("Bestätigt")).toBeInTheDocument();
    expect(screen.queryByText("Gelesen bestätigen")).not.toBeInTheDocument();
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

    expect(screen.getByText("Felix Schneider")).toBeInTheDocument();
    expect(screen.getByText("Mila Schneider")).toBeInTheDocument();
    // Two children, two option sets.
    expect(screen.getAllByRole("radio", { name: "Ja" })).toHaveLength(2);
  });
});

describe("announcement detail presentation", () => {
  it("leads with the message from the school and presents it as a mobile sheet", () => {
    render(
      <NewsDetailModal
        item={announcement()}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("region", { name: "Elternbrief von OGS Am Berg" }),
    ).toHaveTextContent("Das Sommerfest beginnt am Freitag um 15 Uhr.");
    expect(screen.queryByText("Mitteilung der OGS")).not.toBeInTheDocument();
    expect(screen.getByRole("dialog")).toHaveClass("rounded-t-2xl");
  });

  it("makes the meaning of a read acknowledgement explicit", async () => {
    const acknowledge = vi
      .spyOn(parentApi, "acknowledgeAnnouncement")
      .mockResolvedValue(undefined);
    const onUpdated = vi.fn();

    render(
      <NewsDetailModal
        item={announcement({ requires_acknowledgement: true })}
        onClose={vi.fn()}
        onUpdated={onUpdated}
      />,
    );

    expect(screen.getByText("Lesebestätigung")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Damit bestätigen Sie nur, dass Sie den Elternbrief gelesen haben.",
      ),
    ).toBeInTheDocument();
    const messageSection = screen.getByRole("region", {
      name: "Elternbrief von OGS Am Berg",
    });
    const acknowledgement = screen.getByText("Lesebestätigung");
    expect(
      messageSection.compareDocumentPosition(acknowledgement) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByText("Bitte bestätigen")).not.toBeInTheDocument();
    expect(
      screen
        .getByRole("dialog")
        .querySelector('[data-moto-duotone-tone="blue"]'),
    ).not.toBeNull();

    const confirmButton = screen.getByRole("button", {
      name: "Gelesen bestätigen",
    });
    expect(confirmButton).toHaveClass("px-4", "py-2", "text-sm");
    expect(confirmButton).not.toHaveClass("min-h-12", "text-[17px]");
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(acknowledge).toHaveBeenCalledWith("42", "2026-07-01T08:00:00Z");
    });
    expect(onUpdated).toHaveBeenCalledWith("42", {
      read: true,
      acknowledged: true,
    });
  });

  it("uses checkboxes for a multi-choice poll", () => {
    render(
      <NewsDetailModal
        item={poll({ response_type: "multi_choice" })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByRole("checkbox", { name: "Ja" })).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Ja" })).not.toBeInTheDocument();
    expect(screen.getByText("Mehrfachauswahl möglich")).toBeInTheDocument();
    expect(screen.queryByText("Umfrage")).not.toBeInTheDocument();
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
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
