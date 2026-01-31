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
  variant?: "default" | "line";
}) {
  return (
    <TabsPrimitive.List
      data-variant={variant}
      className={cn(
        "group/tabs-list text-muted-foreground inline-flex items-center justify-center",
        variant === "default" && "bg-muted h-9 rounded-lg p-1",
        variant === "line" &&
          "h-auto gap-6 rounded-none border-b border-gray-200 bg-transparent p-0",
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
        "ring-offset-background focus-visible:ring-ring inline-flex items-center justify-center text-sm font-medium whitespace-nowrap transition-all focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
        // Default variant (pill)
        "group-data-[variant=default]/tabs-list:data-[state=active]:bg-background group-data-[variant=default]/tabs-list:data-[state=active]:text-foreground group-data-[variant=default]/tabs-list:rounded-md group-data-[variant=default]/tabs-list:px-3 group-data-[variant=default]/tabs-list:py-1 group-data-[variant=default]/tabs-list:data-[state=active]:shadow",
        // Line variant (underline)
        "group-data-[variant=line]/tabs-list:relative group-data-[variant=line]/tabs-list:bg-transparent group-data-[variant=line]/tabs-list:pb-3 group-data-[variant=line]/tabs-list:text-gray-500 group-data-[variant=line]/tabs-list:shadow-none group-data-[variant=line]/tabs-list:after:absolute group-data-[variant=line]/tabs-list:after:inset-x-0 group-data-[variant=line]/tabs-list:after:bottom-0 group-data-[variant=line]/tabs-list:after:h-0.5 group-data-[variant=line]/tabs-list:after:rounded-full group-data-[variant=line]/tabs-list:hover:text-gray-700 group-data-[variant=line]/tabs-list:data-[state=active]:font-semibold group-data-[variant=line]/tabs-list:data-[state=active]:text-gray-900 group-data-[variant=line]/tabs-list:data-[state=active]:after:bg-gray-900",
        className,
      )}
      {...props}
    />
  );
}
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName;

export { Tabs, TabsList, TabsTrigger };
