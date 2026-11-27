"use client";
import { useForm, type FieldErrors } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { Advertiser, Bidder } from "@/lib/api/types";
import {
  advertiserSchema,
  type AdvertiserFormValues,
} from "@/lib/schemas/advertiser";
import { bidderSchema, type BidderFormValues } from "@/lib/schemas/bidder";
import { applyApiErrors } from "@/lib/api/apply-api-errors";
import type { AdvertiserCreate, AdvertiserPatch } from "@/lib/api/advertisers";
import type { BidderCreate, BidderPatch } from "@/lib/api/bidders";
import { catalogCopy as c } from "@/lib/copy/catalog";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
export function FieldError({ id, message }: { id: string; message?: string }) {
  return message ? (
    <p id={id} className="field-error" role="alert">
      {message}
    </p>
  ) : null;
}
function FormServerError({
  errors,
}: {
  errors: FieldErrors<AdvertiserFormValues> | FieldErrors<BidderFormValues>;
}) {
  return (
    <FieldError id="form-server-error" message={errors.root?.server?.message} />
  );
}
function closeGuard(dirty: boolean, pending: boolean, onClose: () => void) {
  if (!pending && (!dirty || window.confirm(c.discardChanges))) onClose();
}
export function AdvertiserForm({
  advertiser,
  onSave,
  onClose,
}: {
  advertiser?: Advertiser;
  onSave: (body: AdvertiserCreate | AdvertiserPatch) => Promise<unknown>;
  onClose: () => void;
}) {
  const form = useForm<AdvertiserFormValues>({
    resolver: zodResolver(advertiserSchema),
    defaultValues: {
      name: advertiser?.name ?? "",
      contact_email: advertiser?.contact_email ?? "",
    },
  });
  const {
    register,
    handleSubmit,
    formState: { errors, isDirty, isSubmitting, dirtyFields },
  } = form;
  useDirtyGuard(isDirty);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) closeGuard(isDirty, isSubmitting, onClose);
      }}
    >
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {advertiser ? c.editAdvertiser : c.newAdvertiser}
          </DialogTitle>
          <DialogDescription>{c.advertiserFormHelp}</DialogDescription>
        </DialogHeader>
        <form
          noValidate
          className="catalog-form"
          onSubmit={handleSubmit(async (values) => {
            form.clearErrors("root");
            const body: AdvertiserCreate | AdvertiserPatch = advertiser
              ? {
                  ...(dirtyFields.name ? { name: values.name } : {}),
                  ...(dirtyFields.contact_email
                    ? { contact_email: values.contact_email || null }
                    : {}),
                }
              : {
                  name: values.name,
                  contact_email: values.contact_email || null,
                };
            try {
              await onSave(body);
              onClose();
            } catch (error) {
              applyApiErrors(form, error, ["name", "contact_email"]);
            }
          })}
        >
          <div className="form-field">
            <label htmlFor="advertiser-name">{c.name}</label>
            <Input
              id="advertiser-name"
              {...register("name")}
              placeholder={c.advertiserNamePlaceholder}
              aria-invalid={Boolean(errors.name)}
              aria-describedby={
                errors.name ? "advertiser-name-error" : undefined
              }
            />
            <FieldError
              id="advertiser-name-error"
              message={errors.name?.message}
            />
          </div>
          <div className="form-field">
            <label htmlFor="advertiser-email">{c.email}</label>
            <Input
              id="advertiser-email"
              type="email"
              {...register("contact_email")}
              placeholder={c.emailPlaceholder}
              aria-invalid={Boolean(errors.contact_email)}
              aria-describedby={`advertiser-email-help${errors.contact_email ? " advertiser-email-error" : ""}`}
            />
            <small id="advertiser-email-help">{c.emailOptional}</small>
            <FieldError
              id="advertiser-email-error"
              message={errors.contact_email?.message}
            />
          </div>
          <FormServerError errors={errors} />
          <div className="form-actions">
            <Button
              type="button"
              variant="outline"
              disabled={isSubmitting}
              onClick={() => closeGuard(isDirty, isSubmitting, onClose)}
            >
              {c.cancel}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || Boolean(advertiser && !isDirty)}
            >
              {isSubmitting
                ? c.saving
                : advertiser
                  ? c.save
                  : c.createAdvertiser}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
