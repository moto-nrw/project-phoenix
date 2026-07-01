import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { MobileBackButton } from "./mobile-back-button";

const meta: Meta<typeof MobileBackButton> = {
  title: "components/ui/MobileBackButton",
  component: MobileBackButton,
  parameters: {
    viewport: {
      defaultViewport: "mobile1",
    },
  },
  beforeEach: () => {
    const originalDescriptor = Object.getOwnPropertyDescriptor(
      globalThis,
      "innerWidth",
    );

    Object.defineProperty(globalThis, "innerWidth", {
      writable: true,
      configurable: true,
      value: 375,
    });
    globalThis.dispatchEvent(new Event("resize"));

    return () => {
      if (originalDescriptor) {
        Object.defineProperty(globalThis, "innerWidth", originalDescriptor);
      } else {
        Reflect.deleteProperty(globalThis, "innerWidth");
      }
      globalThis.dispatchEvent(new Event("resize"));
    };
  },
};

export default meta;

type Story = StoryObj<typeof MobileBackButton>;

export const Default: Story = {
  args: {},
};

export const CustomDestination: Story = {
  args: {
    href: "/students",
    ariaLabel: "Zurück zur Schülerliste",
  },
};
