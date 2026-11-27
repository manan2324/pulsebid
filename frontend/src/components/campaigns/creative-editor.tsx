"use client";
import { useDeferredValue } from "react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { AdPreview } from "./ad-preview";
import { starterCreative, type CreativeValues } from "@/lib/schemas/creative";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function CreativeEditor({
  value,
  onChange,
  prefix = "creative",
  disabled = false,
  errors = {},
}: {
  value: CreativeValues;
  onChange: (value: CreativeValues) => void;
  prefix?: string;
  disabled?: boolean;
  errors?: Partial<Record<keyof CreativeValues, string>>;
}) {
  const markup = useDeferredValue(value.markup);
  const set = (key: keyof CreativeValues, next: string) =>
    onChange({ ...value, [key]: next });
  const field = (
    key: "name" | "width" | "height" | "landing_url",
    label: string,
    type = "text",
  ) => (
    <div className="form-field">
      <label htmlFor={`${prefix}-${key}`}>{label}</label>
      <Input
        id={`${prefix}-${key}`}
        type={type}
        value={value[key]}
        onChange={(event) => set(key, event.target.value)}
        aria-invalid={Boolean(errors[key])}
        aria-describedby={errors[key] ? `${prefix}-${key}-error` : undefined}
      />
      {errors[key] && (
        <p id={`${prefix}-${key}-error`} className="field-error" role="alert">
          {errors[key]}
        </p>
      )}
    </div>
  );
  return (
    <fieldset disabled={disabled} className="creative-editor">
      <div className="creative-fields">
        {field("name", c.creativeName)}
        <label className="form-field">
          {c.format}
          <select
            value={value.format}
            onChange={(event) => set("format", event.target.value)}
          >
            <option value="banner">{c.banner}</option>
            <option value="native">{c.native}</option>
            <option value="video">{c.video}</option>
          </select>
        </label>
        <label className="form-field">
          {c.size}
          <select
            value={
              ["300x250", "728x90", "320x50", "160x600"].includes(
                `${value.width}x${value.height}`,
              )
                ? `${value.width}x${value.height}`
                : "custom"
            }
            onChange={(event) => {
              if (event.target.value === "custom") {
                onChange({ ...value, width: "", height: "" });
              } else {
                const [width, height] = event.target.value.split("x");
                onChange({
                  ...value,
                  width: width ?? "",
                  height: height ?? "",
                });
              }
            }}
          >
            {["300x250", "728x90", "320x50", "160x600"].map((size) => (
              <option key={size}>{size}</option>
            ))}
            <option value="custom">{c.custom}</option>
          </select>
        </label>
        <div className="field-columns">
          {field("width", c.width, "number")}
          {field("height", c.height, "number")}
        </div>
        {field("landing_url", c.landing)}
        <label className="form-field">
          {c.creativeStatus}
          <select
            value={value.status}
            onChange={(event) => set("status", event.target.value)}
          >
            <option value="active">{c.active}</option>
            <option value="paused">{c.paused}</option>
          </select>
        </label>
        <div className="form-field">
          <label htmlFor={`${prefix}-markup`}>{c.markup}</label>
          <Textarea
            id={`${prefix}-markup`}
            rows={7}
            value={value.markup}
            onChange={(event) => set("markup", event.target.value)}
            className="markup-input"
            aria-invalid={Boolean(errors.markup)}
            aria-describedby={`${prefix}-markup-help${errors.markup ? ` ${prefix}-markup-error` : ""}`}
          />
          <small id={`${prefix}-markup-help`}>{c.placeholderHint}</small>
          {errors.markup && (
            <p
              id={`${prefix}-markup-error`}
              className="field-error"
              role="alert"
            >
              {errors.markup}
            </p>
          )}
          <Button
            type="button"
            variant="outline"
            onClick={() => set("markup", starterCreative.markup)}
          >
            {c.starter}
          </Button>
          {(!value.markup.includes("{click_url}") ||
            !value.markup.includes("{impression_url}")) && (
            <p className="notice">{c.missingPlaceholder}</p>
          )}
        </div>
      </div>
      <AdPreview
        markup={markup}
        width={Number(value.width) || 300}
        height={Number(value.height) || 250}
      />
    </fieldset>
  );
}
