"use client";
import { useEffect, useState } from "react";
import { useCreatives, useCampaignMutation } from "@/hooks/use-campaigns";
import type { Creative } from "@/lib/api/types";
import {
  creativeSchema,
  creativePayload,
  starterCreative,
  type CreativeValues,
} from "@/lib/schemas/creative";
import { CreativeEditor } from "./creative-editor";
import { AdPreview } from "./ad-preview";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogHeader,
} from "@/components/ui/dialog";
import { EmptyState, ErrorState } from "@/components/data/shared";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiClientError } from "@/lib/api/client";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
import { campaignCopy as c } from "@/lib/copy/campaign";
function CreativeDialog({
  creative,
  id,
  onClose,
}: {
  creative?: Creative;
  id: string;
  onClose: () => void;
}) {
  const initial: CreativeValues = creative
    ? {
        name: creative.name,
        format: creative.format,
        width: creative.width?.toString() ?? "",
        height: creative.height?.toString() ?? "",
        markup: creative.markup,
        landing_url: creative.landing_url,
        status: creative.status,
      }
    : { ...starterCreative };
  const [value, setValue] = useState(initial);
  const [errors, setErrors] = useState<
    Partial<Record<keyof CreativeValues, string>>
  >({});
  const mutation = useCampaignMutation();
  const dirty = JSON.stringify(initial) !== JSON.stringify(value);
  useDirtyGuard(dirty);
  const close = () => {
    if (!mutation.isPending && (!dirty || window.confirm(c.discard))) onClose();
  };
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <DialogContent className="creative-dialog max-h-[90vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{creative ? c.editCreative : c.addCreative}</DialogTitle>
          <DialogDescription>{c.placeholderHint}</DialogDescription>
        </DialogHeader>
        <CreativeEditor
          value={value}
          onChange={setValue}
          disabled={mutation.isPending}
          errors={errors}
        />
        {mutation.error && (
          <ErrorState
            message={mutation.error.message}
            requestId={
              mutation.error instanceof ApiClientError
                ? mutation.error.requestId
                : undefined
            }
          />
        )}
        <div className="form-actions">
          <Button
            variant="outline"
            disabled={mutation.isPending}
            onClick={close}
          >
            {c.cancel}
          </Button>
          <Button
            disabled={mutation.isPending || (Boolean(creative) && !dirty)}
            onClick={() => {
              const parsed = creativeSchema.safeParse(value);
              if (!parsed.success) {
                const issues: typeof errors = {};
                for (const issue of parsed.error.issues)
                  issues[issue.path[0] as keyof CreativeValues] = issue.message;
                setErrors(issues);
                return;
              }
              setErrors({});
              const body = creativePayload(parsed.data);
              mutation.mutate(
                creative
                  ? { type: "creative-update", id: creative.id, body }
                  : { type: "creative-create", id, body },
                {
                  onSuccess: onClose,
                  onError: (error) => {
                    if (error instanceof ApiClientError) {
                      const issues: typeof errors = {};
                      for (const detail of error.details)
                        if (
                          [
                            "name",
                            "format",
                            "width",
                            "height",
                            "markup",
                            "landing_url",
                            "status",
                          ].includes(detail.field)
                        )
                          issues[detail.field as keyof CreativeValues] =
                            detail.issue;
                      setErrors(issues);
                    }
                  },
                },
              );
            }}
          >
            {mutation.isPending ? c.saving : c.save}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
export function CampaignCreatives({
  id,
  disabled,
}: {
  id: string;
  disabled: boolean;
}) {
  const query = useCreatives(id);
  const mutation = useCampaignMutation();
  const [editor, setEditor] = useState<Creative | "new" | null>(null);
  const [deleting, setDeleting] = useState<Creative | null>(null);
  const [page, setPage] = useState(0);
  useEffect(() => {
    if (query.data && page > 0 && page * 6 >= query.data.length) setPage(0);
  }, [query.data, page]);
  return (
    <section className="panel detail-panel">
      <div className="section-heading">
        <h2>{c.creatives}</h2>
        <Button disabled={disabled} onClick={() => setEditor("new")}>
          {c.addCreative}
        </Button>
      </div>
      {query.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : query.isError ? (
        <ErrorState
          message={query.error.message}
          requestId={
            query.error instanceof ApiClientError
              ? query.error.requestId
              : undefined
          }
          retry={() => {
            void query.refetch();
          }}
        />
      ) : query.data.length === 0 ? (
        <EmptyState title={c.noCreatives} description={c.noCreativesHelp} />
      ) : (
        <>
          <div className="creative-cards">
            {query.data.slice(page * 6, page * 6 + 6).map((creative) => (
              <article key={creative.id} className="creative-card">
                <div className="section-heading">
                  <h3>{creative.name}</h3>
                  <span className={`campaign-status ${creative.status}`}>
                    {c[creative.status]}
                  </span>
                </div>
                <p className="field-help">
                  {creative.format} · {creative.width ?? "—"}×
                  {creative.height ?? "—"}
                </p>
                <AdPreview
                  markup={creative.markup}
                  width={creative.width ?? 300}
                  height={creative.height ?? 250}
                />
                <div className="row-actions">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={disabled}
                    aria-label={`${c.editCreative} ${creative.name}`}
                    onClick={() => setEditor(creative)}
                  >
                    {c.editCreative}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={disabled || mutation.isPending}
                    aria-label={`${creative.status === "active" ? c.pause : c.activate} ${creative.name}`}
                    onClick={() =>
                      mutation.mutate({
                        type: "creative-update",
                        id: creative.id,
                        body: {
                          status:
                            creative.status === "active" ? "paused" : "active",
                        },
                      })
                    }
                  >
                    {creative.status === "active" ? c.pause : c.activate}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={disabled}
                    aria-label={`${c.deleteCreative} ${creative.name}`}
                    onClick={() => setDeleting(creative)}
                  >
                    {c.delete}
                  </Button>
                </div>
              </article>
            ))}
          </div>
          <div className="form-actions">
            <Button
              variant="outline"
              disabled={page === 0}
              onClick={() => setPage(page - 1)}
            >
              {c.back}
            </Button>
            <span>
              {c.step} {page + 1} /{" "}
              {Math.max(1, Math.ceil(query.data.length / 6))}
            </span>
            <Button
              variant="outline"
              disabled={(page + 1) * 6 >= query.data.length}
              onClick={() => setPage(page + 1)}
            >
              {c.next}
            </Button>
          </div>
        </>
      )}
      {editor && (
        <CreativeDialog
          key={editor === "new" ? "new" : editor.id}
          id={id}
          creative={editor === "new" ? undefined : editor}
          onClose={() => setEditor(null)}
        />
      )}
      {deleting && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open && !mutation.isPending) setDeleting(null);
          }}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{c.deleteCreative}</DialogTitle>
              <DialogDescription>{c.deleteCreativeHelp}</DialogDescription>
            </DialogHeader>
            <strong>{deleting.name}</strong>
            {mutation.error && (
              <ErrorState
                message={mutation.error.message}
                requestId={
                  mutation.error instanceof ApiClientError
                    ? mutation.error.requestId
                    : undefined
                }
              />
            )}
            <div className="form-actions">
              <Button
                variant="outline"
                disabled={mutation.isPending}
                onClick={() => setDeleting(null)}
              >
                {c.cancel}
              </Button>
              <Button
                variant="destructive"
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate(
                    { type: "creative-delete", id: deleting.id },
                    { onSuccess: () => setDeleting(null) },
                  )
                }
              >
                {c.delete}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      )}
    </section>
  );
}
