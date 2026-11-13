import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { usePhoneViewport } from "~/test/helpers/viewport";
import germanMessages from "~/i18n/messages/de/holidayRegistration.json";
import { ApiError } from "~/lib/api-error";
import type {
  HolidayAddressRequirement,
  HolidayForm,
  HolidayFormField,
  HolidayHealthRequirement,
} from "~/lib/traeger-holiday-form-api";

const { mockSave, mockSavedAddress, mockSavedHealth } = vi.hoisted(() => ({
  mockSave: vi.fn(),
  mockSavedAddress: vi.fn(),
  mockSavedHealth: vi.fn(),
}));

vi.mock("~/lib/traeger-holiday-form-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveHolidayForm: (
    periodId: string,
    fields: HolidayFormField[],
    _blocks: unknown,
    address: HolidayAddressRequirement,
    health: HolidayHealthRequirement,
  ) => {
    mockSavedAddress(address);
    mockSavedHealth(health);
    return mockSave(periodId, fields);
  },
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
    addressRequirement: "off",
    healthRequirement: "optional",
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

// Mirrors the page: the editor remounts with each saved version.
function RemountingPage({ initial }: Readonly<{ initial: HolidayForm }>) {
  const [current, setCurrent] = useState(initial);
  return (
    <HolidayFormEditor
      key={current.version}
      periodId="5"
      form={current}
      onSaved={setCurrent}
    />
  );
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
    mockSavedAddress.mockReset();
    mockSavedHealth.mockReset();
  });

  it("switches the address block and saves it with the next version (#5625)", async () => {
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({
        ...form(),
        version: 2,
        fields,
        addressRequirement: "required",
      }),
    );
    const onSaved = renderEditor();
    const save = screen.getByRole("button", {
      name: "Als neue Version speichern",
    });
    const block = within(screen.getByRole("region", { name: "Anschrift" }));
    const off = block.getByRole("button", { name: "Nicht abfragen" });
    expect(off).toHaveAttribute("aria-pressed", "true");
    expect(save).toBeDisabled();
    expect(
      screen.queryByText(germanMessages.addressTitleOptional),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Allgemeine Fragen und Infos")).toBeInTheDocument();

    fireEvent.click(block.getByRole("button", { name: "Freiwillig" }));
    expect(
      screen.getByText(germanMessages.addressTitleOptional),
    ).toBeInTheDocument();
    expect(screen.getByText("Straße und Hausnummer")).toBeInTheDocument();
    fireEvent.click(block.getByRole("button", { name: "Pflicht" }));
    expect(screen.getByText("Straße und Hausnummer *")).toBeInTheDocument();
    expect(save).toBeEnabled();

    fireEvent.click(save);
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(mockSavedAddress).toHaveBeenCalledWith("required");
    expect(onSaved).toHaveBeenCalledWith(
      expect.objectContaining({ version: 2, addressRequirement: "required" }),
    );
  });

  it("requires allergies and medication with the next version (#6686)", async () => {
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({
        ...form(),
        version: 2,
        fields,
        healthRequirement: "required",
      }),
    );
    const onSaved = renderEditor();
    const save = screen.getByRole("button", {
      name: "Als neue Version speichern",
    });
    const block = within(screen.getByRole("region", { name: "Gesundheit" }));
    expect(block.getByRole("button", { name: "Freiwillig" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Allergien")).toBeInTheDocument();
    expect(
      screen.getByText("Gesundheitsangaben (wenn angegeben) *"),
    ).toBeInTheDocument();

    fireEvent.click(block.getByRole("button", { name: "Pflicht" }));
    expect(
      screen.getByText("Allergien: Nein, keine / Ja *"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Medikamente: Nein, keine / Ja *"),
    ).toBeInTheDocument();
    expect(screen.getByText("Gesundheitsangaben *")).toBeInTheDocument();
    expect(save).toBeEnabled();

    fireEvent.click(save);
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(mockSavedHealth).toHaveBeenCalledWith("required");
    expect(mockSavedAddress).toHaveBeenCalledWith("off");
    expect(onSaved).toHaveBeenCalledWith(
      expect.objectContaining({ version: 2, healthRequirement: "required" }),
    );
  });

  it("names own questions that repeat required health fields (#6686)", () => {
    renderEditor(form(TEMPLATE));
    const block = within(screen.getByRole("region", { name: "Gesundheit" }));
    expect(block.queryByRole("alert")).not.toBeInTheDocument();
    expect(block.getByText(/Eltern können Allergien/)).toBeInTheDocument();
    fireEvent.click(block.getByRole("button", { name: "Pflicht" }));
    expect(block.getByRole("alert")).toHaveTextContent(
      "Allergien und Medikamente fragt das Formular schon ab. Diese eigenen Fragen fragen es noch einmal: „Lebensmittelunverträglichkeiten“. Entfernen Sie sie, sonst antworten Eltern doppelt.",
    );
  });

  it("adds the class question with the standard template (#5625)", () => {
    const klasse: HolidayFormField = {
      key: "klasse",
      label: "Klasse",
      type: "text",
      applies_to_child: true,
      share_with_host: true,
      sort_order: 2,
    };
    render(
      <ToastProvider>
        <HolidayFormEditor
          periodId="5"
          form={{ ...form(TEMPLATE), template: [...TEMPLATE, klasse] }}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    expect(questionInputs().map((input) => input.value)).toEqual([
      "Lebensmittelunverträglichkeiten",
      "Darf an Ausflügen teilnehmen",
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "Standardvorlage einfügen" }),
    );
    expect(questionInputs().map((input) => input.value)).toEqual([
      "Lebensmittelunverträglichkeiten",
      "Darf an Ausflügen teilnehmen",
      "Klasse",
    ]);
  });

  it("names the preview sections like the parent form (#5467)", () => {
    renderEditor();
    expect(
      screen.getByText(germanMessages.childQuestionsTitle),
    ).toBeInTheDocument();
    expect(
      screen.getByText(germanMessages.familyQuestionsTitle),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Keine Fragen oder Infos zum Kind"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Keine allgemeinen Fragen oder Infos"),
    ).toBeInTheDocument();
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

  it("explains that a lone yes/no question needs another question", () => {
    renderEditor(form([TEMPLATE[1]!]));

    expect(
      screen.getByText(
        "Legen Sie zuerst eine andere Ja/Nein- oder Auswahlfrage mit Fragetext an. Diese Frage kann dann von deren Antwort abhängen.",
      ),
    ).toBeInTheDocument();
  });

  it("explains why a family question cannot depend on a child question", () => {
    renderEditor(
      form([
        { key: "family", label: "Familienfrage", type: "text", sort_order: 0 },
        TEMPLATE[1]!,
      ]),
    );

    expect(
      screen.getByText(
        "Eine allgemeine Frage kann nicht von einer Frage pro Kind abhängen. Legen Sie eine andere allgemeine Ja/Nein- oder Auswahlfrage an. Schreiben Sie auch den Fragetext dazu.",
      ),
    ).toBeInTheDocument();
  });

  it("asks for the missing text of a family question even with a child question present", () => {
    renderEditor(
      form([
        { key: "family", label: "Familienfrage", type: "text", sort_order: 0 },
        { key: "", label: "", type: "boolean", sort_order: 1 },
        TEMPLATE[1]!,
      ]),
    );

    const familyQuestion = within(screen.getAllByRole("article")[0]!);
    expect(
      familyQuestion.getByText(
        "Schreiben Sie zuerst den Fragetext der anderen Ja/Nein- oder Auswahlfrage. Diese Frage kann dann von deren Antwort abhängen.",
      ),
    ).toBeInTheDocument();
  });

  it("offers a condition once the other yes/no question has text", () => {
    renderEditor(
      form([
        { key: "family", label: "Familienfrage", type: "text", sort_order: 0 },
        { key: "", label: "", type: "boolean", sort_order: 1 },
      ]),
    );
    expect(
      screen.getByText(/Schreiben Sie zuerst den Fragetext/),
    ).toBeInTheDocument();

    fireEvent.change(questionInputs()[1]!, {
      target: { value: "Weitere Angaben nötig?" },
    });

    expect(
      screen.getByRole("checkbox", {
        name: /Nur unter einer Bedingung anzeigen/,
      }),
    ).toBeEnabled();
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
        share_with_host: true,
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
        // A health question keeps the default share since #4722.
        share_with_host: true,
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

  it("shares new questions by default, a health question too (#5470, #4722)", async () => {
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

    expect(choice(/^OGS sieht die Antwort/)).toBeChecked();
    expect(choice(/^OGS sieht die Antwort/, 1)).toBeChecked();
    // A health question can be shared too since #4722: the share stays on
    // and the row says the OGS sees it with the health information.
    fireEvent.click(choice(/^Gesundheitsangabe/, 1));
    const shared = choice(/^OGS sieht die Antwort/, 1);
    expect(shared).toBeChecked();
    expect(shared).toBeEnabled();
    expect(
      screen.getByText(
        "Die OGS, die die Woche ausrichtet, sieht die Antwort bei den Gesundheitsangaben des Kindes. Nur wenn die Eltern zustimmen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("OGS sieht es").length).toBeGreaterThan(0);

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
      { key: "forderbedarf", health: true, share_with_host: true },
    ]);
  });

  it("lets the Träger turn off sharing for a new question (#5470)", async () => {
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({ ...form(), version: 2, fields }),
    );
    renderEditor();
    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Taschengeld" },
    });
    const share = choice(/^OGS sieht die Antwort/);
    expect(share).toBeChecked();
    fireEvent.click(share);
    expect(share).not.toBeChecked();
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );
    await waitFor(() =>
      expect(mockSave).toHaveBeenCalledWith("5", [
        expect.objectContaining({ key: "taschengeld", share_with_host: false }),
      ]),
    );
  });

  it("keeps saved questions unshared when adding a new question (#5470)", async () => {
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({ ...form(), version: 2, fields }),
    );
    renderEditor(
      form([
        { key: "alt", label: "Alte Frage", type: "text", sort_order: 0 },
        {
          key: "privat",
          label: "Nicht freigegeben",
          type: "text",
          sort_order: 1,
          share_with_host: false,
        },
      ]),
    );
    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    fireEvent.change(questionInputs()[2]!, {
      target: { value: "Neue Frage" },
    });
    expect(choice(/^OGS sieht die Antwort/, 2)).toBeChecked();
    expect(choice(/^OGS sieht die Antwort/)).not.toBeChecked();
    expect(choice(/^OGS sieht die Antwort/, 1)).not.toBeChecked();
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );
    await waitFor(() =>
      expect(mockSave).toHaveBeenCalledWith("5", [
        expect.objectContaining({ key: "alt", share_with_host: false }),
        expect.objectContaining({ key: "privat", share_with_host: false }),
        expect.objectContaining({ key: "neue_frage", share_with_host: true }),
      ]),
    );
  });

  it("confirms a saved version with a toast that survives the remount (#5466)", async () => {
    mockSave.mockImplementation((_id: string, fields: HolidayFormField[]) =>
      Promise.resolve({ ...form(), version: 4, fields }),
    );
    render(
      <ToastProvider>
        <RemountingPage initial={{ ...form([TEMPLATE[1]!]), version: 3 }} />
      </ToastProvider>,
    );
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Ausflüge erlaubt" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    expect(
      await screen.findByText(
        "Das Anmeldeformular ist als Version 4 gespeichert.",
      ),
    ).toBeInTheDocument();
    // The remounted editor shows the saved version, not a draft.
    expect(questionInputs()[0]).toHaveValue("Ausflüge erlaubt");
    expect(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    ).toBeDisabled();
  });

  it("shows no success toast when saving fails (#5466)", async () => {
    mockSave.mockRejectedValue(new ApiError("boom", 500, {}));
    const onSaved = renderEditor(form([TEMPLATE[1]!]));
    fireEvent.change(questionInputs()[0]!, {
      target: { value: "Ausflüge erlaubt" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Als neue Version speichern" }),
      ).toBeEnabled(),
    );
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByText(/Das Anmeldeformular ist als Version/),
    ).not.toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
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

describe("HolidayFormEditor on a phone", () => {
  usePhoneViewport();
  beforeEach(() => {
    mockSave.mockReset();
  });

  // The long form stays walkable: each question folds to its head with a
  // summary; a new one opens, and so does one the check marks.
  it("folds the questions to their heads and opens a new one", () => {
    renderEditor(form(TEMPLATE));

    expect(
      screen.getByText(
        "Mehrzeiliger Text · freiwillig · pro Kind · Gesundheitsangabe",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: "Frage im Elternformular" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Frage 1 ausklappen" }));
    expect(questionInputs().map((input) => input.value)).toEqual([
      "Lebensmittelunverträglichkeiten",
    ]);

    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    expect(questionInputs()).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "Frage 3 einklappen" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("opens a folded question the check marks", () => {
    renderEditor(form(TEMPLATE));
    fireEvent.click(screen.getByRole("button", { name: /Frage hinzufügen/ }));
    fireEvent.click(screen.getByRole("button", { name: "Frage 3 einklappen" }));
    expect(
      screen.queryByRole("textbox", { name: "Frage im Elternformular" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );

    expect(questionInputs()).toHaveLength(1);
    expect(questionInputs()[0]).toHaveAttribute("aria-invalid", "true");
    expect(mockSave).not.toHaveBeenCalled();
  });
});
