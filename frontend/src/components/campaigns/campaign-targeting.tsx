"use client";
import { useEffect, useState } from "react";
import { useTargeting, useCampaignMutation } from "@/hooks/use-campaigns";
import { targetingSchema, type TargetingValues } from "@/lib/schemas/targeting";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
import { TargetingEditor } from "./targeting-editor";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/data/shared";
import { Button } from "@/components/ui/button";
import { ApiClientError } from "@/lib/api/client";
import { campaignCopy as c } from "@/lib/copy/campaign";
function Editor({
  initial,
  id,
  disabled,
  onDirty,
}: {
  initial: TargetingValues;
  id: string;
  disabled: boolean;
  onDirty: (dirty: boolean) => void;
}) {
  const [value, setValue] = useState(initial);
  const [error, setError] = useState("");
  const mutation = useCampaignMutation();
  const [saved, setSaved] = useState(initial);
  const dirty = JSON.stringify(saved) !== JSON.stringify(value);
  useDirtyGuard(dirty);
  useEffect(() => {
    onDirty(dirty);
    return () => onDirty(false);
  }, [dirty, onDirty]);
  return (
    <>
      <TargetingEditor
        value={value}
        onChange={setValue}
        disabled={disabled || mutation.isPending}
      />
      {dirty && <p className="notice">{c.unsaved}</p>}
      {error && (
        <p role="alert" className="field-error">
          {error}
        </p>
      )}
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
      <Button
        disabled={!dirty || disabled || mutation.isPending}
        onClick={() => {
          const parsed = targetingSchema.safeParse(value);
          if (!parsed.success) {
            setError(
              parsed.error.issues.map((issue) => issue.message).join("; "),
            );
            return;
          }
          setError("");
          mutation.mutate(
            { type: "targeting", id, body: parsed.data },
            {
              onSuccess: () => {
                setSaved(parsed.data);
                onDirty(false);
              },
            },
          );
        }}
      >
        {mutation.isPending ? c.saving : c.save}
      </Button>
    </>
  );
}
export function CampaignTargeting({
  id,
  disabled,
  onDirty,
}: {
  id: string;
  disabled: boolean;
  onDirty: (dirty: boolean) => void;
}) {
  const query = useTargeting(id);
  return (
    <section className="panel detail-panel">
      {query.isPending ? (
        <Skeleton className="h-96 w-full" />
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
      ) : (
        <Editor
          key={id}
          id={id}
          initial={{
            countries: query.data.countries,
            device_types: query.data.device_types,
            placement_ids: query.data.placement_ids,
            hours_of_day: query.data.hours_of_day,
            timezone: query.data.timezone,
          }}
          disabled={disabled}
          onDirty={onDirty}
        />
      )}
    </section>
  );
}
