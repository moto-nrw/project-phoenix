import type { AccentColor } from "./themes";

type AccentClasses = {
  text: string;
  ring: string;
  spinner: string;
  grad: string;
  hoverGrad: string;
  listHover: { title: string; arrow: string; border: string };
};

const base: Record<AccentColor, AccentClasses> = {
  blue: {
    text: "text-blue-600",
    ring: "focus:ring-blue-500",
    spinner: "border-blue-500",
    grad: "from-blue-500 to-blue-600",
    hoverGrad: "hover:from-blue-600 hover:to-blue-700",
    listHover: {
      title: "group-hover:text-blue-600",
      arrow: "group-hover:text-blue-500",
      border: "hover:border-blue-300",
    },
  },
  purple: {
    text: "text-purple-600",
    ring: "focus:ring-purple-500",
    spinner: "border-purple-500",
    grad: "from-purple-500 to-purple-600",
    hoverGrad: "hover:from-purple-600 hover:to-purple-700",
    listHover: {
      title: "group-hover:text-purple-600",
      arrow: "group-hover:text-purple-500",
      border: "hover:border-purple-300",
    },
  },
  green: {
    text: "text-green-600",
    ring: "focus:ring-green-500",
    spinner: "border-green-500",
    grad: "from-green-500 to-green-600",
    hoverGrad: "hover:from-green-600 hover:to-green-700",
    listHover: {
      title: "group-hover:text-green-600",
      arrow: "group-hover:text-green-500",
      border: "hover:border-green-300",
    },
  },
  red: {
    text: "text-red-600",
    ring: "focus:ring-red-500",
    spinner: "border-red-500",
    grad: "from-red-500 to-red-600",
    hoverGrad: "hover:from-red-600 hover:to-red-700",
    listHover: {
      title: "group-hover:text-red-600",
      arrow: "group-hover:text-red-500",
      border: "hover:border-red-300",
    },
  },
  indigo: {
    text: "text-indigo-600",
    ring: "focus:ring-indigo-500",
    spinner: "border-indigo-500",
    grad: "from-indigo-500 to-indigo-600",
    hoverGrad: "hover:from-indigo-600 hover:to-indigo-700",
    listHover: {
      title: "group-hover:text-indigo-600",
      arrow: "group-hover:text-indigo-500",
      border: "hover:border-indigo-300",
    },
  },
  gray: {
    text: "text-gray-600",
    ring: "focus:ring-gray-500",
    spinner: "border-gray-500",
    grad: "from-gray-500 to-gray-600",
    hoverGrad: "hover:from-gray-600 hover:to-gray-700",
    listHover: {
      title: "group-hover:text-gray-700",
      arrow: "group-hover:text-gray-500",
      border: "hover:border-gray-300",
    },
  },
  amber: {
    text: "text-amber-600",
    ring: "focus:ring-amber-500",
    spinner: "border-amber-500",
    grad: "from-amber-500 to-amber-600",
    hoverGrad: "hover:from-amber-600 hover:to-amber-700",
    listHover: {
      title: "group-hover:text-amber-600",
      arrow: "group-hover:text-amber-500",
      border: "hover:border-amber-300",
    },
  },
  orange: {
    text: "text-orange-600",
    ring: "focus:ring-orange-500",
    spinner: "border-orange-500",
    grad: "from-orange-500 to-orange-600",
    hoverGrad: "hover:from-orange-600 hover:to-orange-700",
    listHover: {
      title: "group-hover:text-orange-600",
      arrow: "group-hover:text-orange-500",
      border: "hover:border-orange-300",
    },
  },
  pink: {
    text: "text-pink-600",
    ring: "focus:ring-pink-500",
    spinner: "border-pink-500",
    grad: "from-pink-500 to-rose-600",
    hoverGrad: "hover:from-pink-600 hover:to-rose-700",
    listHover: {
      title: "group-hover:text-pink-600",
      arrow: "group-hover:text-pink-500",
      border: "hover:border-pink-300",
    },
  },
  yellow: {
    text: "text-yellow-600",
    ring: "focus:ring-yellow-500",
    spinner: "border-yellow-500",
    grad: "from-yellow-500 to-yellow-600",
    hoverGrad: "hover:from-yellow-600 hover:to-yellow-700",
    listHover: {
      title: "group-hover:text-yellow-600",
      arrow: "group-hover:text-yellow-500",
      border: "hover:border-yellow-300",
    },
  },
};

export function getAccent(accent: AccentColor): AccentClasses {
  return base[accent] ?? base.indigo;
}

export const getAccentText = (a: AccentColor) => getAccent(a).text;
export const getAccentRing = (a: AccentColor) => getAccent(a).ring;
export const getAccentSpinner = (a: AccentColor) => getAccent(a).spinner;
