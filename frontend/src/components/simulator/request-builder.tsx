"use client";
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import { z } from "zod";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { CopyButton } from "@/components/data/shared";
import {
  bidSchema,
  presetRequest,
  fieldsToRequest,
  requestToFields,
  parseBidJson,
  type BidFields,
  type BidRequest,
} from "@/lib/schemas/bid";
import { buildCurl } from "@/lib/simulator";
import type { BurstConfig } from "@/lib/burst";
import { simulatorCopy as c } from "@/lib/copy/simulator";

const BurstControls = dynamic(() =>
  import("./burst-controls").then((module) => module.BurstControls),
);

const fieldPaths: Record<keyof BidFields, string> = {
  id: "id",
  impression: "imp.id",
  placement: "imp.placement_id",
  width: "imp.w",
  height: "imp.h",
  floor: "imp.bidfloor",
  currency: "imp.bidfloorcur",
  device: "device.type",
  country: "geo.country",
  timeout: "tmax",
  os: "device.os",
  ua: "device.ua",
  region: "geo.region",
  city: "geo.city",
  siteId: "site.id",
  domain: "site.domain",
  userId: "user.id",
};

export function RequestBuilder({
  initial,
  testMode,
  gatewayUrl,
  sending,
  running,
  onSend,
  onBurst,
  onCancel,
  initialTime = "",
  serverErrors = [],
}: {
  initial?: BidRequest;
  testMode: boolean;
  gatewayUrl: string;
  sending: boolean;
  running: boolean;
  initialTime?: string;
  serverErrors?: { field: string; issue: string }[];
  onSend: (request: BidRequest, testTime?: string) => void;
  onBurst: (
    request: BidRequest,
    config: BurstConfig,
    testTime?: string,
  ) => void;
  onCancel: () => void;
}) {
  const first = initial ?? presetRequest("mobile");
  const [fields, setFields] = useState(() => requestToFields(first));
  const [json, setJson] = useState(() => JSON.stringify(first, null, 2));
  const [mode, setMode] = useState<"form" | "json">("form");
  const [preset, setPreset] = useState(initial ? "custom" : "mobile");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [testTime, setTestTime] = useState(testMode ? initialTime : "");
  const [showBurst, setShowBurst] = useState(false);
  const [curl, setCurl] = useState("");
  const [hydrated, setHydrated] = useState(false);
  useEffect(() => setHydrated(true), []);
  const busy = !hydrated || sending || running;
  const error = (path: string) =>
    errors[path] ?? serverErrors.find((item) => item.field === path)?.issue;
  const assign = (request: BidRequest) => {
    setFields(requestToFields(request));
    setJson(JSON.stringify(request, null, 2));
    setErrors({});
    setCurl("");
  };
  const validate = () => {
    try {
      const request =
        mode === "json"
          ? parseBidJson(json)
          : bidSchema.parse(fieldsToRequest(fields));
      if (
        testMode &&
        testTime &&
        !z.iso.datetime({ offset: true }).safeParse(testTime).success
      ) {
        setErrors({ testTime: c.testTimeError });
        return null;
      }
      setErrors({});
      return request;
    } catch (caught) {
      const mapped: Record<string, string> = {};
      if (caught instanceof z.ZodError) {
        for (const issue of caught.issues)
          mapped[issue.path.join(".") || "json"] = issue.message;
      } else
        mapped.json =
          caught instanceof Error ? caught.message : c.protocolError;
      setErrors(mapped);
      return null;
    }
  };
  const update = (key: keyof BidFields, value: string) => {
    const next = { ...fields, [key]: value };
    setFields(next);
    setJson(JSON.stringify(fieldsToRequest(next), null, 2));
    setPreset("custom");
    setCurl("");
  };
  const input = (key: keyof BidFields, type = "text") => (
    <div className="form-field" key={key}>
      <label htmlFor={`sim-${key}`}>{c.fields[key]}</label>
      <Input
        id={`sim-${key}`}
        type={type}
        value={fields[key]}
        onChange={(event) => update(key, event.target.value)}
        aria-invalid={Boolean(error(fieldPaths[key]))}
        aria-describedby={
          error(fieldPaths[key]) ? `sim-${key}-error` : undefined
        }
      />
      {error(fieldPaths[key]) && (
        <p id={`sim-${key}-error`} className="field-error" role="alert">
          {error(fieldPaths[key])}
        </p>
      )}
    </div>
  );
  return (
    <section className="panel simulator-builder">
      <h2>{c.builder}</h2>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const request = validate();
          if (request)
            onSend(request, testMode ? testTime || undefined : undefined);
        }}
        noValidate
      >
        <fieldset disabled={busy}>
          <label className="form-field">
            {c.preset}
            <select
              aria-label={c.preset}
              value={preset}
              onChange={(event) => {
                setPreset(event.target.value);
                assign(
                  presetRequest(
                    event.target.value,
                    `sim-${crypto.randomUUID()}`,
                  ),
                );
              }}
            >
              <option value="custom" disabled>
                {c.customRequest}
              </option>
              {Object.entries(c.presets).map(([key, label]) => (
                <option key={key} value={key}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <div className="simulator-mode" role="group" aria-label={c.builder}>
            <Button
              type="button"
              variant={mode === "form" ? "default" : "outline"}
              aria-pressed={mode === "form"}
              onClick={() => {
                if (mode === "json") {
                  const request = validate();
                  if (!request) return;
                  setFields(requestToFields(request));
                }
                setMode("form");
              }}
            >
              {c.form}
            </Button>
            <Button
              type="button"
              variant={mode === "json" ? "default" : "outline"}
              aria-pressed={mode === "json"}
              onClick={() => {
                if (mode === "json") return;
                setJson(JSON.stringify(fieldsToRequest(fields), null, 2));
                setMode("json");
              }}
            >
              {c.json}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                const request = validate();
                if (request)
                  assign({ ...request, id: `sim-${crypto.randomUUID()}` });
              }}
            >
              {c.newId}
            </Button>
          </div>
          {mode === "form" ? (
            <>
              <div className="simulator-fields">
                {input("id")}
                {input("impression")}
                {input("placement")}
                {input("floor")}
                {input("width", "number")}
                {input("height", "number")}
                <label className="form-field">
                  {c.fields.currency}
                  <select
                    aria-label={c.fields.currency}
                    value={fields.currency}
                    onChange={(event) => update("currency", event.target.value)}
                  >
                    <option value="USD">USD</option>
                  </select>
                </label>
                <label className="form-field">
                  {c.fields.device}
                  <select
                    aria-label={c.fields.device}
                    value={fields.device}
                    onChange={(event) => update("device", event.target.value)}
                  >
                    <option value="">{c.anyDevice}</option>
                    {["mobile", "desktop", "tablet", "ctv"].map((value) => (
                      <option key={value}>{value}</option>
                    ))}
                  </select>
                </label>
                {input("country")}
                {input("timeout", "number")}
              </div>
              <p className="field-help">{c.moneyHelp}</p>
              <details>
                <summary>{c.optional}</summary>
                <div className="simulator-fields">
                  {(
                    [
                      "os",
                      "ua",
                      "region",
                      "city",
                      "siteId",
                      "domain",
                      "userId",
                    ] as const
                  ).map((key) => input(key))}
                </div>
              </details>
            </>
          ) : (
            <div className="form-field">
              <label htmlFor="sim-json">{c.requestJson}</label>
              <Textarea
                id="sim-json"
                className="simulator-json"
                rows={20}
                value={json}
                onChange={(event) => {
                  setJson(event.target.value);
                  setPreset("custom");
                  setCurl("");
                  try {
                    setFields(
                      requestToFields(parseBidJson(event.target.value)),
                    );
                  } catch {
                    /* Keep invalid JSON visible until corrected. */
                  }
                }}
              />
            </div>
          )}
          {testMode && (
            <div className="form-field">
              <label htmlFor="sim-test-time">{c.testTime}</label>
              <Input
                id="sim-test-time"
                value={testTime}
                onChange={(event) => setTestTime(event.target.value)}
                aria-describedby="sim-test-time-help"
              />
              <small id="sim-test-time-help">{c.testTimeHelp}</small>
            </div>
          )}
        </fieldset>
        {Object.keys(errors).length > 0 && (
          <ul className="field-error" role="alert">
            {Object.entries(errors).map(([path, message]) => (
              <li key={path}>
                <code>{path}</code>: {message}
              </li>
            ))}
          </ul>
        )}
        <div className="simulator-actions">
          <Button type="submit" disabled={busy}>
            {sending ? c.sending : c.send}
          </Button>
          {sending && (
            <Button type="button" variant="outline" onClick={onCancel}>
              {c.cancel}
            </Button>
          )}
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => {
              const request = validate();
              if (request) {
                const command = buildCurl(
                  gatewayUrl,
                  request,
                  testMode ? testTime || undefined : undefined,
                );
                setCurl(command);
                void navigator.clipboard
                  .writeText(command)
                  .then(() => toast.success(c.curlCopied))
                  .catch(() => toast.error(c.curlFailed));
              }
            }}
          >
            {c.copyCurl}
          </Button>
        </div>
        {curl && (
          <div className="simulator-curl">
            <p className="field-help">{c.curlHelp}</p>
            <CopyButton value={curl} />
            <pre>{curl}</pre>
          </div>
        )}
      </form>
      <details onToggle={(event) => setShowBurst(event.currentTarget.open)}>
        <summary>{c.burst}</summary>
        {showBurst && (
          <BurstControls
            running={running}
            disabled={sending}
            onCancel={onCancel}
            onRun={(config) => {
              const request = validate();
              if (request)
                onBurst(
                  request,
                  config,
                  testMode ? testTime || undefined : undefined,
                );
            }}
          />
        )}
      </details>
    </section>
  );
}