export function BidderForm({
  bidder,
  onSave,
  onClose,
}: {
  bidder?: Bidder;
  onSave: (body: BidderCreate | BidderPatch) => Promise<unknown>;
  onClose: () => void;
}) {
  const form = useForm<BidderFormValues>({
    resolver: zodResolver(bidderSchema),
    defaultValues: {
      id: bidder?.id ?? "",
      name: bidder?.name ?? "",
      endpoint_url: bidder?.endpoint_url ?? "",
      timeout_ms: bidder?.timeout_ms?.toString() ?? "",
      enabled: bidder?.enabled ?? true,
    },
  });
  const {
    register,
    handleSubmit,
    formState: { errors, isDirty, isSubmitting, dirtyFields },
  } = form;
  useDirtyGuard(isDirty);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) closeGuard(isDirty, isSubmitting, onClose);
      }}
    >
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{bidder ? c.editBidder : c.newBidder}</DialogTitle>
          <DialogDescription>{c.bidderFormHelp}</DialogDescription>
        </DialogHeader>
        <form
          noValidate
          className="catalog-form"
          onSubmit={handleSubmit(async (values) => {
            form.clearErrors("root");
            const payload = {
              name: values.name,
              endpoint_url: values.endpoint_url,
              timeout_ms:
                values.timeout_ms === "" ? null : Number(values.timeout_ms),
              enabled: values.enabled,
            };
            const body: BidderCreate | BidderPatch = bidder
              ? {
                  ...(dirtyFields.name ? { name: payload.name } : {}),
                  ...(dirtyFields.endpoint_url
                    ? { endpoint_url: payload.endpoint_url }
                    : {}),
                  ...(dirtyFields.timeout_ms
                    ? { timeout_ms: payload.timeout_ms }
                    : {}),
                  ...(dirtyFields.enabled ? { enabled: payload.enabled } : {}),
                }
              : { id: values.id, ...payload };
            try {
              await onSave(body);
              onClose();
            } catch (error) {
              applyApiErrors(form, error, [
                "id",
                "name",
                "endpoint_url",
                "timeout_ms",
                "enabled",
              ]);
            }
          })}
        >
          <div className="form-field">
            <label htmlFor="bidder-id">{c.id}</label>
            <Input
              id="bidder-id"
              {...register("id")}
              readOnly={Boolean(bidder)}
              placeholder={c.bidderIdPlaceholder}
              aria-invalid={Boolean(errors.id)}
              aria-describedby={`bidder-id-help${errors.id ? " bidder-id-error" : ""}`}
            />
            <small id="bidder-id-help">
              {bidder ? c.idImmutable : c.idHint}
            </small>
            <FieldError id="bidder-id-error" message={errors.id?.message} />
          </div>
          <div className="form-field">
            <label htmlFor="bidder-name">{c.name}</label>
            <Input
              id="bidder-name"
              {...register("name")}
              placeholder={c.bidderNamePlaceholder}
              aria-invalid={Boolean(errors.name)}
              aria-describedby={errors.name ? "bidder-name-error" : undefined}
            />
            <FieldError id="bidder-name-error" message={errors.name?.message} />
          </div>
          <div className="form-field">
            <label htmlFor="bidder-endpoint">{c.endpoint}</label>
            <Input
              id="bidder-endpoint"
              {...register("endpoint_url")}
              placeholder={c.endpointPlaceholder}
              aria-invalid={Boolean(errors.endpoint_url)}
              aria-describedby={`bidder-endpoint-help${errors.endpoint_url ? " bidder-endpoint-error" : ""}`}
            />
            <small id="bidder-endpoint-help">{c.endpointHint}</small>
            <FieldError
              id="bidder-endpoint-error"
              message={errors.endpoint_url?.message}
            />
          </div>
          <div className="form-field">
            <label htmlFor="bidder-timeout">{c.timeout}</label>
            <Input
              id="bidder-timeout"
              type="number"
              min={10}
              max={1000}
              step={1}
              {...register("timeout_ms")}
              placeholder={c.defaultTimeout}
              aria-invalid={Boolean(errors.timeout_ms)}
              aria-describedby={`bidder-timeout-help${errors.timeout_ms ? " bidder-timeout-error" : ""}`}
            />
            <small id="bidder-timeout-help">{c.timeoutHint}</small>
            <FieldError
              id="bidder-timeout-error"
              message={errors.timeout_ms?.message}
            />
          </div>
          <label className="checkbox-label" htmlFor="bidder-enabled">
            <input
              id="bidder-enabled"
              type="checkbox"
              {...register("enabled")}
            />
            {c.enabled}
          </label>
          <FormServerError errors={errors} />
          <div className="form-actions">
            <Button
              type="button"
              variant="outline"
              disabled={isSubmitting}
              onClick={() => closeGuard(isDirty, isSubmitting, onClose)}
            >
              {c.cancel}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || Boolean(bidder && !isDirty)}
            >
              {isSubmitting ? c.saving : bidder ? c.save : c.createBidder}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
