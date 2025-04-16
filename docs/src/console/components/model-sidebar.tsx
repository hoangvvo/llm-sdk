import type { LanguageModelMetadata } from "@hoangvvo/llm-sdk";
import type { Dispatch, SetStateAction } from "react";

export interface ModelSelection {
  provider: string;
  modelId: string;
}

export interface ModelOption extends ModelSelection {
  label: string;
  metadata?: LanguageModelMetadata;
}

interface SidebarProps {
  models: ModelOption[];
  selection: ModelSelection | null;
  onChange: Dispatch<SetStateAction<ModelSelection | null>>;
  errorMessage?: string | null;
}

export function Sidebar({
  models,
  selection,
  onChange,
  errorMessage,
}: SidebarProps) {
  const selected = selection
    ? models.find(
        (item) =>
          item.provider === selection.provider &&
          item.modelId === selection.modelId,
      )
    : null;

  return (
    <aside className="w-96 shrink-0 border-l border-slate-200/70 bg-white/60 px-6 py-6 backdrop-blur-sm">
      <div className="space-y-4">
        <div>
          <h2 className="text-xs uppercase tracking-[0.3em] text-slate-500">
            Model
          </h2>
          <p className="mt-2 text-xs text-slate-500">
            Choose which LLM model the agent should use.
          </p>
          {models.length > 0 ? (
            <select
              className="mt-3 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-700 focus:border-slate-500 focus:outline-none focus:ring-2 focus:ring-slate-300"
              value={selected ? `${selected.provider}:${selected.modelId}` : ""}
              onChange={(event) => {
                const [provider, modelId] = event.target.value.split(":");
                onChange({ provider, modelId });
              }}
            >
              <option value="" disabled>
                Select a model
              </option>
              {models.map((option) => (
                <option
                  key={`${option.provider}:${option.modelId}`}
                  value={`${option.provider}:${option.modelId}`}
                >
                  {option.label}
                </option>
              ))}
            </select>
          ) : (
            <div className="mt-3 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-500">
              {errorMessage ?? "Loading models…"}
            </div>
          )}
        </div>
        {selected ? <ModelDetails option={selected} /> : null}
      </div>
    </aside>
  );
}

function ModelDetails({ option }: { option: ModelOption }) {
  const pricing = option.metadata?.pricing;
  const capabilities = option.metadata?.capabilities ?? [];
  const pricingEntries: [string, number | null | undefined][] = pricing
    ? [
        ["Input text token", pricing.input_cost_per_text_token],
        ["Input cached text token", pricing.input_cost_per_cached_text_token],
        ["Output text token", pricing.output_cost_per_text_token],
        ["Input audio token", pricing.input_cost_per_audio_token],
        ["Input cached audio token", pricing.input_cost_per_cached_audio_token],
        ["Output audio token", pricing.output_cost_per_audio_token],
        ["Input image token", pricing.input_cost_per_image_token],
        ["Input cached image token", pricing.input_cost_per_cached_image_token],
        ["Output image token", pricing.output_cost_per_image_token],
      ]
    : [];
  const hasPricing = pricing ? hasAnyPricing(pricing) : false;

  return (
    <div className="rounded-xl border border-slate-200/80 bg-white/70 p-4 text-xs text-slate-600">
      <h3 className="text-[11px] uppercase tracking-[0.25em] text-slate-500">
        Capabilities
      </h3>
      {capabilities.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-2">
          {capabilities.map((capability) => (
            <li
              key={capability}
              className="rounded-full border border-slate-300 px-2 py-0.5 text-[11px] uppercase tracking-wide text-slate-500"
            >
              {formatCapability(capability)}
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-2 text-slate-400">No capability data.</p>
      )}

      <h3 className="mt-4 text-[11px] uppercase tracking-[0.25em] text-slate-500">
        Pricing (USD/M tokens)
      </h3>
      {pricing ? (
        <div className="mt-2 space-y-1 text-slate-500">
          {pricingEntries.map(([label, value]) => {
            if (value === undefined || value === null) {
              return null;
            }
            return (
              <p key={label}>
                <span className="font-semibold text-slate-600">{label}:</span>{" "}
                <span className="text-slate-700">
                  ${(value * 1_000_000).toFixed(2)}
                </span>
              </p>
            );
          })}
          {hasPricing ? null : (
            <p className="text-slate-400">No pricing data.</p>
          )}
        </div>
      ) : (
        <p className="mt-2 text-slate-400">No pricing data.</p>
      )}
    </div>
  );
}

function formatCapability(capability: string): string {
  return capability
    .split("-")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function hasAnyPricing(pricing: LanguageModelMetadata["pricing"]): boolean {
  if (!pricing) return false;
  return [
    pricing.input_cost_per_text_token,
    pricing.input_cost_per_cached_text_token,
    pricing.output_cost_per_text_token,
    pricing.input_cost_per_audio_token,
    pricing.input_cost_per_cached_audio_token,
    pricing.output_cost_per_audio_token,
    pricing.input_cost_per_image_token,
    pricing.input_cost_per_cached_image_token,
    pricing.output_cost_per_image_token,
  ].some((value) => value !== undefined);
}
