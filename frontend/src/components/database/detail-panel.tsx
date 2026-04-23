"use client";

import type { ReactNode } from "react";
import { cn } from "~/lib/utils";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";

export interface DetailTab {
  id: string;
  label: string;
  content: ReactNode;
  disabled?: boolean;
}

interface DetailPanelProps {
  header: ReactNode;
  tabs: DetailTab[];
  activeTab: string;
  onTabChange: (id: string) => void;
  className?: string;
}

export function DetailPanel({
  header,
  tabs,
  activeTab,
  onTabChange,
  className,
}: DetailPanelProps) {
  return (
    <div className={cn("flex h-full flex-col", className)}>
      <div className="border-b border-gray-200">{header}</div>
      <Tabs
        value={activeTab}
        onValueChange={onTabChange}
        className="flex min-h-0 flex-1 flex-col"
      >
        <TabsList
          variant="line"
          className="justify-start border-b border-gray-200 px-6 pt-4"
        >
          {tabs.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id} disabled={tab.disabled}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <div className="min-h-0 flex-1 overflow-auto">
          {tabs.map((tab) => (
            <TabsContent
              key={tab.id}
              value={tab.id}
              className="mt-0 p-6 focus-visible:ring-0 focus-visible:ring-offset-0"
            >
              {tab.content}
            </TabsContent>
          ))}
        </div>
      </Tabs>
    </div>
  );
}
