/**
 * Tests for Database Types
 * Tests utility functions and type definitions
 */
import { describe, it, expect } from "vitest";
import {
  defineEntityConfig,
  configToFormSection,
  type SectionConfig,
} from "./types";
import { databaseThemes } from "@/components/ui/database/themes";

describe("defineEntityConfig", () => {
  it("returns the config as-is with proper typing", () => {
    const config = defineEntityConfig({
      name: {
        singular: "Test",
        plural: "Tests",
      },
      theme: databaseThemes.students,
      api: {
        basePath: "/api/test",
      },
      form: {
        sections: [],
      },
      detail: {
        sections: [],
      },
      list: {
        title: "Test",
        description: "Test description",
        searchPlaceholder: "Search...",
        item: {
          title: () => "Test",
        },
      },
    });

    expect(config.name.singular).toBe("Test");
    expect(config.name.plural).toBe("Tests");
    expect(config.api.basePath).toBe("/api/test");
  });
});

describe("configToFormSection", () => {
  it("converts SectionConfig to FormSection", () => {
    const sectionConfig: SectionConfig = {
      title: "Test Section",
      subtitle: "Test subtitle",
      fields: [
        {
          name: "test_field",
          label: "Test Field",
          type: "text",
          required: true,
          placeholder: "Enter text",
        },
      ],
      columns: 2,
      backgroundColor: "bg-blue-50",
      iconPath: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
    };

    const formSection = configToFormSection(sectionConfig);

    expect(formSection.title).toBe("Test Section");
    expect(formSection.subtitle).toBe("Test subtitle");
    expect(formSection.fields).toHaveLength(1);
    expect(formSection.fields[0]?.name).toBe("test_field");
    expect(formSection.fields[0]?.label).toBe("Test Field");
    expect(formSection.fields[0]?.type).toBe("text");
    expect(formSection.fields[0]?.required).toBe(true);
    expect(formSection.fields[0]?.placeholder).toBe("Enter text");
    expect(formSection.columns).toBe(2);
    expect(formSection.backgroundColor).toBe("bg-blue-50");
    expect(formSection.iconPath).toBe("M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z");
  });

  it("converts multiple fields correctly", () => {
    const sectionConfig: SectionConfig = {
      title: "Multi Field Section",
      fields: [
        {
          name: "field1",
          label: "Field 1",
          type: "text",
        },
        {
          name: "field2",
          label: "Field 2",
          type: "email",
          required: true,
        },
        {
          name: "field3",
          label: "Field 3",
          type: "select",
          options: [
            { value: "a", label: "A" },
            { value: "b", label: "B" },
          ],
        },
      ],
    };

    const formSection = configToFormSection(sectionConfig);

    expect(formSection.fields).toHaveLength(3);
    expect(formSection.fields[0]?.name).toBe("field1");
    expect(formSection.fields[1]?.name).toBe("field2");
    expect(formSection.fields[1]?.required).toBe(true);
    expect(formSection.fields[2]?.name).toBe("field3");
    expect(formSection.fields[2]?.options).toHaveLength(2);
  });

  it("preserves field helper text and validation", () => {
    const sectionConfig: SectionConfig = {
      title: "Test Section",
      fields: [
        {
          name: "validated_field",
          label: "Validated Field",
          type: "text",
          helperText: "Helper text",
          validation: (value) => (value === "test" ? null : "Invalid value"),
        },
      ],
    };

    const formSection = configToFormSection(sectionConfig);

    expect(formSection.fields[0]?.helperText).toBe("Helper text");
    expect(formSection.fields[0]?.validation).toBeDefined();
    expect(formSection.fields[0]?.validation?.("test")).toBeNull();
    expect(formSection.fields[0]?.validation?.("wrong")).toBe("Invalid value");
  });

  it("preserves number field constraints", () => {
    const sectionConfig: SectionConfig = {
      title: "Number Section",
      fields: [
        {
          name: "number_field",
          label: "Number",
          type: "number",
          min: 1,
          max: 100,
        },
      ],
    };

    const formSection = configToFormSection(sectionConfig);

    expect(formSection.fields[0]?.min).toBe(1);
    expect(formSection.fields[0]?.max).toBe(100);
  });

  it("preserves colSpan property", () => {
    const sectionConfig: SectionConfig = {
      title: "Span Section",
      fields: [
        {
          name: "wide_field",
          label: "Wide Field",
          type: "textarea",
          colSpan: 2,
        },
      ],
    };

    const formSection = configToFormSection(sectionConfig);

    expect(formSection.fields[0]?.colSpan).toBe(2);
  });
});
