"use client";

// Das gespeicherte Original eines Jahresbescheids als PDF im Seitenpanel
// (#6347). Es ist dieselbe Datei, die die Familie per E-Mail und in der
// Eltern-App bekommt.

import {
  SlideOver,
  SlideOverBody,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import {
  type CareNoticeMailing,
  downloadCareNoticePDF,
} from "~/lib/traeger-care-notice-letters-api";
import { NOTICE_KIND_LABELS } from "./care-notice-format";
import { formatEuro } from "./contribution-format";
import { LetterFileView } from "./letter-file-view";

export function CareNoticeLetterPanel({
  runId,
  notice,
  onClose,
}: Readonly<{
  runId: string;
  /** The notice to show; null closes the panel. */
  notice: CareNoticeMailing | null;
  onClose: () => void;
}>) {
  const title = notice
    ? `${NOTICE_KIND_LABELS[notice.kind]} ${notice.number}`
    : "";
  return (
    <SlideOver
      open={notice !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      {notice ? (
        <SlideOverContent size="lg">
          <SlideOverHeader>
            <SlideOverTitle>{title}</SlideOverTitle>
            <SlideOverDescription className="wrap-anywhere">
              An {notice.recipientFirstName} {notice.recipientLastName} · Für{" "}
              {notice.childFirstName} {notice.childLastName} ·{" "}
              <span className="tabular-nums">
                {formatEuro(notice.totalCents)}
              </span>
            </SlideOverDescription>
          </SlideOverHeader>
          <SlideOverBody>
            <LetterFileView
              fileKey={`notice-${notice.noticeId}`}
              load={() => downloadCareNoticePDF(runId, notice.contractId)}
              title={title}
            />
          </SlideOverBody>
        </SlideOverContent>
      ) : null}
    </SlideOver>
  );
}
