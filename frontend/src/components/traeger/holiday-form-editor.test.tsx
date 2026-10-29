import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  HolidayForm,
  HolidayFormField,
} from "~/lib/traeger-holiday-form-api";

const { mockSave } = vi.hoisted(() => ({ mockSave: vi.fn() }));

vi.mock("~/lib/traeger-holiday-form-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveHolidayForm: (periodId: string, fields: HolidayFormField[]) =>
    mockSave(periodId, fields),
}));

import { HolidayFormEditor } from "./holiday-form-editor";

const TEMPLATE: HolidayFormField[] = [
  {
    key: "lebensmittelunvertraeglichkeiten",
    label: "Lebensmittelunverträglichkeiten",
    type: "textarea",
    applies_to_child: true,
    health: true,
    sort_order: 0,
  },
  {
    key: "darf_an_ausfluegen_teilnehmen",
    label: "Darf an Ausflügen teilnehmen",
    type: "boolean",
    applies_to_child: true,
    sort_order: 1,
  },
];

function form(fields: HolidayFormField[] = []): HolidayForm {
  return {
    version: 1,
    fields,
    createdAt: "2026-10-01T08:00:00Z",
    template: TEMPLATE,
  };
}

function renderEditor(initial: HolidayForm = form()) {
  const onSaved = vi.fn();
  render(
    <ToastProvider>
      <HolidayFormEditor periodId="5" form={initial} onSaved={onSaved} />
    </ToastProvider>,
  );
  return onSaved;
}

function questionInputs(): HTMLInputElement[] {
  return screen.getAllByRole("textbox", {
    name: "Frage im Elternformular",
  }) as HTMLInputElement[];
}

function choice(name: RegExp, index = 0): HTMLElement {
  return screen.getAllByRole("checkbox", { name })[index]!;
}

