import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import { ToastProvider } from "~/contexts/ToastContext";
import { SettingsCategory } from "./settings-category";
import type { SchemaCategory } from "~/lib/settings-api";

function renderWithProviders(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>);
}

function makeCategory(overrides: Partial<SchemaCategory> = {}): SchemaCategory {
  return {
    key: "test-category",
    label: "Test Category",
    items: [
      {
        key: "test.enabled",
        label: "Enabled",
        description: "Toggle",
        type: "boolean",
        default: true,
        value: true,
        is_default: true,
        writable: true,
        visible: true,
        sort_order: 1,
        access_policy: "shared",
        validation: null,
        depends_on: null,
        options: null,
      },
      {
        key: "test.name",
        label: "Name",
        description: "A name",
        type: "text",
        default: "",
        value: "hello",
        is_default: false,
        writable: true,
        visible: true,
        sort_order: 2,
        access_policy: "shared",
        validation: null,
        depends_on: null,
        options: null,
      },
    ],
    ...overrides,
  };
}

describe("SettingsCategory", () => {
  it("renders category label", () => {
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Test Category")).toBeDefined();
  });

  it("renders all visible items", () => {
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Enabled")).toBeDefined();
    expect(getByText("Name")).toBeDefined();
  });

  it("renders nothing when all items are hidden", () => {
    const { container } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({
          items: [
            {
              key: "hidden.field",
              label: "Hidden",
              description: "",
              type: "text",
              default: "",
              value: "",
              is_default: true,
              writable: true,
              visible: false,
              sort_order: 1,
              access_policy: "shared",
              validation: null,
              depends_on: null,
              options: null,
            },
          ],
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(container.querySelector("h3")).toBeNull();
    expect(container.querySelector("label")).toBeNull();
  });

  it("renders nothing with empty items", () => {
    const { container } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({ items: [] })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(container.querySelector("h3")).toBeNull();
    expect(container.querySelector("label")).toBeNull();
  });
});
