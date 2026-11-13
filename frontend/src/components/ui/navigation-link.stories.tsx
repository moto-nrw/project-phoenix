import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import NavigationLink from "./navigation-link";

const meta = {
  title: "components/ui/NavigationLink",
  component: NavigationLink,
  parameters: {
    layout: "padded",
  },
  args: {
    href: "/rooms",
    children: "Zu den Räumen",
  },
} satisfies Meta<typeof NavigationLink>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Ohne eigene Klassen: der Link sieht aus wie ein `<a>` der Seite. */
export const Default: Story = {};

/** Mit eigenen Klassen, wie im Fließtext einer Seite. */
export const InText: Story = {
  render: (args) => (
    <p className="text-sm text-gray-700">
      Die Belegung steht in der{" "}
      <NavigationLink
        {...args}
        className="font-medium text-blue-700 underline underline-offset-2"
      >
        Raumübersicht
      </NavigationLink>
      .
    </p>
  ),
};
