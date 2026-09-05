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

  it("overrides the 'aktivitaeten' key to the umlaut label", () => {
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({ key: "aktivitaeten", label: "aktivitaeten" })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Aktivitäten")).toBeDefined();
  });

  it("uses the product term Betreuungsplan for timetable settings", () => {
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({ key: "stundenplan", label: "Stundenplan" })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Betreuungsplan")).toBeDefined();
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

  it("hides empty enrollment legal text fields while their toggle is off", () => {
    const { queryByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({
          key: "rechtstexte",
          items: [
            {
              key: "enrollment.legal_dsgvo_enabled",
              label: "Datenschutzinformation anzeigen",
              description: "",
              type: "boolean",
              default: false,
              value: false,
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
              key: "enrollment.legal_dsgvo_text",
              label: "Datenschutzinformation",
              description: "",
              type: "textarea",
              default: "",
              value: "",
              is_default: true,
              writable: true,
              visible: true,
              sort_order: 2,
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

    expect(queryByText("Datenschutzinformation anzeigen")).toBeDefined();
    expect(queryByText("Datenschutzinformation")).toBeNull();
  });

  it("hides enrollment legal text fields while their toggle is off even when text exists", () => {
    const { getByText, queryByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({
          key: "rechtstexte",
          items: [
            {
              key: "enrollment.legal_dsgvo_enabled",
              label: "Datenschutzinformation anzeigen",
              description: "",
              type: "boolean",
              default: false,
              value: false,
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
              key: "enrollment.legal_dsgvo_text",
              label: "Datenschutzinformation",
              description: "",
              type: "textarea",
              default: "",
              value: "Bestehender Datenschutztext",
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
        })}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );

    expect(getByText("Datenschutzinformation anzeigen")).toBeDefined();
    expect(
      getByText("Text ist gespeichert, wird aber nicht angezeigt."),
    ).toBeDefined();
    expect(queryByText("Datenschutzinformation")).toBeNull();
    expect(queryByText("Bestehender Datenschutztext")).toBeNull();
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

describe("SettingsCategory collapsible (#2830)", () => {
  it("collapsed: shows the summary line and hides the fields", () => {
    const onToggle = vi.fn();
    const { getByRole, getByText, queryByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        collapsible
        collapsed
        onToggle={onToggle}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    const heading = getByRole("button", { name: /Test Category/ });
    expect(heading.getAttribute("aria-expanded")).toBe("false");
    expect(getByText("Enabled, Name")).toBeDefined();
    expect(getByText("1 geändert")).toBeDefined();
    expect(queryByText("Toggle")).toBeNull();
    expect(queryByText("A name")).toBeNull();

    heading.click();
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("expanded: renders the fields and no summary", () => {
    const { getByRole, getByText, queryByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        collapsible
        collapsed={false}
        onToggle={vi.fn()}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(
      getByRole("button", { name: /Test Category/ }).getAttribute(
        "aria-expanded",
      ),
    ).toBe("true");
    expect(getByText("Toggle")).toBeDefined();
    expect(queryByText("Enabled, Name")).toBeNull();
  });

  it("summarises long categories with a remainder count", () => {
    const items = ["A", "B", "C", "D", "E"].map((label, idx) => ({
      ...makeCategory().items[0]!,
      key: `test.${label.toLowerCase()}`,
      label,
      sort_order: idx,
    }));
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory({ items })}
        collapsible
        collapsed
        onToggle={vi.fn()}
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("A, B, C und 2 weitere")).toBeDefined();
  });

  it("filterQuery keeps only matching settings and shows the tab label", () => {
    const { getByText, queryByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        filterQuery="name"
        tabLabel="Betrieb"
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Betrieb")).toBeDefined();
    expect(getByText("Name")).toBeDefined();
    expect(queryByText("Enabled")).toBeNull();
  });

  it("filterQuery matching the category name keeps every setting", () => {
    const { getByText } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        filterQuery="test category"
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(getByText("Name")).toBeDefined();
    expect(getByText("Enabled")).toBeDefined();
  });

  it("filterQuery without a hit renders nothing", () => {
    const { container } = renderWithProviders(
      <SettingsCategory
        category={makeCategory()}
        filterQuery="zzz"
        onSave={vi.fn().mockResolvedValue(null)}
        onReset={vi.fn().mockResolvedValue(null)}
      />,
    );
    expect(container.querySelector("h3")).toBeNull();
  });
});
