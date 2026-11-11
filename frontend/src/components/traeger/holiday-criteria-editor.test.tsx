import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayCriterion } from "~/lib/traeger-holiday-allocation-api";

const { save } = vi.hoisted(() => ({ save: vi.fn() }));
vi.mock("~/lib/traeger-holiday-allocation-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-holiday-allocation-api")>()),
  saveHolidayCriteria: (...args: unknown[]) => save(...args),
}));

import { HolidayCriteriaEditor } from "./holiday-criteria-editor";

const STORED: HolidayCriterion[] = [
  {
    id: "1",
    name: "Beide Eltern berufstätig",
    question: "Sind beide Eltern berufstätig?",
    scope: "family",
    active: true,
    position: 1,
    answered: true,
  },
];

function renderEditor(onSaved = vi.fn()) {
  render(
    <ToastProvider>
      <HolidayCriteriaEditor criteria={STORED} onSaved={onSaved} />
    </ToastProvider>,
  );
  return onSaved;
}

describe("holiday criteria editor (#4700)", () => {
  beforeEach(() => {
    save.mockReset();
  });

  it("adds a template, moves it to the top and saves the list in that order", async () => {
    const saved: HolidayCriterion[] = [];
    save.mockResolvedValue(saved);
    const onSaved = renderEditor();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Vorlage „Geschwisterkind ist schon angemeldet“ hinzufügen",
      }),
    );
    expect(
      screen.queryByRole("button", {
        name: "Vorlage „Geschwisterkind ist schon angemeldet“ hinzufügen",
      }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", {
        name: "„Geschwisterkind ist schon angemeldet“ nach oben, höhere Priorität",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Kriterien speichern" }),
    );

    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
    expect(save.mock.calls[0]![0]).toEqual([
      {
        id: "",
        name: "Geschwisterkind ist schon angemeldet",
        question:
          "Ist ein Geschwisterkind schon für diese Ferienbetreuung angemeldet?",
        scope: "child",
        active: true,
      },
      {
        id: "1",
        name: "Beide Eltern berufstätig",
        question: "Sind beide Eltern berufstätig?",
        scope: "family",
        active: true,
      },
    ]);
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(saved));
  });

  it("shows the parents' view of the question and keeps the scope of an answered criterion", () => {
    renderEditor();
    const preview = screen.getByText(
      "So sehen Eltern die Frage",
    ).parentElement!;
    expect(
      within(preview).getByText("Sind beide Eltern berufstätig?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Eltern haben schon geantwortet. Darum bleibt das so."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jedes Kind" })).toBeDisabled();
  });

  it("marks an empty own criterion at its fields before saving", async () => {
    renderEditor();
    fireEvent.click(screen.getByRole("button", { name: "Eigenes Kriterium" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Kriterien speichern" }),
    );
    expect(
      await screen.findByText("Bitte geben Sie einen Namen ein."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte schreiben Sie die Frage an die Eltern."),
    ).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });

  it("shows a refused scope at its choice", async () => {
    save.mockRejectedValue(
      new ApiError("invalid", 400, {
        code: "general.input",
        errors: [{ field: "criteria.0.scope", reason: "answered" }],
      }),
    );
    renderEditor();
    fireEvent.change(screen.getByLabelText(/^Name/), {
      target: { value: "Beide Eltern arbeiten" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Kriterien speichern" }),
    );
    const group = await screen.findByRole("group", {
      name: "Gilt für: Beide Eltern arbeiten",
    });
    await waitFor(() =>
      expect(group).toHaveAttribute("aria-describedby", expect.any(String)),
    );
    const hint = document.getElementById(
      group.getAttribute("aria-describedby")!,
    );
    expect(hint).toBeInTheDocument();
    expect(hint).not.toBeEmptyDOMElement();
  });

  it("puts a refusal of the backend at its field", async () => {
    save.mockRejectedValue(
      new ApiError("invalid", 400, {
        code: "general.input",
        errors: [{ field: "criteria.0.name", reason: "too_long" }],
      }),
    );
    renderEditor();
    fireEvent.change(screen.getByLabelText(/^Name/), {
      target: { value: "Beide Eltern arbeiten" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Kriterien speichern" }),
    );
    expect(
      await screen.findByText(/Bitte prüfen Sie Ihre Angaben\./),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText(/^Name/)).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
  });
});
