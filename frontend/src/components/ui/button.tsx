"use client";

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
    isLoading?: boolean;
    loadingText?: string;
    variant?: "primary" | "secondary" | "outline" | "outline_danger" | "danger";  // Added danger variant
    size?: "sm" | "base" | "lg" | "xl";
}

export function Button({
                           children,
                           isLoading,
                           loadingText,
                           variant = "primary",
                           size = "base",
                           className = "",
                           ...props
                       }: ButtonProps) {
    // Text sizes basierend auf size prop
    const textSizes = {
        sm: "text-sm",    // 14px
        base: "text-base", // 16px
        lg: "text-lg",    // 18px
        xl: "text-xl",    // 20px
    };

    // Base styles ohne text-sm
    const baseStyles =
        "flex w-full justify-center rounded-lg px-5 py-3 font-medium shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 transition-all duration-200";

    // Variant-specific styles
    const variantStyles = {
        primary:
            "bg-gradient-to-r from-teal-500 to-blue-500 text-white hover:from-teal-600 hover:to-blue-600 hover:scale-[1.02] hover:shadow-lg focus:ring-teal-500",
        secondary:
            "bg-gray-200 text-gray-800 hover:bg-gray-300 hover:scale-[1.02] hover:shadow-md focus:ring-gray-500",
        outline:
            "bg-transparent text-teal-600 ring-1 ring-teal-500 hover:bg-teal-50 hover:scale-[1.02] hover:shadow-sm focus:ring-teal-500",
        outline_danger:
            "bg-red-50 text-[#FF3130] ring-1 ring-[#FF3130] hover:bg-red-100 hover:scale-[1.02] hover:shadow-sm focus:ring-[#FF3130]",
        danger:
            "bg-gradient-to-r from-[#FF3130] to-[#FF5050] text-white hover:from-[#FF1515] hover:to-[#FF3535] hover:scale-[1.02] hover:shadow-lg focus:ring-red-500",
    };

    return (
        <button
            type="submit"
            disabled={isLoading}
            className={`${baseStyles} ${variantStyles[variant]} ${textSizes[size]} ${className}`}
            {...props}
        >
            {isLoading ? (loadingText ?? "Loading...") : children}
        </button>
    );
}