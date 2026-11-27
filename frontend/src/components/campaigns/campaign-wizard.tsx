"use client";
import { useEffect, useReducer, useRef, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useForm, useWatch } from "react-hook-form";
import { z } from "zod";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/data/shared";

import {
  campaignSchema,
  campaignFieldsSchema,
  type CampaignFormValues,
} from "@/lib/schemas/campaign";
import {
  targetingSchema,
  emptyTargeting,
  type TargetingValues,
} from "@/lib/schemas/targeting";
import {
  creativeSchema,
  starterCreative,
  type CreativeValues,
} from "@/lib/schemas/creative";
import {
  journalSchema,
  emptyJournal,
  type CreationJournal,
} from "@/lib/campaign-journal";
import { useCatalogOptions } from "@/hooks/use-campaigns";
import { useCampaignCreation } from "@/hooks/use-campaign-creation";
import { useAdvertiserMutation } from "@/hooks/use-catalog";
import type { AdvertiserCreate } from "@/lib/api/advertisers";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
import { useQueryClient } from "@tanstack/react-query";
import { getBidderIndex } from "@/lib/bidder-index";
import { formatNumber } from "@/lib/format";
import { ApiClientError } from "@/lib/api/client";
import { campaignCopy as c } from "@/lib/copy/campaign";
const CampaignFields = dynamic(
  () => import("./campaign-fields").then((module) => module.CampaignFields),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
const BidderHealth = dynamic(
  () =>
    import("@/components/catalog/bidder-health").then(
      (module) => module.BidderHealth,
    ),
  { ssr: false, loading: () => <Skeleton className="h-4 w-16" /> },
);
const TargetingEditor = dynamic(
  () => import("./targeting-editor").then((module) => module.TargetingEditor),
  { loading: () => <Skeleton className="h-64 w-full" /> },
);
const CreativeEditor = dynamic(
  () => import("./creative-editor").then((module) => module.CreativeEditor),
  { loading: () => <Skeleton className="h-64 w-full" /> },
);
const AdvertiserForm = dynamic(() =>
  import("@/components/catalog/catalog-forms").then(
    (module) => module.AdvertiserForm,
  ),
);
const WizardReview = dynamic(
  () => import("./wizard-review").then((m) => m.WizardReview),
  { ssr: false, loading: () => <Skeleton className="h-96 w-full" /> },
);
const storageKey = "pulsebid.campaign-wizard.v1";
const rawFields = campaignFieldsSchema.extend({
  name: z.string(),
  advertiser_id: z.string(),
  bidder_id: z.string(),
  daily_budget: z.string(),
  max_bid_cpm: z.string(),
});
const rawCreative = creativeSchema.extend({
  name: z.string(),
  width: z.string(),
  height: z.string(),
  markup: z.string(),
  landing_url: z.string(),
});
const storedSchema = z.object({
  version: z.literal(1),
  values: rawFields,
  targeting: targetingSchema,
  creatives: z.array(rawCreative),
  activate: z.boolean(),
  step: z.number().int().min(0).max(4),
  journal: journalSchema,
});
export type State = z.infer<typeof storedSchema>;
type Action =
  | { type: "hydrate"; state: State }
  | { type: "values"; values: CampaignFormValues }
  | { type: "targeting"; value: TargetingValues }
  | { type: "creatives"; value: CreativeValues[] }
  | { type: "step"; value: number }
  | { type: "activate"; value: boolean }
  | { type: "checkpoint"; value: CreationJournal };
const initial: State = {
  version: 1,
  values: {
    name: "",
    advertiser_id: "",
    bidder_id: "",
    daily_budget: "50",
    max_bid_cpm: "3",
    pacing_mode: "even",
    start_at: "",
    end_at: "",
  },
  targeting: emptyTargeting,
  creatives: [starterCreative],
  activate: true,
  step: 0,
  journal: emptyJournal,
};
function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "hydrate":
      return action.state;
    case "values":
      return { ...state, values: action.values };
    case "targeting":
      return { ...state, targeting: action.value };
    case "creatives":
      return { ...state, creatives: action.value };
    case "step":
      return { ...state, step: action.value };
    case "activate":
      return { ...state, activate: action.value };
    case "checkpoint":
      return { ...state, journal: action.value };
  }
}
export function CampaignWizard({
  basePort,
  count,
}: {
  basePort: number;
  count: number;
}) {
  const [state, dispatch] = useReducer(reducer, initial);
  const [ready, setReady] = useState(false);
  const readyRef = useRef(false);
  const latest = useRef(state);
  latest.current = state;
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [creativeErrors, setCreativeErrors] = useState<
    Record<number, Partial<Record<keyof CreativeValues, string>>>
  >({});
  const [newAdvertiser, setNewAdvertiser] = useState(false);
  const form = useForm<CampaignFormValues>({
    resolver: async (values, context, options) =>
      (await import("@hookform/resolvers/zod")).zodResolver(campaignSchema)(
        values,
        context,
        options,
      ),
    defaultValues: initial.values,
  });
  const watched = useWatch({ control: form.control });
  const options = useCatalogOptions();
  const creation = useCampaignCreation();
  const advertiserMutation = useAdvertiserMutation();
  const queryClient = useQueryClient();
  const locked =
    Boolean(state.journal.campaignId || state.journal.pending) ||
    creation.isPending;
  const allowLeave = useDirtyGuard(
    ready && Boolean(state.values.name || state.journal.campaignId),
  );
  const persist = (next: State) => {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(next));
    } catch {
      setNotice(c.invalidDraft);
    }
  };
  useEffect(() => {
    try {
      const raw = sessionStorage.getItem(storageKey);
      if (raw) {
        const saved = storedSchema.parse(JSON.parse(raw));
        form.reset(saved.values);
        dispatch({ type: "hydrate", state: saved });
        setNotice(c.draftRestored);
      }
    } catch {
      setNotice(c.invalidDraft);
    }
    readyRef.current = true;
    setReady(true);
  }, [form]);
  useEffect(() => {
    const subscription = form.watch(() => {
      if (readyRef.current)
        dispatch({ type: "values", values: form.getValues() });
    });
    return () => subscription.unsubscribe();
  }, [form]);
  useEffect(() => {
    if (ready) persist(state);
  }, [state, ready]);
  const validateCreative = () => {
    const errors: typeof creativeErrors = {};
    state.creatives.forEach((value, index) => {
      const parsed = creativeSchema.safeParse(value);
      if (!parsed.success) {
        errors[index] = {};
        for (const issue of parsed.error.issues) {
          const field = issue.path[0] as keyof CreativeValues;
          errors[index]![field] = issue.message;
        }
      }
    });
    setCreativeErrors(errors);
    if (!state.creatives.length) {
      setError(c.creativeRequired);
      return false;
    }
    if (
      new Set(state.creatives.map((item) => item.name)).size !==
      state.creatives.length
    ) {
      setError(c.uniqueCreatives);
      return false;
    }
    return Object.keys(errors).length === 0;
  };
  const next = async () => {
    setError("");
    let valid = true;
    if (state.step === 0)
      valid = await form.trigger([
        "name",
        "advertiser_id",
        "bidder_id",
        "start_at",
        "end_at",
      ]);
    if (state.step === 1)
      valid = await form.trigger([
        "daily_budget",
        "max_bid_cpm",
        "pacing_mode",
      ]);
    if (state.step === 2) {
      const result = targetingSchema.safeParse(state.targeting);
      valid = result.success;
      if (!result.success)
        setError(result.error.issues.map((issue) => issue.message).join("; "));
    }
    if (state.step === 3) valid = validateCreative();
    if (valid) dispatch({ type: "step", value: Math.min(4, state.step + 1) });
  };
  if (!ready || options.advertisers.isPending || options.bidders.isPending)
    return <Skeleton className="h-96 w-full" />;
  const optionError = options.advertisers.error ?? options.bidders.error;
  if (optionError)
    return (
      <ErrorState
        message={c.optionError}
        retry={() => {
          void options.advertisers.refetch();
          void options.bidders.refetch();
        }}
      />
    );
  const advertisers = options.advertisers.data ?? [];
  const bidders = options.bidders.data ?? [];
  const selectedBidder = bidders.find((item) => item.id === watched.bidder_id);
  const steps = [c.basics, c.budgetStep, c.targeting, c.creatives, c.review];
  return (
    <>
      <PageHeader title={c.newCampaign} description={c.creationHelp} />
      {notice && (
        <p className="notice" role="status">
          {notice}
        </p>
      )}
      <ol className="wizard-steps">
        {steps.map((label, index) => (
          <li
            key={label}
            aria-current={index === state.step ? "step" : undefined}
          >
            <span>{index + 1}</span>
            {label}
          </li>
        ))}
      </ol>
      <form
        className="panel wizard-panel"
        onSubmit={(event) => {
          event.preventDefault();
          if (state.step < 4) void next();
        }}
      >
        <h2>{steps[state.step]}</h2>
        {state.step === 0 && (
          <>
            <p className="field-help">{c.basicsHelp}</p>
            <CampaignFields
              form={form}
              section="basics"
              advertisers={advertisers}
              bidders={bidders}
              disabled={locked}
            />
            <Button
              type="button"
              variant="outline"
              disabled={locked}
              onClick={() => setNewAdvertiser(true)}
            >
              {c.createAdvertiser}
            </Button>
            {selectedBidder && (
              <div className="inline-control">
                <span>
                  {c.selectedBidder}: {selectedBidder.name}
                </span>
                <BidderHealth
                  index={getBidderIndex(selectedBidder.endpoint_url, {
                    basePort,
                    count,
                  })}
                />
              </div>
            )}
            {(!advertisers.some((item) => item.status === "active") ||
              !bidders.some((item) => item.enabled)) && (
              <p className="notice">{c.noOptions}</p>
            )}
          </>
        )}
        {state.step === 1 && (
          <>
            <CampaignFields form={form} section="budget" disabled={locked} />
            <p className="estimate">
              {c.estimate}:{" "}
              <strong>
                {Number(watched.max_bid_cpm) > 0
                  ? formatNumber(
                      (Number(watched.daily_budget) /
                        Number(watched.max_bid_cpm)) *
                        1000,
                    )
                  : "—"}
              </strong>
            </p>
            <p className="field-help">{c.estimateHelp}</p>
          </>
        )}
        {state.step === 2 && (
          <TargetingEditor
            value={state.targeting}
            disabled={locked}
            onChange={(value) => dispatch({ type: "targeting", value })}
          />
        )}
        {state.step === 3 && (
          <>
            <div className="wizard-creatives">
              {state.creatives.map((value, index) => (
                <section key={index} className="wizard-creative">
                  <CreativeEditor
                    prefix={`creative-${index}`}
                    value={value}
                    disabled={locked}
                    errors={creativeErrors[index]}
                    onChange={(value) =>
                      dispatch({
                        type: "creatives",
                        value: state.creatives.map((item, i) =>
                          i === index ? value : item,
                        ),
                      })
                    }
                  />
                  <Button
                    type="button"
                    variant="outline"
                    disabled={locked}
                    onClick={() =>
                      dispatch({
                        type: "creatives",
                        value: state.creatives.filter((_, i) => i !== index),
                      })
                    }
                  >
                    {c.removeCreative}
                  </Button>
                </section>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              disabled={locked}
              onClick={() =>
                dispatch({
                  type: "creatives",
                  value: [
                    ...state.creatives,
                    {
                      ...starterCreative,
                      name: `${c.creativeName} ${state.creatives.length + 1}`,
                    },
                  ],
                })
              }
            >
              {c.addCreative}
            </Button>
          </>
        )}
        {state.step === 4 && (
          <WizardReview
            state={state}
            advertisers={advertisers}
            bidderName={selectedBidder?.name}
            locked={locked}
            onActivate={(value) => dispatch({ type: "activate", value })}
          />
        )}
        {error && (
          <p role="alert" className="field-error">
            {error}
          </p>
        )}
        {creation.error && (
          <ErrorState
            message={creation.error.message}
            requestId={
              creation.error instanceof ApiClientError
                ? creation.error.requestId
                : undefined
            }
          />
        )}
        <div className="wizard-actions">
          <Button
            type="button"
            variant="outline"
            disabled={state.step === 0 || creation.isPending || locked}
            onClick={() => dispatch({ type: "step", value: state.step - 1 })}
          >
            {c.back}
          </Button>
          {state.step < 4 ? (
            <Button
              type="button"
              onClick={() => {
                void next();
              }}
            >
              {c.next}
            </Button>
          ) : (
            <Button
              type="button"
              disabled={
                creation.isPending ||
                (state.activate &&
                  !state.creatives.some((value) => value.status === "active"))
              }
              onClick={async () => {
                setError("");
                const valid = await form.trigger();
                if (
                  !valid ||
                  !validateCreative() ||
                  !targetingSchema.safeParse(state.targeting).success
                ) {
                  setError(c.fieldErrors);
                  return;
                }
                try {
                  const campaign = await creation.mutateAsync({
                    draft: {
                      values: form.getValues(),
                      targeting: state.targeting,
                      creatives: state.creatives,
                      activate: state.activate,
                    },
                    journal: state.journal,
                    onCheckpoint: (journal) => {
                      const next = { ...latest.current, journal };
                      latest.current = next;
                      persist(next);
                      dispatch({ type: "checkpoint", value: journal });
                    },
                  });
                  sessionStorage.removeItem(storageKey);
                  allowLeave();
                  window.location.assign(`/campaigns/${campaign.id}`);
                } catch {
                  /* Mutation feedback retains the request ID and saved checkpoint. */
                }
              }}
            >
              {creation.isPending
                ? c.creating
                : creation.error
                  ? c.retry
                  : c.create}
            </Button>
          )}
        </div>
      </form>
      <div className="wizard-footer">
        <Link
          href="/campaigns"
          onClick={(event) => {
            if (creation.isPending) event.preventDefault();
          }}
        >
          {c.cancelCreation}
        </Link>
        <Button
          variant="ghost"
          disabled={creation.isPending}
          onClick={() => {
            if (window.confirm(c.resetDraftConfirm)) {
              sessionStorage.removeItem(storageKey);
              allowLeave();
              form.reset(initial.values);
              dispatch({ type: "hydrate", state: initial });
              creation.reset();
              setError("");
              setNotice("");
            }
          }}
        >
          {c.resetDraft}
        </Button>
      </div>
      {newAdvertiser && (
        <AdvertiserForm
          onClose={() => setNewAdvertiser(false)}
          onSave={async (body) => {
            const result = await advertiserMutation.mutateAsync({
              type: "create",
              body: body as AdvertiserCreate,
            });
            await queryClient.invalidateQueries({
              queryKey: ["advertiser-options"],
            });
            if (result)
              form.setValue("advertiser_id", result.id, {
                shouldDirty: true,
                shouldValidate: true,
              });
          }}
        />
      )}
    </>
  );
}
