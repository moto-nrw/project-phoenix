import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import de from "~/i18n/messages/de.generated";

const { captureException } = vi.hoisted(() => ({
  captureException: vi.fn(),
}));
vi.mock("@sentry/nextjs", () => ({ captureException }));

import ParentError from "./error";

describe("parents error boundary", () => {
  it("shows the catalog crash text with retry and reports the crash", () => {
    const retry = vi.fn();
    const crash = new Error("render failed");

    render(<ParentError error={crash} retry={retry} />);

    expect(
      screen.getByText(
        "Die Seite konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Support/)).not.toBeInTheDocument();
    expect(screen.queryByText(/render failed/)).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: de.parentCrash.home }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledTimes(1);
    expect(captureException).toHaveBeenCalledWith(crash);
  });

  it("logs the crash for Loki without its message (#2512)", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

    render(
      <ParentError
        error={new TypeError("Anna is undefined")}
        retry={vi.fn()}
      />,
    );

    expect(warn).toHaveBeenCalledWith("error_displayed", {
      surface: "page",
      error_class: "crash",
      error_name: "TypeError",
    });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
    warn.mockRestore();
  });
});
