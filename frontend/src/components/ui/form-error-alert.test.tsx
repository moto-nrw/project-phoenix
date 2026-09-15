import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FormErrorAlert } from "./form-error-alert";

describe("FormErrorAlert", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders nothing without a message", () => {
    const { container } = render(<FormErrorAlert message={null} />);

    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the message as an assertive error alert", () => {
    render(<FormErrorAlert message="Bitte einen Titel eintragen." />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bitte einen Titel eintragen.",
    );
  });

  it("scrolls into view when the message appears or changes", () => {
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(() => {});

    const { rerender } = render(<FormErrorAlert message={null} />);
    expect(scrollIntoView).not.toHaveBeenCalled();

    rerender(<FormErrorAlert message="Erster Fehler" />);
    expect(scrollIntoView).toHaveBeenCalledTimes(1);

    rerender(<FormErrorAlert message="Erster Fehler" />);
    expect(scrollIntoView).toHaveBeenCalledTimes(1);

    rerender(<FormErrorAlert message="Zweiter Fehler" />);
    expect(scrollIntoView).toHaveBeenCalledTimes(2);
  });

  it("scrolls again for a repeated attempt with the same message", () => {
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(() => {});

    const { rerender } = render(
      <FormErrorAlert message={{ message: "Gleicher Fehler", attempt: 1 }} />,
    );
    expect(scrollIntoView).toHaveBeenCalledTimes(1);

    rerender(
      <FormErrorAlert message={{ message: "Gleicher Fehler", attempt: 2 }} />,
    );
    expect(scrollIntoView).toHaveBeenCalledTimes(2);
    expect(screen.getByRole("alert")).toHaveTextContent("Gleicher Fehler");
  });
});
