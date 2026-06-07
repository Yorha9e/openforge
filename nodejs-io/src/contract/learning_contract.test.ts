import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  QueryKnowledgeRequestSchema,
  QueryKnowledgeResponseSchema,
  type QueryKnowledgeRequest,
  type QueryKnowledgeResponse,
} from "../gen/agent/v1/learning_pb";
import { loadGolden } from "./golden_loader.js";

describe("Learning Contract (QueryKnowledge)", () => {
  it("QueryKnowledgeRequest maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("learning_query_knowledge.req.json") as {
      project_id: string;
      query: string;
      top_k: number;
      categories: string[];
      exclude_untrusted: boolean;
    };

    const req: QueryKnowledgeRequest = create(QueryKnowledgeRequestSchema, {
      projectId: golden.project_id,
      query: golden.query,
      topK: golden.top_k,
      categories: golden.categories,
      excludeUntrusted: golden.exclude_untrusted,
    });

    expect(req.projectId).toBe("proj-1");
    expect(req.query).toBe("how to validate auth tokens");
    expect(req.topK).toBe(5);
    expect(req.categories).toEqual(["code_style", "architecture"]);
    expect(req.excludeUntrusted).toBe(true);
  });

  it("QueryKnowledgeResponse maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("learning_query_knowledge.resp.json") as {
      matches: {
        entry: {
          type: string;
          category: string;
          description: string;
          confidence: number;
          source: string;
        };
        similarity_score: number;
        knowledge_id: string;
        trust_level: string;
      }[];
    };

    const resp: QueryKnowledgeResponse = create(QueryKnowledgeResponseSchema, {
      matches: golden.matches.map((m) => ({
        entry: {
          type: m.entry.type,
          category: m.entry.category,
          description: m.entry.description,
          confidence: m.entry.confidence,
          source: m.entry.source,
        },
        similarityScore: m.similarity_score,
        knowledgeId: m.knowledge_id,
        trustLevel: m.trust_level,
      })),
    });

    expect(resp.matches).toHaveLength(1);
    const m = resp.matches[0];
    expect(m.entry?.type).toBe("preference");
    expect(m.entry?.category).toBe("code_style");
    expect(m.entry?.description).toBe("Use zod for runtime validation");
    expect(m.entry?.confidence).toBeCloseTo(0.87);
    expect(m.entry?.source).toBe("pipeline_review");
    expect(m.similarityScore).toBeCloseTo(0.84);
    expect(m.knowledgeId).toBe("k-001");
    expect(m.trustLevel).toBe("trusted");
  });
});
