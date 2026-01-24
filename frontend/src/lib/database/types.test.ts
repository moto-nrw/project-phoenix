/**
 * Tests for database types utility functions
 */
import { describe, it, expect } from "vitest";
import type { SectionConfig, EntityConfig, FieldConfig } from "./types";
import { defineEntityConfig, configToFormSection } from "./types";

describe("defineEntityConfig", () => {
  it("should return the same config object (identity function)", () => {
    const config: EntityConfig<{ id: string; name: string }> = {
      name: { singular: "Item", plural: "Items" },
      theme: {
        name: "students",
        colors: {
          primary: "#000",
          secondary: "#fff",
          background: "#eee",
          text: "#333",
          border: "#ccc",
        },
        gradients: {
          header: "linear-gradient(to right, #000, #333)",
        },
        icons: {
          main: "/icons/test.svg",
        },
      },
      api: { basePath: "/api/items" },
      form: { sections: [] },
      detail: { sections: [] },
      list: {
        title: "Items",
        description: "List of items",
        searchPlaceholder: "Search items...",
        item: { title: (e) => e.name },
      },
    };

    const result = defineEntityConfig(config);

    expect(result).toBe(config);
    expect(result.name.singular).toBe("Item");
    expect(result.name.plural).toBe("Items");
  });

  it("should preserve all config properties", () => {
    const config: EntityConfig<{ id: string }> = {
      name: { singular: "Test", plural: "Tests" },
      theme: {
        name: "students",
        colors: {
          primary: "#123",
          secondary: "#456",
          background: "#789",
          text: "#abc",
          border: "#def",
        },
        gradients: { header: "gradient" },
        icons: { main: "/icon.svg" },
      },
      api: {
        basePath: "/api/tests",
        endpoints: {
          list: "/api/tests/all",
          get: "/api/tests/{id}",
        },
      },
      form: {
        sections: [
          {
            title: "Section 1",
            fields: [{ name: "field1", label: "Field 1", type: "text" }],
          },
        ],
        defaultValues: { id: "default" },
      },
      detail: {
        sections: [
          {
            title: "Details",
            items: [{ label: "ID", value: (e) => e.id }],
          },
        ],
      },
      list: {
        title: "Tests",
        description: "All tests",
        searchPlaceholder: "Search...",
        item: { title: (e) => e.id },
        features: { create: true, search: true },
      },
      labels: {
        createButton: "Add Test",
        emptyState: "No tests found",
      },
    };

    const result = defineEntityConfig(config);

    expect(result).toEqual(config);
    expect(result.api.endpoints?.list).toBe("/api/tests/all");
    expect(result.form.defaultValues?.id).toBe("default");
    expect(result.labels?.createButton).toBe("Add Test");
  });
});

describe("configToFormSection", () => {
  it("should convert a simple SectionConfig to FormSection", () => {
    const sectionConfig: SectionConfig = {
      title: "Basic Info",
      fields: [
        { name: "name", label: "Name", type: "text", required: true },
        { name: "email", label: "Email", type: "email" },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.title).toBe("Basic Info");
    expect(result.fields).toHaveLength(2);
    expect(result.fields[0]).toEqual({
      name: "name",
      label: "Name",
      type: "text",
      required: true,
      placeholder: undefined,
      options: undefined,
      validation: undefined,
      component: undefined,
      helperText: undefined,
      autoComplete: undefined,
      colSpan: undefined,
      min: undefined,
      max: undefined,
    });
  });

  it("should preserve subtitle and columns", () => {
    const sectionConfig: SectionConfig = {
      title: "Section Title",
      subtitle: "Section Subtitle",
      columns: 2,
      fields: [],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.title).toBe("Section Title");
    expect(result.subtitle).toBe("Section Subtitle");
    expect(result.columns).toBe(2);
    expect(result.fields).toEqual([]);
  });

  it("should preserve backgroundColor and iconPath", () => {
    const sectionConfig: SectionConfig = {
      title: "Styled Section",
      backgroundColor: "#f0f0f0",
      iconPath: "/icons/section.svg",
      fields: [],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.backgroundColor).toBe("#f0f0f0");
    expect(result.iconPath).toBe("/icons/section.svg");
  });

  it("should convert all field properties", () => {
    const validationFn = (value: unknown) =>
      value ? null : "Value is required";
    const CustomComponent = () => null;

    const field: FieldConfig = {
      name: "customField",
      label: "Custom Field",
      type: "custom",
      required: true,
      placeholder: "Enter value",
      helperText: "Help text here",
      validation: validationFn,
      options: [{ value: "1", label: "Option 1" }],
      component: CustomComponent,
      colSpan: 2,
      autoComplete: "off",
      min: 0,
      max: 100,
    };

    const sectionConfig: SectionConfig = {
      title: "Test",
      fields: [field],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields[0]).toEqual({
      name: "customField",
      label: "Custom Field",
      type: "custom",
      required: true,
      placeholder: "Enter value",
      helperText: "Help text here",
      validation: validationFn,
      options: [{ value: "1", label: "Option 1" }],
      component: CustomComponent,
      colSpan: 2,
      autoComplete: "off",
      min: 0,
      max: 100,
    });
  });

  it("should handle number fields with min/max", () => {
    const sectionConfig: SectionConfig = {
      title: "Numbers",
      fields: [
        {
          name: "age",
          label: "Age",
          type: "number",
          min: 0,
          max: 150,
        },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields[0].type).toBe("number");
    expect(result.fields[0].min).toBe(0);
    expect(result.fields[0].max).toBe(150);
  });

  it("should handle select fields with options array", () => {
    const options = [
      { value: "a", label: "Option A" },
      { value: "b", label: "Option B" },
    ];

    const sectionConfig: SectionConfig = {
      title: "Select",
      fields: [
        {
          name: "choice",
          label: "Choice",
          type: "select",
          options: options,
        },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields[0].type).toBe("select");
    expect(result.fields[0].options).toEqual(options);
  });

  it("should handle select fields with async options function", () => {
    const loadOptions = async () => [{ value: "1", label: "Dynamic" }];

    const sectionConfig: SectionConfig = {
      title: "Async Select",
      fields: [
        {
          name: "dynamic",
          label: "Dynamic Choice",
          type: "select",
          options: loadOptions,
        },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields[0].options).toBe(loadOptions);
  });

  it("should handle multiple fields in sequence", () => {
    const sectionConfig: SectionConfig = {
      title: "Form",
      fields: [
        { name: "first", label: "First", type: "text" },
        { name: "second", label: "Second", type: "email" },
        { name: "third", label: "Third", type: "textarea" },
        { name: "fourth", label: "Fourth", type: "checkbox" },
        { name: "fifth", label: "Fifth", type: "date" },
        { name: "sixth", label: "Sixth", type: "password" },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields).toHaveLength(6);
    expect(result.fields.map((f) => f.type)).toEqual([
      "text",
      "email",
      "textarea",
      "checkbox",
      "date",
      "password",
    ]);
  });

  it("should handle multiselect type", () => {
    const sectionConfig: SectionConfig = {
      title: "Multi",
      fields: [
        {
          name: "tags",
          label: "Tags",
          type: "multiselect",
          options: [
            { value: "tag1", label: "Tag 1" },
            { value: "tag2", label: "Tag 2" },
          ],
        },
      ],
    };

    const result = configToFormSection(sectionConfig);

    expect(result.fields[0].type).toBe("multiselect");
  });
});
