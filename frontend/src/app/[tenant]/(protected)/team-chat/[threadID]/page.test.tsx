import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TeamChatThreadParts } from "~/components/messaging/team-chat-thread";
import { renderThreadFrame } from "./page";

function parts(
  overrides: Partial<TeamChatThreadParts> = {},
): TeamChatThreadParts {
  return {
    title: "Unterhaltung",
    roleLabel: null,
    stats: "",
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

describe("renderThreadFrame", () => {
  // Der Rückweg steht in jedem Zustand in der Kopfkarte, über dem Titel und
  // auf jeder Breite (#4364) -- nicht als eigener Knopf über dem Chat.
  it.each([
    ["ready", parts()],
    [
      "disabled",
      parts({
        state: "disabled",
        empty: {
          icon: null,
          title: "Der Team-Chat ist ausgeschaltet",
          description: "Der Team-Chat ist nicht verfügbar.",
        },
      }),
    ],
    [
      "error",
      parts({ state: "error", error: "Diese Unterhaltung gibt es nicht." }),
    ],
  ])("puts the back link into the page head when %s", (_state, frameParts) => {
    render(renderThreadFrame(frameParts));

    const link = screen.getByRole("link", { name: "Zurück zum Team-Chat" });
    expect(link).toHaveAttribute("href", "/test-tenant/team-chat");
    expect(link.closest("header")).not.toBeNull();
    expect(link.className).not.toMatch(/(^|\s)(sm|md|lg|xl):hidden/);
    expect(screen.getAllByRole("link", { name: /Team-Chat/ })).toHaveLength(1);
  });

  it("opts the chat card out of the page grow rule so only the list scrolls", () => {
    render(renderThreadFrame(parts({ body: <p>Verlauf</p> })));

    // Ohne die Klasse wächst die Karte auf die Höhe des ganzen Verlaufs und
    // die feste Hülle schneidet das Eingabefeld ab (#3328).
    expect(screen.getByText("Verlauf").closest("section")).toHaveClass(
      "moto-scroll-surface",
    );
  });
});
