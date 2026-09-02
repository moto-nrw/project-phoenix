import type { SVGProps } from "react";

// TypeScript 6 does not retain SVG attributes from the package's conditional
// component-props base type. The icon components accept standard SVG props.
declare module "@phosphor-icons/react" {
  interface IconProps extends SVGProps<SVGSVGElement> {}
}

declare module "@phosphor-icons/react/dist/lib/types" {
  interface IconProps extends SVGProps<SVGSVGElement> {}
}
