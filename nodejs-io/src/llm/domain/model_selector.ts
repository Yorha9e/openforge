// llm/domain/model_selector.ts — WFQ priority scheduler
// Model names may carry a context-window suffix: "mimo-v2.5-pro[1m]" → 1M tokens.

interface ModelEntry {
  provider: string;
  model: string;
  priority: number;
  weight: number;
  contextWindow: number;
}

/** Strip and parse a "[Nm]" or "[Nk]" suffix from a model name. */
function parseContextWindow(model: string): { cleanName: string; contextWindow: number } {
  const match = model.match(/\[(\d+)([mk])\]$/i);
  if (!match) return { cleanName: model, contextWindow: 200_000 };
  const num = parseInt(match[1]!, 10);
  const unit = match[2]!.toLowerCase();
  const multiplier = unit === "m" ? 1_048_576 : 1_024;
  return {
    cleanName: model.slice(0, match.index),
    contextWindow: num * multiplier,
  };
}

function envModel(key: string, fallback: string): string {
  return process.env[key] || fallback;
}

export class ModelSelector {
  private models: ModelEntry[];

  constructor() {
    const sonnetModel  = envModel("ANTHROPIC_DEFAULT_SONNET_MODEL", "claude-sonnet-4-6[200k]");
    const opusModel    = envModel("ANTHROPIC_DEFAULT_OPUS_MODEL",   "claude-opus-4-7[200k]");
    const haikuModel   = envModel("ANTHROPIC_DEFAULT_HAIKU_MODEL",  "claude-haiku-4-5-20251001[200k]");

    this.models = [sonnetModel, opusModel, haikuModel]
      .filter((m, i, arr) => arr.indexOf(m) === i)          // dedupe
      .map((m, i) => {
        const { cleanName, contextWindow } = parseContextWindow(m);
        return {
          provider: "anthropic",
          model: cleanName,
          priority: i,
          weight: 1,
          contextWindow,
        };
      });
  }

  select(preferred?: string): ModelEntry {
    if (preferred) {
      const found = this.models.find((m) => m.model === preferred);
      if (found) return found;
    }
    return this.models[0]!;
  }

  list(): ModelEntry[] {
    return [...this.models];
  }
}
