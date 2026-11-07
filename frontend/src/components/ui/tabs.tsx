"use client";

import * as React from "react";
import * as TabsPrimitive from "@radix-ui/react-tabs";

import { cn } from "~/lib/utils";

const Tabs = TabsPrimitive.Root;

function TabsList({
  className,
  variant = "default",
  ...props
}: React.ComponentPropsWithoutRef<typeof TabsPrimitive.List> & {
  /** `icon`: volle Breite, Symbol über dem Text, ab `sm` daneben. */
  variant?: "default" | "line" | "icon";
}) {
  return (
    <TabsPrimitive.List
      data-variant={variant}
      className={cn(
        "group/tabs-list inline-flex items-center justify-center text-gray-500",
        variant === "default" && "h-9 rounded-lg bg-gray-100 p-1",
        variant === "line" &&
          "h-auto gap-6 rounded-none border-b border-gray-200 bg-transparent p-0",
        variant === "icon" &&
          "flex h-auto w-full gap-1 rounded-xl bg-gray-100 p-1",
        className,
      )}
      {...props}
    />
  );
}
TabsList.displayName = TabsPrimitive.List.displayName;

function TabsTrigger({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      className={cn(
        // Base styles
        "focus-visible:ring-moto-blue inline-flex items-center justify-center gap-1.5 text-sm font-medium whitespace-nowrap ring-offset-white transition-all focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
        // Default variant (pill)
        "group-data-[variant=default]/tabs-list:rounded-md group-data-[variant=default]/tabs-list:px-3 group-data-[variant=default]/tabs-list:py-1 group-data-[variant=default]/tabs-list:data-[state=active]:bg-white group-data-[variant=default]/tabs-list:data-[state=active]:text-gray-900 group-data-[variant=default]/tabs-list:data-[state=active]:shadow",
        // Line variant (underline)
        "group-data-[variant=line]/tabs-list:relative group-data-[variant=line]/tabs-list:bg-transparent group-data-[variant=line]/tabs-list:pb-3 group-data-[variant=line]/tabs-list:text-gray-500 group-data-[variant=line]/tabs-list:shadow-none group-data-[variant=line]/tabs-list:after:absolute group-data-[variant=line]/tabs-list:after:inset-x-0 group-data-[variant=line]/tabs-list:after:bottom-0 group-data-[variant=line]/tabs-list:after:h-0.5 group-data-[variant=line]/tabs-list:after:rounded-full group-data-[variant=line]/tabs-list:hover:text-gray-700 group-data-[variant=line]/tabs-list:data-[state=active]:font-semibold group-data-[variant=line]/tabs-list:data-[state=active]:text-gray-900 group-data-[variant=line]/tabs-list:data-[state=active]:after:bg-gray-900",
        // Icon variant (symbol above the label, side by side from `sm`)
        "group-data-[variant=icon]/tabs-list:min-h-16 group-data-[variant=icon]/tabs-list:min-w-0 group-data-[variant=icon]/tabs-list:flex-1 group-data-[variant=icon]/tabs-list:flex-col group-data-[variant=icon]/tabs-list:gap-1 group-data-[variant=icon]/tabs-list:rounded-lg group-data-[variant=icon]/tabs-list:px-1.5 group-data-[variant=icon]/tabs-list:py-2 group-data-[variant=icon]/tabs-list:text-base group-data-[variant=icon]/tabs-list:text-gray-600 group-data-[variant=icon]/tabs-list:hover:bg-white/60 group-data-[variant=icon]/tabs-list:hover:text-gray-900 group-data-[variant=icon]/tabs-list:data-[state=active]:bg-white group-data-[variant=icon]/tabs-list:data-[state=active]:font-semibold group-data-[variant=icon]/tabs-list:data-[state=active]:text-gray-900 group-data-[variant=icon]/tabs-list:data-[state=active]:shadow-sm group-data-[variant=icon]/tabs-list:sm:min-h-12 group-data-[variant=icon]/tabs-list:sm:flex-row group-data-[variant=icon]/tabs-list:sm:gap-2 group-data-[variant=icon]/tabs-list:sm:px-4",
        className,
      )}
      {...props}
    />
  );
}
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName;

function TabsContent({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      className={cn(
        "focus-visible:ring-moto-blue mt-2 ring-offset-white focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none",
        className,
      )}
      {...props}
    />
  );
}
TabsContent.displayName = TabsPrimitive.Content.displayName;

export { Tabs, TabsList, TabsTrigger, TabsContent };