describe("HolidayFormEditor (#5198)", () => {
  beforeEach(() => {
    mockSave.mockReset();
  });

  it("inserts the missing questions of the standard template once", () => {
    renderEditor(form([TEMPLATE[1]!]));
    const insert = screen.getByRole("button", {
      name: "Standardvorlage einfügen",
    });
    fireEvent.click(insert);

    expect(
      screen.getAllByText("Lebensmittelunverträglichkeiten").length,
    ).toBeGreaterThan(0);
    expect(insert).toBeDisabled();
    expect(
      screen.getByText("Alle Fragen der Standardvorlage sind im Formular."),
    ).toBeInTheDocument();
  });

  it("offers only the holiday field types and conditions on other questions", () => {
    renderEditor();
    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    fireEvent.click(screen.getByRole("combobox", { name: "Typ" }));
    const types = screen.getAllByRole("option").map((o) => o.textContent);
    expect(types).toEqual([
      "Ja/Nein",
      "Text",
      "Mehrzeiliger Text",
      "Datum",
      "Auswahl",
    ]);
    // A question in the family scope sees no answer to depend on yet.
    expect(
      screen.getByText("Keine Sichtbarkeitsregel möglich"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/oder dieses Feld pro Kind angezeigt wird/),
    ).not.toBeInTheDocument();
  });

  it("saves questions per child, required, health, condition and order as a new version", async () => {
    const saved = form([]);
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({ ...saved, version: 2, fields }),
    );
    const onSaved = renderEditor();
    const add = screen.getByRole("button", { name: /Frage hinzufügen/ });

    fireEvent.click(add);
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Förderbedarf" },
    });
    fireEvent.click(choice(/^Pflichtfrage/));
    fireEvent.click(choice(/^Pro Kind abfragen/));
    fireEvent.click(choice(/^Gesundheitsangabe/));

    fireEvent.click(add);
    fireEvent.change(questionInputs()[1]!, {
      target: { value: "Geht allein nach Hause" },
    });
    fireEvent.click(screen.getAllByRole("combobox", { name: "Typ" })[1]!);
    fireEvent.click(screen.getByRole("option", { name: "Ja/Nein" }));
    fireEvent.click(choice(/^Pro Kind abfragen/, 1));
    // Move the yes/no question first, then let the other one depend on it.
    fireEvent.click(
      screen.getAllByRole("button", {
        name: "Frage nach oben verschieben",
      })[1]!,
    );
    // The yes/no question itself has nothing to depend on; only the second
    // question offers the condition.
    fireEvent.click(choice(/^Nur unter einer Bedingung anzeigen/));

    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    await waitFor(() => expect(mockSave).toHaveBeenCalledTimes(1));
    const [periodId, fields] = mockSave.mock.calls[0] as [
      string,
      HolidayFormField[],
    ];
    expect(periodId).toBe("5");
    expect(fields).toEqual([
      {
        key: "geht_allein_nach_hause",
        label: "Geht allein nach Hause",
        type: "boolean",
        required: false,
        help_text: "",
        sort_order: 0,
        applies_to_child: true,
        health: false,
        share_with_host: false,
      },
      {
        key: "forderbedarf",
        label: "Förderbedarf",
        type: "text",
        required: true,
        help_text: "",
        sort_order: 1,
        applies_to_child: true,
        health: true,
        share_with_host: false,
        visible_when: {
          source: "field",
          field: "geht_allein_nach_hause",
          operator: "eq",
          value: true,
        },
      },
    ]);
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
  });

  it("shares a question without health information with the hosting OGS (#5202)", async () => {
    const saved = form([]);
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({ ...saved, version: 2, fields }),
    );
    renderEditor();
    const add = screen.getByRole("button", { name: /Frage hinzufügen/ });
    fireEvent.click(add);
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Taschengeld" },
    });
    fireEvent.click(add);
    fireEvent.change(questionInputs()[1]!, {
      target: { value: "Förderbedarf" },
    });

    fireEvent.click(choice(/^OGS sieht die Antwort/));
    fireEvent.click(choice(/^OGS sieht die Antwort/, 1));
    expect(choice(/^OGS sieht die Antwort/, 1)).toBeChecked();
    // A health question stays with the Träger: the share goes off and
    // cannot be chosen, and the row says why.
    fireEvent.click(choice(/^Gesundheitsangabe/, 1));
    const blocked = choice(/^OGS sieht die Antwort/, 1);
    expect(blocked).not.toBeChecked();
    expect(blocked).toBeDisabled();
    expect(
      screen.getByText(
        "Nicht bei Gesundheitsangaben. Diese bleiben beim Träger.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("OGS sieht es")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    await waitFor(() => expect(mockSave).toHaveBeenCalledTimes(1));
    const [, fields] = mockSave.mock.calls[0] as [string, HolidayFormField[]];
    expect(
      fields.map(({ key, health, share_with_host }) => ({
        key,
        health,
        share_with_host,
      })),
    ).toEqual([
      { key: "taschengeld", health: false, share_with_host: true },
      { key: "forderbedarf", health: true, share_with_host: false },
    ]);
  });

  it("marks a question without text instead of saving", async () => {
    renderEditor();
    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Fragen."),
    ).toBeInTheDocument();
    expect(questionInputs()[0]).toHaveAttribute("aria-invalid", "true");
    expect(mockSave).not.toHaveBeenCalled();
  });

  it("shows a refused question from the server at the question", async () => {
    mockSave.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "enrollment.holiday_form_field_not_allowed",
        errors: [{ field: "fields.0.label", reason: "not allowed" }],
      }),
    );
    renderEditor(form([TEMPLATE[1]!]));
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Darf mit auf Ausflüge" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    await waitFor(() =>
      expect(questionInputs()[0]).toHaveAttribute("aria-invalid", "true"),
    );
  });

  it("keeps saving off while nothing changed and discards a draft", () => {
    renderEditor(form([TEMPLATE[1]!]));
    const save = screen.getByRole("button", {
      name: "Als neue Version speichern",
    });
    expect(save).toBeDisabled();
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Ausflüge erlaubt" },
    });
    expect(save).toBeEnabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen verwerfen" }),
    );
    expect(questionInputs()[0]).toHaveValue("Darf an Ausflügen teilnehmen");
    expect(save).toBeDisabled();
  });

  it("restores saved selection options when discarding a draft", () => {
    renderEditor(
      form([
        {
          key: "anreise",
          label: "Anreise",
          type: "select",
          options: [
            { label: "Bus", value: "bus" },
            { label: "Bahn", value: "bahn" },
          ],
          sort_order: 0,
        },
      ]),
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Auswahloptionen" }), {
      target: { value: "Bus\nFahrrad" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen verwerfen" }),
    );

    expect(
      screen.getByRole("textbox", { name: "Auswahloptionen" }),
    ).toHaveValue("Bus\nBahn");
    expect(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    ).toBeDisabled();
  });
});
