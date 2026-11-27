"use client";
import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/data/shared";
import { ApiClientError } from "@/lib/api/client";
import { catalogCopy as c } from "@/lib/copy/catalog";
export function ConfirmAction({
  kind,
  name,
  onConfirm,
  onClose,
}: {
  kind: "archive" | "delete";
  name: string;
  onConfirm: () => Promise<unknown>;
  onClose: () => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {kind === "archive" ? c.archiveTitle : c.deleteTitle}
          </DialogTitle>
          <DialogDescription>
            {kind === "archive" ? c.archiveHelp : c.deleteHelp}
          </DialogDescription>
        </DialogHeader>
        <p className="confirm-record">{name}</p>
        {error && (
          <ErrorState
            message={error.message}
            requestId={
              error instanceof ApiClientError ? error.requestId : undefined
            }
          />
        )}
        <div className="form-actions">
          <Button variant="outline" disabled={pending} onClick={onClose}>
            {c.cancel}
          </Button>
          <Button
            variant="destructive"
            disabled={pending}
            onClick={async () => {
              setPending(true);
              setError(null);
              try {
                await onConfirm();
                onClose();
              } catch (error) {
                setError(
                  error instanceof Error ? error : new Error(c.conflict),
                );
              } finally {
                setPending(false);
              }
            }}
          >
            {pending ? c.confirming : kind === "archive" ? c.archive : c.delete}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
