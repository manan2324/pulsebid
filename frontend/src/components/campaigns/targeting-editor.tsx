"use client";
import { useState } from "react";
import { countryCodes, countryName, timezones } from "@/lib/countries";
import {
  deviceTypes,
  placementTags,
  type TargetingValues,
} from "@/lib/schemas/targeting";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Combobox } from "@/components/ui/combobox";
import { HourGrid } from "./hour-grid";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function TargetingEditor({
  value,
  onChange,
  disabled = false,
}: {
  value: TargetingValues;
  onChange: (value: TargetingValues) => void;
  disabled?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [placement, setPlacement] = useState("");
  const toggle = (key: "countries" | "device_types", item: string) => {
    if (disabled) return;
    const values = value[key] as string[];
    onChange({
      ...value,
      [key]: values.includes(item)
        ? values.filter((code) => code !== item)
        : [...values, item].sort(),
    });
  };
  const add = () => {
    if (disabled) return;
    onChange({
      ...value,
      placement_ids: [
        ...new Set([...value.placement_ids, ...placementTags(placement)]),
      ],
    });
    setPlacement("");
  };
  return (
    <fieldset disabled={disabled} className="targeting-editor">
      <p className="field-help">{c.emptyMeansAny}</p>
      <section>
        <h3>{c.countries}</h3>
        <Input
          aria-label={c.countrySearch}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={c.countrySearch}
        />
        <div className="country-options">
          {countryCodes
            .filter((code) =>
              `${code} ${countryName(code)}`
                .toLowerCase()
                .includes(search.toLowerCase()),
            )
            .map((code) => (
              <label key={code}>
                <input
                  type="checkbox"
                  checked={value.countries.includes(code)}
                  onChange={() => toggle("countries", code)}
                />
                <span>
                  {countryName(code)} <code>{code}</code>
                </span>
              </label>
            ))}
        </div>
        <p className="field-help">
          {value.countries.length
            ? value.countries.map((code) => countryName(code)).join(", ")
            : c.anyCountry}
        </p>
      </section>
      <section>
        <h3>{c.devices}</h3>
        <div className="checkbox-row">
          {deviceTypes.map((device) => (
            <label key={device}>
              <input
                type="checkbox"
                checked={value.device_types.includes(device)}
                onChange={() => toggle("device_types", device)}
              />
              {device}
            </label>
          ))}
        </div>
        <p className="field-help">
          {value.device_types.join(", ") || c.anyDevice}
        </p>
      </section>
      <section>
        <h3>{c.placements}</h3>
        <div className="inline-control">
          <Input
            aria-label={c.placements}
            value={placement}
            onChange={(event) => setPlacement(event.target.value)}
            placeholder={c.placementPlaceholder}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                add();
              }
            }}
          />
          <Button type="button" variant="outline" onClick={add}>
            {c.addPlacement}
          </Button>
        </div>
        <div className="tag-list">
          {value.placement_ids.map((id) => (
            <Button
              key={id}
              type="button"
              variant="outline"
              size="sm"
              aria-label={`${c.remove} ${id}`}
              onClick={() =>
                onChange({
                  ...value,
                  placement_ids: value.placement_ids.filter(
                    (item) => item !== id,
                  ),
                })
              }
            >
              {id} ×
            </Button>
          ))}
        </div>
        <p className="field-help">
          {value.placement_ids.join(", ") || c.anyPlacement}
        </p>
      </section>
      <section>
        <h3>{c.hours}</h3>
        <HourGrid
          value={value.hours_of_day}
          onChange={(hours) => onChange({ ...value, hours_of_day: hours })}
          disabled={disabled}
        />
      </section>
      <section>
        <h3>{c.timezone}</h3>
        <Combobox
          label={c.timezone}
          value={value.timezone}
          options={timezones.map((zone) => ({ value: zone, label: zone }))}
          onChange={(timezone) => {
            if (!disabled) onChange({ ...value, timezone });
          }}
        />
        {value.hours_of_day.length > 0 &&
          value.timezone === "UTC" &&
          Intl.DateTimeFormat().resolvedOptions().timeZone !== "UTC" && (
            <p className="notice">{c.timezoneWarning}</p>
          )}
      </section>
    </fieldset>
  );
}
