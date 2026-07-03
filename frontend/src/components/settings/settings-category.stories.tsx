import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ResolvedSetting, SchemaCategory } from "~/lib/settings-api";
import { SettingsCategory } from "~/components/settings/settings-category";

function makeSetting(
  overrides: Partial<ResolvedSetting> & Pick<ResolvedSetting, "key" | "type">,
): ResolvedSetting {
  return {
    label: overrides.key,
    description: "",
    default: null,
    value: null,
    is_default: true,
    writable: true,
    visible: true,
    sort_order: 0,
    access_policy: "shared",
    ...overrides,
  };
}

const booleanCategory: SchemaCategory = {
  key: "sessions",
  label: "Sitzungen",
  items: [
    makeSetting({
      key: "operations.session_reminder_enabled",
      type: "boolean",
      label: "Erinnerung aktivieren",
      description: "Erinnert Mitarbeitende an das Sitzungsende.",
      value: true,
    }),
    makeSetting({
      key: "operations.session_end_time",
      type: "time",
      label: "Sitzungsende",
      description: "Uhrzeit, zu der Sitzungen enden.",
      value: "16:00",
    }),
  ],
};

const noop = async () => null;

const meta = {
  title: "settings/SettingsCategory",
  component: SettingsCategory,
  args: {
    category: booleanCategory,
    onSave: noop,
    onReset: noop,
  },
} satisfies Meta<typeof SettingsCategory>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A category rendered as it appears on the tenant settings page. */
export const Default: Story = {};

/** A category with no visible items renders nothing. */
export const NoVisibleItems: Story = {
  args: {
    category: {
      key: "hidden",
      label: "Versteckt",
      items: [
        makeSetting({
          key: "operations.hidden_setting",
          type: "boolean",
          visible: false,
        }),
      ],
    },
  },
};

/** Operator audience flips the "auch von {other side} änderbar" hint copy. */
export const OperatorAudience: Story = {
  args: {
    audience: "operator",
  },
};
