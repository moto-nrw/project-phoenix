import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TeamChatInboxParts } from "~/components/messaging/team-chat-inbox";
import type { TeamChatThreadParts } from "~/components/messaging/team-chat-thread";
import { renderInboxFrame, renderThreadFrame } from "./team-chat-frames";

function inboxParts(
  overrides: Partial<TeamChatInboxParts> = {},
): TeamChatInboxParts {
  return {
    title: "Team-Chat",
    stats: "2 Unterhaltungen · 0 ungelesen",
    statsLoading: false,
    chatEnabled: true,
    count: 2,
    search: {
      value: "",
      onChange: () => undefined,
      placeholder: "Person oder Gruppenchat suchen…",
    },
    onlyUnread: false,
    setOnlyUnread: () => undefined,
    composeButton: null,
    loading: false,
    empty: null,
    error: null,
    staleWarning: null,
    list: null,
    overlays: null,
    ...overrides,
  };
}

function threadParts(
  overrides: Partial<TeamChatThreadParts> = {},
): TeamChatThreadParts {
  return {
    title: "Gruppenchat",
    roleLabel: null,
    stats: "2 Nachrichten · 3 Mitglieder",
    audience:
      "Nur die Mitglieder sehen diese Nachrichten. Wer neu dazukommt, sieht sie ab dem Beitritt.",
    actions: null,
    statsLoading: false,
    state: "ready",
    empty: { icon: null, title: "", description: "" },
    error: null,
    loading: false,
    back: { href: "/team-chat", label: "Zurück zum Team-Chat" },
    containerRef: { current: null },
    body: null,
    ...overrides,
  };
}

describe("Team-Chat-Hüllen", () => {
  it("zeigt die Zahl der gefilterten Unterhaltungen in der Suchzeile", () => {
    render(renderInboxFrame(inboxParts()));

    expect(screen.getAllByText("2")).toHaveLength(2);
  });

  it("zeigt keinen Zähler, solange die Unterhaltungsliste lädt", () => {
    render(renderInboxFrame(inboxParts({ loading: true, count: null })));

    // Anker: Das Gerüst zeigt wirklich den Ladezustand der Unterhaltungsliste.
    expect(
      screen.getByRole("status", {
        name: "Der Bereich „Team-Chat“ wird geladen…",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("2")).not.toBeInTheDocument();
  });

  it("legt den Hinweis zur Sichtbarkeit der Unterhaltung hinter den Info-Knopf", () => {
    const audience =
      "Nur die Mitglieder sehen diese Nachrichten. Wer neu dazukommt, sieht sie ab dem Beitritt.";
    render(renderThreadFrame(threadParts({ audience })));

    fireEvent.click(
      screen.getByRole("button", { name: "Erklärung zu Gruppenchat" }),
    );

    expect(screen.getByText(audience)).toBeInTheDocument();
  });
});
