"use client";
import { useMemo } from "react";
import { previewDocument } from "@/lib/ad-preview";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function AdPreview({
  markup,
  width = 300,
  height = 250,
}: {
  markup: string;
  width?: number;
  height?: number;
}) {
  const srcDoc = useMemo(() => previewDocument(markup), [markup]);
  return (
    <figure className="ad-preview">
      <figcaption>{c.preview}</figcaption>
      <div className="preview-scroll">
        <iframe
          title={c.previewTitle}
          sandbox=""
          referrerPolicy="no-referrer"
          srcDoc={srcDoc}
          width={width}
          height={height}
        />
      </div>
      <small>{c.previewHelp}</small>
    </figure>
  );
}
