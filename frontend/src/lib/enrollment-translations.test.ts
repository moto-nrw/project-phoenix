import { describe, expect, it } from "vitest";
import {
  localizeFormFields,
  localizeLegalBlocks,
  localizeNamed,
  localizeOfferings,
  selectionGroupLabel,
  translatedText,
  translationStatus,
  withTranslation,
} from "./enrollment-translations";

describe("translationStatus", () => {
  it("distinguishes missing, current and stale translations", () => {
    expect(translationStatus(undefined, "Name")).toBe("missing");
    expect(translationStatus({ text: "  " }, "Name")).toBe("missing");
    expect(translationStatus({ text: "Имя", source: "Name " }, " Name")).toBe(
      "current",
    );
    expect(translationStatus({ text: "Имя", source: "Vorname" }, "Name")).toBe(
      "stale",
    );
  });
});

describe("withTranslation", () => {
  it("records the German text the translator saw", () => {
    expect(withTranslation(undefined, "ru", "label", "Имя", " Name ")).toEqual({
      ru: { label: { text: "Имя", source: "Name" } },
    });
  });

  it("confirms a stale translation by rewriting its source", () => {
    const stale = { ru: { label: { text: "Имя", source: "Vorname" } } };
    const next = withTranslation(stale, "ru", "label", "Имя", "Name");
    expect(translationStatus(next.ru?.label, "Name")).toBe("current");
    expect(stale.ru.label.source).toBe("Vorname");
  });

  it("removes an emptied entry and its emptied language", () => {
    const stored = {
      ru: { label: { text: "Имя", source: "Name" } },
      en: { label: { text: "Name", source: "Name" } },
    };
    expect(withTranslation(stored, "ru", "label", " ", "Name")).toEqual({
      en: { label: { text: "Name", source: "Name" } },
    });
  });
});

describe("translatedText", () => {
  const entity = {
    translations: {
      ru: {
        label: { text: "Имя" },
        help_text: { text: "Подсказка", source: "Alter Hinweis" },
      },
    },
  };

  it("returns the translation of a pre-filtered public payload", () => {
    expect(translatedText(entity, "label", "Name", "ru")).toBe("Имя");
  });

  it("falls back to German per attribute", () => {
    expect(translatedText(entity, "label", "Name", "de")).toBe("Name");
    expect(translatedText(entity, "label", "Name", "uk")).toBe("Name");
    expect(translatedText(entity, "content", "Text", "ru")).toBe("Text");
  });

  it("ignores a stored translation whose German source changed", () => {
    expect(translatedText(entity, "help_text", "Neuer Hinweis", "ru")).toBe(
      "Neuer Hinweis",
    );
  });
});

describe("selectionGroupLabel", () => {
  const offerings = [
    { selection_group: "Betreuungsumfang" },
    {
      selection_group: " Betreuungsumfang ",
      translations: { ru: { selection_group: { text: "Объём присмотра" } } },
    },
    {
      selection_group: "Essen",
      translations: { ru: { selection_group: { text: "Питание" } } },
    },
  ];

  it("takes the translation from any offering of the group", () => {
    expect(selectionGroupLabel(offerings, "Betreuungsumfang", "ru")).toBe(
      "Объём присмотра",
    );
  });

  it("falls back to the German group name", () => {
    expect(selectionGroupLabel(offerings, "Betreuungsumfang", "uk")).toBe(
      "Betreuungsumfang",
    );
    expect(selectionGroupLabel(offerings, "Betreuungsumfang", "de")).toBe(
      "Betreuungsumfang",
    );
  });
});

describe("localize helpers", () => {
  it("translate field texts and option labels, keeping values", () => {
    const fields = [
      {
        key: "meal",
        label: "Essen",
        help_text: "",
        options: [
          {
            label: "Vegetarisch",
            value: "veg",
            translations: { ru: { label: { text: "Вегетарианское" } } },
          },
          { label: "Normal", value: "normal" },
        ],
        translations: { ru: { label: { text: "Питание" } } },
      },
    ];

    const [field] = localizeFormFields(fields, "ru");

    expect(field?.label).toBe("Питание");
    expect(field?.help_text).toBe("");
    expect(field?.options).toEqual([
      expect.objectContaining({ label: "Вегетарианское", value: "veg" }),
      { label: "Normal", value: "normal" },
    ]);
    expect(fields[0]?.label).toBe("Essen");
    expect(localizeFormFields(fields, "de")).toBe(fields);
  });

  it("translate legal blocks, offerings and names", () => {
    const [block] = localizeLegalBlocks(
      [
        {
          key: "agb",
          title: "AGB",
          label: "Ich stimme zu.",
          text: "Bedingungen",
          translations: { en: { title: { text: "Terms" } } },
        },
      ],
      "en",
    );
    expect(block).toMatchObject({
      key: "agb",
      title: "Terms",
      label: "Ich stimme zu.",
    });

    const [offering] = localizeOfferings(
      [
        {
          id: "1",
          name: "OGS",
          description: null,
          translations: { ru: { name: { text: "Продлёнка" } } },
        },
      ],
      "ru",
    );
    expect(offering).toMatchObject({ name: "Продлёнка", description: null });

    expect(
      localizeNamed(
        { name: "Schuljahr", translations: { pl: { name: { text: "Rok" } } } },
        "pl",
      ).name,
    ).toBe("Rok");
  });
});
