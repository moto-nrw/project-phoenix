import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  NewsCard,
  NewsDetailModal,
  isOutstandingAnnouncement,
} from "./news-components";
import { declarationErrorKey, isOpenDeclaration } from "./declaration-section";
import type {
  ParentAnnouncement,
  ParentDeclaration,
  ParentDeclarationChild,
} from "~/lib/parent-api";
import * as parentApi from "~/lib/parent-api";
import { ParentApiError } from "~/lib/parent-api";

// Erklärungen (#3430) in the parent portal: the portal shows exactly the
// server's allowed actions, never preselects one, repeats child, action and
// version in a confirmation, asks for the password there when the school
// wants it, and turns every backend refusal into a sentence.

function child(
  overrides: Partial<ParentDeclarationChild> = {},
): ParentDeclarationChild {
  return {
    student_id: "5",
    first_name: "Mia",
    last_name: "Muster",
    can_submit: true,
    state: "open",
    my_action: null,
    my_submitted_at: null,
    allowed_actions: ["agreed", "declined"],
    other_signers: [],
    ...overrides,
  };
}

function declaration(
  overrides: Partial<ParentDeclaration> = {},
): ParentDeclaration {
  return {
    kind: "consent",
    signers: "any",
    revocable: true,
    requires_password: false,
    deadline: null,
    closed: false,
    version: { id: "12", version_no: 2, content_hash: "ab".repeat(32) },
    children: [child()],
    ...overrides,
  };
}

function item(
  overrides: Partial<ParentAnnouncement> = {},
  decl: Partial<ParentDeclaration> = {},
): ParentAnnouncement {
  return {
    id: "42",
    title: "Ausflug in den Zoo",
    body: "Wir fahren am Freitag in den Zoo.",
    priority: "info",
    requires_acknowledgement: false,
    delivery_mode: "declaration",
    school_name: "OGS Am Berg",
    published_at: "2026-09-01T08:00:00Z",
    read: true,
    acknowledged: false,
    response_type: "none",
    declaration: declaration(decl),
    ...overrides,
  };
}

function submission(
  action: "agreed" | "declined" | "acknowledged" | "revoked",
) {
  return {
    submission: {
      id: "77",
      action,
      submitted_at: "2026-09-09T08:30:00Z",
      version_no: 2,
      content_hash: "ab".repeat(32),
      record_hash: "cd".repeat(32),
      password_confirmed: false,
    },
    created: true,
  };
}

