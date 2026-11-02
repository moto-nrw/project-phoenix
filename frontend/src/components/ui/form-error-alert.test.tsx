import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FormErrorAlert, LoadErrorAlert } from "./form-error-alert";

const session = vi.hoisted(() => ({ permissions: [] as string[] }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "1", roles: [], permissions: session.permissions },
      expires: "",
    },
    status: "authenticated",
  }),
}));

describe("FormErrorAlert", () => {
  afterEach(() => {
    session.permissions = [];
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

  it("scrolls to the first marked field of its form instead of itself", () => {
    const scrolled: Element[] = [];
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(function (
      this: Element,
    ) {
      scrolled.push(this);
    });

    render(
      <form>
        <FormErrorAlert message="Bitte prüfen Sie die markierten Felder." />
        <input aria-label="Vorname" />
        <button type="button" data-form-invalid="true">
          Einwilligung
        </button>
        <input aria-label="Telefon" aria-invalid="true" />
      </form>,
    );

    expect(scrolled).toEqual([
      screen.getByRole("button", { name: "Einwilligung" }),
    ]);
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

  it("explains a form refusal to staff through their Leitung", () => {
    render(
      <FormErrorAlert
        message={{
          message: "Katalogtext",
          notBookedModule: "zeiterfassung",
          attempt: 1,
        }}
      />,
    );

    expect(screen.getByRole("status")).toHaveTextContent(
      "Diese Seite gehört zum Modul Zeiterfassung. Ihre Schule nutzt es nicht. Wenn Sie es brauchen, sprechen Sie mit Ihrer Leitung.",
    );
  });

  it("explains a load refusal to the Leitung", () => {
    session.permissions = ["config:manage"];

    render(
      <LoadErrorAlert
        error={{
          message: "Katalogtext",
          notBookedModule: "zeiterfassung",
          attempt: 1,
        }}
      />,
    );

    expect(screen.getByRole("status")).toHaveTextContent(
      "Diese Seite gehört zum Modul Zeiterfassung. Ihre Schule hat es nicht gebucht. Für eine Buchung wenden Sie sich an das moto-Team.",
    );
  });

  it("puts a retry beside the message only without a request ID", () => {
    const retry = { label: "Wiederholen", onClick: vi.fn() };
    const { rerender } = render(
      <LoadErrorAlert
        error={{ message: "Laden fehlgeschlagen", retry, attempt: 1 }}
        actionLayout="responsive"
      />,
    );
    // Beside the message from sm on.
    expect(
      screen
        .getByRole("button", { name: "Wiederholen" })
        .closest("[class*='sm:basis-auto']"),
    ).not.toBeNull();

    rerender(
      <LoadErrorAlert
        error={{
          message: "Laden fehlgeschlagen",
          retry,
          requestId: {
            value: "8c3f2a1e-5b7d-4e9a-a6c2-1f0d9b3e7a54",
            label: "Vorgangskennung: {requestId}",
            copyLabel: "Vorgangskennung kopieren",
            copiedLabel: "Kopiert",
            copyFailedLabel: "Nicht kopiert",
          },
          attempt: 1,
        }}
        actionLayout="responsive"
      />,
    );
    // The long ID would squeeze the message: under it, as by default.
    expect(
      screen
        .getByRole("button", { name: "Wiederholen" })
        .closest("[class*='sm:basis-auto']"),
    ).toBeNull();
    expect(
      screen
        .getByRole("button", { name: "Wiederholen" })
        .closest("[class*='basis-full']"),
    ).not.toBeNull();
  });
});
