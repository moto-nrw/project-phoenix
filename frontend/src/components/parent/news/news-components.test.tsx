import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NewsCard, NewsDetailModal, isOpenPoll } from "./news-components";
import type { ParentAnnouncement } from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";
import * as dateHelpers from "~/lib/date-helpers";
import { BELOW_SM } from "~/lib/hooks/use-media-query";

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

function declaration(
  overrides: Partial<ParentAnnouncement> = {},
): ParentAnnouncement {
  return announcement({
    title: "Einverständnis für den Ausflug",
    delivery_mode: "declaration",
    declaration: {
      kind: "consent",
      signers: "all",
      revocable: true,
      requires_password: false,
      deadline: null,
      closed: false,
      version: {
        id: "1",
        version_no: 1,
        content_hash: "test-content-hash",
      },
      children: [
        {
          student_id: "10",
          first_name: "Felix",
          last_name: "Schneider",
          can_submit: true,
          state: "partial",
          my_action: "agreed",
          my_submitted_at: "2026-07-01T08:00:00Z",
          allowed_actions: [],
          other_signers: [
            {
              first_name: "Mila",
              last_name: "Schneider",
              action: null,
              submitted_at: null,
            },
          ],
        },
      ],
    },
    ...overrides,
  });
}

// Opening the detail view asks the backend for the message's attachments
// (#2890). Without a stub that becomes a real fetch, which happy-dom aborts at
// teardown — noise that has nothing to do with what these tests check.
beforeEach(() => {
  vi.spyOn(parentApi, "listAnnouncementAttachments").mockResolvedValue([]);
});

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

  // Android im Hochformat (#3661): unter 640px lief der Dialog als Vaul-Drawer,
  // der jede Berührung für die Zieh-Geste abfing. Antworten muss dort genauso
  // gehen wie im Querformat. Der Handy-Mock bleibt, damit eine wieder
  // eingebaute Breitenverzweigung hier in den Handy-Zweig läuft.
  it("lets a phone in portrait pick and save an answer without a Vaul drawer", () => {
    vi.spyOn(window, "matchMedia").mockImplementation(
      (query) =>
        ({
          matches: query === BELOW_SM,
          media: query,
          onchange: null,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          addListener: vi.fn(),
          removeListener: vi.fn(),
          dispatchEvent: vi.fn(),
        }) as MediaQueryList,
    );

    render(
      <NewsDetailModal item={poll()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    expect(screen.getByRole("dialog")).toHaveClass("rounded-t-2xl");
    expect(document.querySelector("[data-vaul-drawer]")).toBeNull();

    fireEvent.click(screen.getByRole("radio", { name: "Ja" }));
    expect(screen.getByRole("radio", { name: "Ja" })).toBeChecked();
    expect(
      screen.getByRole("button", { name: "Antwort speichern" }),
    ).toBeEnabled();
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

  // The whole card is the button. A bold action word in its last line read as
  // a second button, so the line states where the item stands instead.
  it("flags an unanswered poll as a state, not as an action word", () => {
    render(<NewsCard item={poll()} onOpen={vi.fn()} />);
    expect(screen.getByText("Umfrage")).toBeInTheDocument();
    expect(screen.getByText("Antwort nötig")).toBeInTheDocument();
    expect(screen.queryByText("Antworten")).not.toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Ja" })).not.toBeInTheDocument();
  });

  it("shows no answer status when no child can answer the poll", () => {
    render(<NewsCard item={poll({ children: [] })} onOpen={vi.fn()} />);

    expect(screen.getByText("Umfrage")).toBeInTheDocument();
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
    expect(screen.queryByText("Beantwortet")).not.toBeInTheDocument();
  });

  it("shows no status line and no tint on an unread message that asks for nothing", () => {
    render(<NewsCard item={announcement({ read: false })} onOpen={vi.fn()} />);
    const card = screen.getByRole("button", { name: /Sommerfest/ });
    expect(screen.queryByText("Lesen")).not.toBeInTheDocument();
    expect(screen.queryByText("Gelesen")).not.toBeInTheDocument();
    expect(screen.getByText("Offen")).toHaveClass("sr-only");
    expect(card.className).not.toContain("bg-moto-blue");
    expect(card.className).toContain("bg-white");
  });

  it("flags a missing read confirmation as a state on the card", () => {
    render(
      <NewsCard
        item={announcement({
          read: true,
          requires_acknowledgement: true,
          acknowledged: false,
          delivery_mode: "letter",
        })}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.getAllByText("Bestätigung erforderlich")).toHaveLength(1);
    expect(screen.queryByText("Gelesen bestätigen")).not.toBeInTheDocument();
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
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
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

    expect(screen.getByText("Antwort nötig")).toBeInTheDocument();
    expect(screen.getByText("1 von 2 beantwortet")).toBeInTheDocument();
    expect(screen.getByText("Mila: Nein")).toBeInTheDocument();
  });

  it("does not mark a partial declaration as settled after this guardian agreed", () => {
    render(<NewsCard item={declaration()} onOpen={vi.fn()} />);

    const state = screen.getByText("Teilweise beantwortet");
    expect(state).toHaveClass("text-gray-600");
    expect(state).not.toHaveClass("text-moto-green-strong");
    expect(state.querySelector("svg")).toBeNull();
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

// Up to three mini-headings ("Elternbrief · Erinnerung · Wichtig") stacked
// above the title on a phone. Only a type that differs from the page's
// "Elternbriefe" stays up there; the flags join the state line below.
describe("card head without stacked mini-headings (#3719)", () => {
  it("puts nothing above a letter's title and lists its flags below it", () => {
    render(
      <NewsCard
        item={announcement({
          requires_acknowledgement: true,
          acknowledged: false,
          delivery_mode: "letter",
          priority: "important",
          reminder_sent_at: "2026-09-08T06:00:00Z",
        })}
        onOpen={vi.fn()}
      />,
    );

    const title = screen.getByText("Infos zum Sommerfest");
    expect(title.previousElementSibling).toBeNull();
    expect(screen.queryByText("Elternbrief")).not.toBeInTheDocument();
    for (const label of ["Bestätigung erforderlich", "Wichtig", "Erinnerung"]) {
      expect(
        title.compareDocumentPosition(screen.getByText(label)) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
  });

  it("names a poll or a cancelled care day above the title", () => {
    const { unmount } = render(
      <NewsCard item={poll({ priority: "important" })} onOpen={vi.fn()} />,
    );
    expect(
      screen.getByText("Kommt Ihr Kind zur Murmelparty?")
        .previousElementSibling,
    ).toHaveTextContent(/^Umfrage$/);
    unmount();

    render(
      <NewsCard
        item={announcement({ system_kind: "care_cancellation" })}
        onOpen={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Infos zum Sommerfest").previousElementSibling,
    ).toHaveTextContent(/^Betreuung fällt aus$/);
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

// Terminabstimmung for an Elternsprechtag (#3861): 40 slots, two children. The
// second child's card folds away, so the phone does not scroll past 80 rows.
describe("long poll for several children (#3861)", () => {
  const slots = Array.from({ length: 40 }, (_, i) => ({
    id: String(i + 1),
    label: `Di 14.10. ${String(14 + Math.floor(i / 4)).padStart(2, "0")}:${String((i % 4) * 15).padStart(2, "0")} Uhr`,
  }));
  const twoChildren = (felix: string[], mila: string[]) => [
    {
      student_id: "10",
      first_name: "Felix",
      last_name: "Schneider",
      selected_options: felix,
    },
    {
      student_id: "11",
      first_name: "Mila",
      last_name: "Schneider",
      selected_options: mila,
    },
  ];

  it("opens the first child still waiting and folds the others", async () => {
    const respond = vi
      .spyOn(parentApi, "respondToAnnouncement")
      .mockResolvedValue(undefined);
    render(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          options: slots,
          children: twoChildren(["1"], []),
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    // Mila has no answer yet, so her card is open; Felix's shows his count.
    expect(screen.getAllByRole("checkbox")).toHaveLength(40);
    expect(screen.getByText("1 von 40 gewählt")).toBeInTheDocument();
    expect(screen.getByText("0 von 40 gewählt")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Mila Schneider einklappen" }),
    ).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(
      screen.getByRole("button", { name: "Felix Schneider ausklappen" }),
    );
    expect(screen.getAllByRole("checkbox")).toHaveLength(80);

    const felixSecondSlot = screen.getAllByRole("checkbox", {
      name: slots[1]!.label,
    })[0]!;
    fireEvent.click(felixSecondSlot);
    expect(screen.getByText("2 von 40 gewählt")).toBeInTheDocument();
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

  it("opens the first unanswered child after a corrected poll is refetched", () => {
    const { rerender } = render(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          options: slots,
          children: twoChildren(["1"], ["2"]),
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Felix Schneider einklappen" }),
    );
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);

    rerender(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          published_at: "2026-07-02T08:00:00Z",
          options: slots,
          children: twoChildren(["1"], []),
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Mila Schneider einklappen" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("checkbox")).toHaveLength(40);
  });

  it("opens the first unanswered child after the child list changes", () => {
    const { rerender } = render(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          options: slots,
          children: twoChildren(["1"], ["2"]),
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Felix Schneider einklappen" }),
    );
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);

    rerender(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          options: slots,
          children: [
            ...twoChildren(["1"], ["2"]),
            {
              student_id: "12",
              first_name: "Noah",
              last_name: "Schneider",
              selected_options: [],
            },
          ],
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Noah Schneider einklappen" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("checkbox")).toHaveLength(40);
  });

  it("lets a closed poll still be unfolded to read the answers", () => {
    render(
      <NewsDetailModal
        item={poll({
          response_type: "multi_choice",
          response_deadline: "2020-01-01T00:00:00Z",
          options: slots,
          children: twoChildren(["3"], ["4"]),
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    const toggle = screen.getByRole("button", {
      name: "Mila Schneider ausklappen",
    });
    expect(toggle).toBeEnabled();
    fireEvent.click(toggle);
    const milaSlot = screen.getAllByRole("checkbox", {
      name: slots[3]!.label,
    })[1]!;
    expect(milaSlot).toBeChecked();
    expect(milaSlot).toBeDisabled();
  });

  it("keeps a short poll fully open for several children", () => {
    render(
      <NewsDetailModal
        item={poll({ children: twoChildren([], []) })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /ausklappen|einklappen/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/von 2 gewählt/)).not.toBeInTheDocument();
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
    expect(isOpenPoll(poll({ children: [] }))).toBe(false);
    expect(isOpenPoll(poll({ response_type: "none" }))).toBe(false);
  });
});

describe("scheduled reminder in the parent feed (#3162)", () => {
  it("marks a reminded message and leads with the reminder wording", () => {
    render(
      <NewsCard
        item={announcement({
          reminder_sent_at: "2026-09-08T06:00:00Z",
          reminder_text: "Morgen endet die Betreuung um 13:00 Uhr.",
        })}
        onOpen={vi.fn()}
      />,
    );

    expect(screen.getByText("Erinnerung")).toBeInTheDocument();
    expect(
      screen.getByText("Morgen endet die Betreuung um 13:00 Uhr."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Das Sommerfest beginnt am Freitag um 15 Uhr."),
    ).not.toBeInTheDocument();
  });

  it("keeps the message text on the card when the reminder has no own wording", () => {
    render(
      <NewsCard
        item={announcement({ reminder_sent_at: "2026-09-08T06:00:00Z" })}
        onOpen={vi.fn()}
      />,
    );

    expect(screen.getByText("Erinnerung")).toBeInTheDocument();
    expect(
      screen.getByText("Das Sommerfest beginnt am Freitag um 15 Uhr."),
    ).toBeInTheDocument();
  });

  it("shows the reminder above the full text in the detail view without asking for anything", () => {
    render(
      <NewsDetailModal
        item={announcement({
          reminder_sent_at: "2026-09-08T06:00:00Z",
          reminder_text: "Morgen endet die Betreuung um 13:00 Uhr.",
        })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByText(/Erinnerung vom 08\.09\.2026/)).toBeInTheDocument();
    expect(
      screen.getByText("Morgen endet die Betreuung um 13:00 Uhr."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Das Sommerfest beginnt am Freitag um 15 Uhr."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Gelesen bestätigen" }),
    ).not.toBeInTheDocument();
  });

  it("uses the school's Berlin date for the reminder note", () => {
    vi.spyOn(dateHelpers, "formatDate").mockReturnValue("20.07.2026");
    vi.spyOn(dateHelpers, "formatBerlinDate").mockReturnValue("21.07.2026");

    render(
      <NewsDetailModal
        item={announcement({ reminder_sent_at: "2026-07-20T22:30:00Z" })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getByText(/Erinnerung vom 21\.07\.2026/)).toBeInTheDocument();
  });
});
