import { act, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import {
  DISCARDED_WRITE_MESSAGE,
  SessionEndNotice,
} from "./session-end-notice";

function sessionEnd(ending: boolean, discardedWrites: number) {
  act(() => {
    window.dispatchEvent(
      new CustomEvent("phoenix:session-end", {
        detail: { ending, discardedWrites },
      }),
    );
  });
}

// A logout that leaves the tab open discards the writes it held (#5126).
describe("SessionEndNotice", () => {
  it("says once that a held change was not saved", () => {
    render(
      <ToastProvider>
        <SessionEndNotice />
      </ToastProvider>,
    );
    sessionEnd(true, 0);
    sessionEnd(false, 2);

    expect(screen.getAllByRole("alert")).toHaveLength(1);
    expect(screen.getByRole("alert")).toHaveTextContent(
      DISCARDED_WRITE_MESSAGE,
    );
  });
});
