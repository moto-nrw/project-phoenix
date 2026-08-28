import { Suspense } from "react";
import { ParentNewsPage } from "~/components/parent/news/parent-news-page";

export default function ParentsNewsPage() {
  return (
    <Suspense>
      <ParentNewsPage />
    </Suspense>
  );
}
