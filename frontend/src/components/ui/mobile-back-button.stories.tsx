import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useEffect } from "react";

import { MobileBackButton } from "./mobile-back-button";

/**
 * `useIsMobile` reads `globalThis.innerWidth` on mount and on resize.
 * This wrapper forces a narrow width before the component mounts so the
 * story renders the mobile-only button in Storybook's desktop preview iframe.
 */
function ForceMobileViewport({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  useEffect(() => {
    Object.defineProperty(globalThis, "innerWidth", {
      writable: true,
      configurable: true,
      value: 375,
    });
    globalThis.dispatchEvent(new Event("resize"));
  }, []);

  return <>{children}</>;
}

const meta: Meta<typeof MobileBackButton> = {
  title: "components/ui/MobileBackButton",
  component: MobileBackButton,
  parameters: {
    viewport: {
      defaultViewport: "mobile1",
    },
  },
  decorators: [
    (Story) => (
      <ForceMobileViewport>
        <Story />
      </ForceMobileViewport>
    ),
  ],
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
