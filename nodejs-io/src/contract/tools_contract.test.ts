import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  SearchToolsRequestSchema,
  SearchToolsResponseSchema,
  type SearchToolsRequest,
  type SearchToolsResponse,
} from "../gen/agent/v1/tools_pb";
import { loadGolden } from "./golden_loader.js";

describe("Tools Contract (SearchTools)", () => {
  it("SearchToolsRequest maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("tools_search.req.json") as {
      query: string;
      top_k: number;
      filter_categories: string[];
    };

    const req: SearchToolsRequest = create(SearchToolsRequestSchema, {
      query: golden.query,
      topK: golden.top_k,
      filterCategories: golden.filter_categories,
    });

    expect(req.query).toBe("search for files matching *.go");
    expect(req.topK).toBe(3);
    expect(req.filterCategories).toEqual(["file", "git"]);
  });

  it("SearchToolsResponse maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("tools_search.resp.json") as {
      matches: {
        tool: {
          name: string;
          description: string;
          category: string;
          is_dynamic: boolean;
          mcp_server_id: string;
        };
        similarity_score: number;
      }[];
    };

    const resp: SearchToolsResponse = create(SearchToolsResponseSchema, {
      matches: golden.matches.map((m) => ({
        tool: {
          name: m.tool.name,
          description: m.tool.description,
          category: m.tool.category,
          isDynamic: m.tool.is_dynamic,
          mcpServerId: m.tool.mcp_server_id,
        },
        similarityScore: m.similarity_score,
      })),
    });

    expect(resp.matches).toHaveLength(1);
    expect(resp.matches[0].tool?.name).toBe("file_grep");
    expect(resp.matches[0].tool?.category).toBe("file");
    expect(resp.matches[0].tool?.isDynamic).toBe(false);
    expect(resp.matches[0].similarityScore).toBeCloseTo(0.92);
  });
});