beforeEach(() => {
  vi.spyOn(parentApi, "listAnnouncementAttachments").mockResolvedValue([]);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("isOpenDeclaration", () => {
  it("is open while an own child still needs this guardian's action", () => {
    expect(isOpenDeclaration(item())).toBe(true);
    expect(isOutstandingAnnouncement(item())).toBe(true);
  });

  it("is settled once this guardian acted for every child", () => {
    const done = item(
      {},
      {
        children: [
          child({
            my_action: "agreed",
            my_submitted_at: "2026-09-09T08:30:00Z",
            state: "agreed",
            allowed_actions: ["revoked"],
          }),
        ],
      },
    );
    expect(isOpenDeclaration(done)).toBe(false);
    expect(isOutstandingAnnouncement(done)).toBe(false);
  });

  it("is not open for a child this account may not act for", () => {
    expect(
      isOpenDeclaration(
        item(
          {},
          { children: [child({ can_submit: false, allowed_actions: [] })] },
        ),
      ),
    ).toBe(false);
  });

  it("is not open once another guardian has decided for the child", () => {
    const decidedByOther = item(
      {},
      {
        signers: "any",
        children: [
          child({ state: "agreed", allowed_actions: ["agreed", "declined"] }),
        ],
      },
    );
    expect(isOpenDeclaration(decidedByOther)).toBe(false);
    expect(isOutstandingAnnouncement({ ...decidedByOther, read: true })).toBe(
      false,
    );
  });

  it("stays open while other guardians are partly done", () => {
    expect(
      isOpenDeclaration(
        item({}, { signers: "all", children: [child({ state: "partial" })] }),
      ),
    ).toBe(true);
  });

  it("is not open once the deadline has passed", () => {
    expect(
      isOpenDeclaration(
        item({}, { closed: true, children: [child({ allowed_actions: [] })] }),
      ),
    ).toBe(false);
  });
});

describe("declarationErrorKey", () => {
  function apiError(status: number, code?: string) {
    return new ParentApiError("failed", status, code);
  }

  it.each([
    [404, "not_found", "notFound", false, true],
    [403, "declaration_not_permitted", "notPermitted", false, true],
    [409, "declaration_version_changed", "versionChanged", false, true],
    [409, "declaration_closed", "closed", false, true],
    [409, "declaration_action_not_allowed", "actionNotAllowed", false, true],
    [409, "child_care_ended", "careEnded", false, true],
    [403, "declaration_password_required", "passwordRequired", true, false],
    [403, "declaration_password_incorrect", "passwordIncorrect", true, false],
    [429, undefined, "tooMany", true, false],
    [400, undefined, "generic", true, false],
    [500, undefined, "uncertain", false, true],
    [502, undefined, "uncertain", false, true],
  ] as const)("maps %s %s to %s", (status, code, key, keepDialog, reload) => {
    expect(declarationErrorKey(apiError(status, code))).toEqual({
      key,
      keepDialog,
      reload,
    });
  });

  it("reloads after a network failure, because the backend may have saved it", () => {
    expect(declarationErrorKey(new TypeError("Failed to fetch"))).toEqual({
      key: "uncertain",
      keepDialog: false,
      reload: true,
    });
  });
});

describe("Erklärung in the feed card", () => {
  it("labels the card and flags the open answer", () => {
    render(<NewsCard item={item()} onOpen={vi.fn()} />);
    expect(screen.getByText("Erklärung")).toBeInTheDocument();
    expect(screen.getByText("Antwort nötig")).toBeInTheDocument();
  });

  it("shows where the child stands once this guardian acted", () => {
    render(
      <NewsCard
        item={item(
          {},
          {
            children: [
              child({
                my_action: "agreed",
                my_submitted_at: "2026-09-09T08:30:00Z",
                state: "agreed",
                allowed_actions: ["revoked"],
              }),
            ],
          },
        )}
        onOpen={vi.fn()}
      />,
    );
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
    expect(screen.getByText("Zugestimmt")).toBeInTheDocument();
  });
});

describe("Erklärung in the detail view", () => {
  it("shows exactly the allowed actions, nothing preselected, with what each means", () => {
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    expect(screen.getByRole("button", { name: "Zustimmen" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Ablehnen" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Zur Kenntnis genommen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zustimmung widerrufen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Mit „Zustimmen“ erklären Sie für Mia, dass Sie einverstanden sind.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Nichts ist vorausgewählt.", { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByText("Fassung 2")).toBeInTheDocument();
  });

  it("shows only the acknowledgement button for a Kenntnisnahme", () => {
    render(
      <NewsDetailModal
        item={item(
          {},
          {
            kind: "acknowledgement",
            revocable: false,
            children: [child({ allowed_actions: ["acknowledged"] })],
          },
        )}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Zur Kenntnis genommen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zustimmen" }),
    ).not.toBeInTheDocument();
  });

  it("confirms child, action and version before it submits", async () => {
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockResolvedValue(submission("agreed"));
    const onUpdated = vi.fn();
    const onStale = vi.fn();

    render(
      <NewsDetailModal
        item={item()}
        onClose={vi.fn()}
        onUpdated={onUpdated}
        onStale={onStale}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    expect(submit).not.toHaveBeenCalled();

    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    expect(within(dialog).getByText("Mia Muster")).toBeInTheDocument();
    expect(within(dialog).getAllByText("Zugestimmt").length).toBeGreaterThan(0);
    expect(within(dialog).getByText("Fassung 2")).toBeInTheDocument();
    expect(within(dialog).queryByLabelText(/Passwort/)).not.toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith("42", {
        studentId: "5",
        action: "agreed",
        versionId: "12",
        password: undefined,
      }),
    );
    expect(onUpdated).toHaveBeenCalledWith(
      "42",
      expect.objectContaining({
        declaration: expect.objectContaining({
          children: [
            expect.objectContaining({
              my_action: "agreed",
              my_submitted_at: "2026-09-09T08:30:00Z",
              allowed_actions: ["revoked"],
            }),
          ],
        }),
      }),
    );
    expect(
      await screen.findByText("Ihre Erklärung für Mia ist gespeichert."),
    ).toBeInTheDocument();
    expect(onStale).toHaveBeenCalledWith("42");
  });

  it("asks for the password in the confirmation and keeps the dialog on a wrong one", async () => {
    const assign = vi.fn();
    vi.spyOn(window, "location", "get").mockReturnValue({
      ...window.location,
      assign,
    });
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockRejectedValueOnce(
        new ParentApiError("wrong", 403, "declaration_password_incorrect"),
      )
      .mockResolvedValueOnce(submission("declined"));

    render(
      <NewsDetailModal
        item={item({}, { requires_password: true })}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });

    // Without a password nothing is sent.
    fireEvent.click(within(dialog).getByRole("button", { name: "Ablehnen" }));
    expect(
      await within(dialog).findByText("Bitte geben Sie Ihr Passwort ein."),
    ).toBeInTheDocument();
    expect(submit).not.toHaveBeenCalled();

    const field = within(dialog).getByLabelText("Passwort Ihres Eltern-Kontos");
    expect(field).toHaveAttribute("type", "password");
    fireEvent.change(field, { target: { value: "falsch" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Ablehnen" }));

    expect(
      await within(dialog).findByText(
        "Das Passwort stimmt nicht. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(submit).toHaveBeenCalledWith("42", {
      studentId: "5",
      action: "declined",
      versionId: "12",
      password: "falsch",
    });
    // A wrong password is no logout.
    expect(assign).not.toHaveBeenCalled();

    fireEvent.change(
      within(dialog).getByLabelText("Passwort Ihres Eltern-Kontos"),
      { target: { value: "richtig" } },
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Ablehnen" }));
    await waitFor(() => expect(submit).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Ihre Antwort prüfen" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("closes the confirmation, tells the parent and reloads when the text changed", async () => {
    vi.spyOn(parentApi, "submitDeclaration").mockRejectedValue(
      new ParentApiError("changed", 409, "declaration_version_changed"),
    );
    const onStale = vi.fn();

    render(
      <NewsDetailModal
        item={item()}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
        onStale={onStale}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    expect(
      await screen.findByText(
        "Die Schule hat den Text geändert. Bitte lesen Sie ihn noch einmal und antworten Sie dann neu.",
      ),
    ).toBeInTheDocument();
    expect(onStale).toHaveBeenCalledWith("42");
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Ihre Antwort prüfen" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("shows the own action with a proof link and offers a withdrawal with its own dialog", async () => {
    const submit = vi
      .spyOn(parentApi, "submitDeclaration")
      .mockResolvedValue(submission("revoked"));

    render(
      <NewsDetailModal
        item={item(
          {},
          {
            children: [
              child({
                my_action: "agreed",
                my_submitted_at: "2026-09-09T08:30:00Z",
                state: "agreed",
                allowed_actions: ["revoked"],
              }),
            ],
          },
        )}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Ihre Antwort: Zugestimmt am 09.09.2026, 10:30 Uhr"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Nachweis ansehen" }),
    ).toHaveAttribute("href", "/news/42/nachweis?student=5");
    expect(
      screen.queryByRole("button", { name: "Zustimmen" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Zustimmung widerrufen" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Zustimmung widerrufen",
    });
    expect(
      within(dialog).getByText("Sie nehmen Ihre Zustimmung für Mia zurück."),
    ).toBeInTheDocument();
    expect(submit).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Widerrufen" }));
    await waitFor(() =>
      expect(submit).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ action: "revoked", versionId: "12" }),
      ),
    );
  });

  it("lists the other guardians when everyone must submit", () => {
    render(
      <NewsDetailModal
        item={item(
          {},
          {
            signers: "all",
            children: [
              child({
                state: "partial",
                other_signers: [
                  {
                    first_name: "Anna",
                    last_name: "Muster",
                    action: "agreed",
                    submitted_at: "2026-09-20T08:30:00Z",
                  },
                  {
                    first_name: "Tom",
                    last_name: "Muster",
                    action: null,
                    submitted_at: null,
                  },
                ],
              }),
            ],
          },
        )}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Jede sorgeberechtigte Person antwortet selbst.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Teilweise beantwortet")).toBeInTheDocument();
    expect(screen.getByText(/Anna Muster:\s*Zugestimmt/)).toBeInTheDocument();
    expect(
      screen.getByText(/Tom Muster:\s*noch keine Antwort/),
    ).toBeInTheDocument();
  });

  it("explains a child this account cannot act for and offers no buttons", () => {
    render(
      <NewsDetailModal
        item={item(
          {},
          { children: [child({ can_submit: false, allowed_actions: [] })] },
        )}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Für Mia können Sie hier nicht antworten. Das machen die sorgeberechtigten Personen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zustimmen" }),
    ).not.toBeInTheDocument();
  });

  it("says the deadline passed and offers nothing after it", () => {
    render(
      <NewsDetailModal
        item={item(
          {},
          {
            closed: true,
            deadline: "2026-09-08T21:59:59Z",
            children: [child({ state: "expired", allowed_actions: [] })],
          },
        )}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(screen.getAllByText("Frist abgelaufen").length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        "Die Frist ist abgelaufen. Sie können nicht mehr antworten.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zustimmen" }),
    ).not.toBeInTheDocument();
  });
});

describe("Erklärung decided by another guardian (#3430)", () => {
  it("shows no pill but keeps the decline button", () => {
    const decided = item(
      {},
      {
        signers: "any",
        children: [
          child({ state: "agreed", allowed_actions: ["agreed", "declined"] }),
        ],
      },
    );
    const { unmount } = render(<NewsCard item={decided} onOpen={vi.fn()} />);
    expect(screen.queryByText("Antwort nötig")).not.toBeInTheDocument();
    unmount();

    render(
      <NewsDetailModal item={decided} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: "Ablehnen" })).toBeEnabled();
  });
});

describe("Erklärung already settled by another guardian (#3430)", () => {
  function settled(submittedAt?: string) {
    return item(
      {},
      {
        signers: "any",
        children: [
          child({
            first_name: "Felix",
            state: "agreed",
            allowed_actions: ["agreed", "declined"],
            other_signers: [
              {
                first_name: "Sabine",
                last_name: "Muster",
                action: "agreed",
                submitted_at: submittedAt ?? null,
              },
            ],
          }),
        ],
      },
    );
  }

  it("names who settled it and keeps the buttons only as a quiet option", () => {
    render(
      <NewsDetailModal
        item={settled()}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Sabine Muster hat zugestimmt. Eine Antwort genügt."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sie können trotzdem selbst antworten."),
    ).toBeInTheDocument();
    for (const name of ["Zustimmen", "Ablehnen"]) {
      const button = screen.getByRole("button", { name });
      expect(button).toBeEnabled();
      // Outline, not the dark primary call to action.
      expect(button.className).not.toContain("bg-gray-900");
    }
  });

  it("adds the date when the backend sends one", () => {
    render(
      <NewsDetailModal
        item={settled("2026-09-02T06:15:00Z")}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
      />,
    );

    expect(
      screen.getByText(
        "Sabine Muster hat am 02.09.2026, 08:15 Uhr zugestimmt. Eine Antwort genügt.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps the primary buttons and no hint while the child is open", () => {
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    expect(screen.queryByText(/Eine Antwort genügt/)).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zustimmen" }).className,
    ).toContain("bg-gray-900");
  });

  it("titles the text card as an Erklärung, not as an Elternbrief", () => {
    render(
      <NewsDetailModal item={item()} onClose={vi.fn()} onUpdated={vi.fn()} />,
    );

    expect(screen.getByText("Erklärung von OGS Am Berg")).toBeInTheDocument();
    expect(
      screen.queryByText("Elternbrief von OGS Am Berg"),
    ).not.toBeInTheDocument();
  });
});

describe("unclear submit result (#3430)", () => {
  it("closes the dialog, says so and reloads the feed when no answer came back", async () => {
    vi.spyOn(parentApi, "submitDeclaration").mockRejectedValue(
      new TypeError("Failed to fetch"),
    );
    const onStale = vi.fn();

    render(
      <NewsDetailModal
        item={item()}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
        onStale={onStale}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    expect(
      await screen.findByText(/Der Stand wird neu geladen/),
    ).toBeInTheDocument();
    expect(onStale).toHaveBeenCalledWith("42");
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Ihre Antwort prüfen" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("keeps the dialog and does not reload for a plain input error", async () => {
    vi.spyOn(parentApi, "submitDeclaration").mockRejectedValue(
      new ParentApiError("bad", 400),
    );
    const onStale = vi.fn();

    render(
      <NewsDetailModal
        item={item()}
        onClose={vi.fn()}
        onUpdated={vi.fn()}
        onStale={onStale}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zustimmen" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Ihre Antwort prüfen",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Zustimmen" }));

    expect(
      await within(dialog).findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(onStale).not.toHaveBeenCalled();
  });
});
