import { render, screen } from "@testing-library/react";
import Link from "next/link";
import { describe, expect, it } from "vitest";
import { renderThreadFrame } from "./page";

describe("renderThreadFrame", () => {
  it("keeps the back navigation when the team chat is disabled", () => {
    render(
      renderThreadFrame({
        title: "Unterhaltung",
        roleLabel: null,
        stats: "",
        statsLoading: false,
        state: "disabled",
        empty: {
          icon: null,
          title: "Der Team-Chat ist ausgeschaltet",
          description: "Der Team-Chat ist nicht verfügbar.",
        },
        errorMessage: "",
        loading: false,
        backNav: <Link href="/team-chat">Zurück zum Team-Chat</Link>,
        containerRef: { current: null },
        body: null,
      }),
    );

    expect(
      screen.getByRole("link", { name: "Zurück zum Team-Chat" }),
    ).toHaveAttribute("href", "/team-chat");
  });
});
