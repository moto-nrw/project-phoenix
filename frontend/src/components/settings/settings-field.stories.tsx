import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { SettingsField } from "~/components/settings/settings-field";
import type { ResolvedSetting } from "~/lib/settings-api";

function makeSetting(overrides: Partial<ResolvedSetting>): ResolvedSetting {
  return {
    key: "operations.example_setting",
    label: "Beispieleinstellung",
    description: "Beschreibung der Beispieleinstellung.",
    type: "text",
    default: "",
    value: "",
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    validation: null,
    depends_on: null,
    options: null,
    ...overrides,
  };
}

async function noopSave(): Promise<string | null> {
  return null;
}

async function noopReset(): Promise<string | null> {
  return null;
}

const meta = {
  title: "settings/SettingsField",
  component: SettingsField,
  decorators: [
    (Story) => (
      <ToastProvider>
        <div className="max-w-xl">
          <Story />
        </div>
      </ToastProvider>
    ),
  ],
  args: {
    onSave: noopSave,
    onReset: noopReset,
  },
} satisfies Meta<typeof SettingsField>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A simple writable text field with a description. */
export const TextSetting: Story = {
  args: {
    setting: makeSetting({
      key: "operations.text_example",
      label: "Textfeld",
      description: "Ein einfaches Textfeld.",
      type: "text",
      value: "Beispielwert",
      is_default: false,
    }),
  },
};

/** A boolean toggle field. */
export const BooleanSetting: Story = {
  args: {
    setting: makeSetting({
      key: "operations.boolean_example",
      label: "Schalter",
      description: "Ein einfacher Ein/Aus-Schalter.",
      type: "boolean",
      value: true,
      is_default: true,
    }),
  },
};

/** A number field with min/max validation. */
export const NumberSetting: Story = {
  args: {
    setting: makeSetting({
      key: "operations.number_example",
      label: "Zahlenfeld",
      description: "Ein Zahlenfeld mit Grenzwerten.",
      type: "number",
      value: 30,
      is_default: false,
      validation: { min: 0, max: 120 },
    }),
  },
};

/** A read-only field showing the "Nur Lesen" badge. */
export const ReadOnlySetting: Story = {
  args: {
    setting: makeSetting({
      key: "operations.readonly_example",
      label: "Nur lesbare Einstellung",
      description: "Diese Einstellung kann nicht geändert werden.",
      type: "text",
      value: "Fest hinterlegt",
      writable: false,
      is_default: true,
    }),
  },
};
