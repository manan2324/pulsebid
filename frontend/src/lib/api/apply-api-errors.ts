import type { FieldValues, Path, UseFormReturn } from "react-hook-form";
import { ApiClientError } from "./client";
import { catalogCopy as c } from "@/lib/copy/catalog";
export function applyApiErrors<T extends FieldValues>(
  form: Pick<UseFormReturn<T>, "setError">,
  error: unknown,
  fields: readonly Path<T>[],
) {
  if (!(error instanceof ApiClientError)) {
    form.setError("root.server", {
      message: error instanceof Error ? error.message : c.fieldError,
    });
    return;
  }
  const permitted = new Set<string>(fields);
  let focused = false;
  for (const detail of error.details) {
    if (!permitted.has(detail.field)) continue;
    form.setError(
      detail.field as Path<T>,
      { type: "server", message: detail.issue },
      { shouldFocus: !focused },
    );
    focused = true;
  }
  form.setError("root.server", {
    message: `${error.message} (${c.requestId}: ${error.requestId})`,
  });
}
