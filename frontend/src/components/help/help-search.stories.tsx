import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { HelpHashScroll, HelpSearchInline } from "./help-search";

const meta: Meta = {
  title: "components/help/HelpSearch",
};

export default meta;

/**
 * The full-width inline help search box, as used on the help landing page and
 * at the top of every guide page. Type at least two characters to see live
 * fuzzy results from the real guide index.
 */
export const Inline: StoryObj<typeof HelpSearchInline> = {
  render: () => <HelpSearchInline />,
};

/**
 * `HelpHashScroll` renders nothing — it only wires up hash-scroll-correction
 * listeners as a side effect. Shown here for completeness; there is no visual
 * output to observe.
 */
export const HashScroll: StoryObj<typeof HelpHashScroll> = {
  render: () => <HelpHashScroll />,
};

/**
 * How the two are actually composed on a guide page (see
 * `guide-components.tsx`): `HelpHashScroll` mounted once alongside the inline
 * search box.
 */
export const Composed: StoryObj = {
  render: () => (
    <>
      <HelpHashScroll />
      <HelpSearchInline />
    </>
  ),
};
