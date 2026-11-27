"use client";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { BurstConfig } from "@/lib/burst";
import { simulatorCopy as c } from "@/lib/copy/simulator";
export function BurstControls({
  running,
  disabled,
  onRun,
  onCancel,
}: {
  running: boolean;
  disabled: boolean;
  onRun: (config: BurstConfig) => void;
  onCancel: () => void;
}) {
  const [count, setCount] = useState("100");
  const [concurrency, setConcurrency] = useState("4");
  const [mode, setMode] = useState<BurstConfig["mode"]>("fixed");
  const [error, setError] = useState("");
  return (
    <div className="burst-controls">
      <p className="notice">{c.burstHelp}</p>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          const n = Number(count),
            parallel = Number(concurrency);
          if (
            !Number.isInteger(n) ||
            n < 1 ||
            n > 200 ||
            !Number.isInteger(parallel) ||
            parallel < 1 ||
            parallel > 10
          ) {
            setError(c.burstError);
            return;
          }
          setError("");
          onRun({ count: n, concurrency: parallel, mode });
        }}
        noValidate
      >
        <fieldset disabled={running || disabled}>
          <div className="simulator-fields">
            <label className="form-field">
              {c.count}
              <Input
                type="number"
                min={1}
                max={200}
                value={count}
                onChange={(event) => setCount(event.target.value)}
              />
            </label>
            <label className="form-field">
              {c.concurrency}
              <Input
                type="number"
                min={1}
                max={10}
                value={concurrency}
                onChange={(event) => setConcurrency(event.target.value)}
              />
            </label>
            <label className="form-field">
              {c.requestMode}
              <select
                aria-label={c.requestMode}
                value={mode}
                onChange={(event) =>
                  setMode(event.target.value as BurstConfig["mode"])
                }
              >
                <option value="fixed">{c.fixed}</option>
                <option value="random">{c.random}</option>
              </select>
            </label>
          </div>
          <p className="field-help">{c.randomHelp}</p>
          <Button disabled={running || disabled}>
            {running ? c.running : c.runBurst}
          </Button>
        </fieldset>
        {error && (
          <p role="alert" className="field-error">
            {error}
          </p>
        )}
      </form>
      {running && (
        <Button variant="outline" onClick={onCancel}>
          {c.cancelBurst}
        </Button>
      )}
    </div>
  );
}
