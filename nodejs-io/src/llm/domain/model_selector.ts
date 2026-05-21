// llm/domain/model_selector.ts — WFQ priority scheduler

interface ModelEntry {
  provider: string;
  model: string;
  priority: number;
  weight: number;
}

export class ModelSelector {
  private models: ModelEntry[] = [
    { provider: "anthropic", model: "claude-sonnet-4-6", priority: 0, weight: 1 },
    { provider: "anthropic", model: "claude-haiku-4-5-20251001", priority: 1, weight: 1 },
  ];

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
