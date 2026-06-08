import { describe, it, expect, vi } from "vitest";
import { buildOnboardingSettings, persistOnboarding } from "./OnboardingFlow";

describe("OnboardingFlow", () => {
  describe("buildOnboardingSettings", () => {
    it("maps role, projectType, and setupComplete for an 'existing' repo", () => {
      const payload = buildOnboardingSettings("developer", "existing", "https://github.com/foo/bar.git");
      expect(payload).toEqual({
        role: "developer",
        projectType: "existing",
        setupComplete: true,
        repoUrl: "https://github.com/foo/bar.git",
      });
    });

    it("omits repoUrl when starting a fresh project", () => {
      const payload = buildOnboardingSettings("pm", "new", "");
      expect(payload).toEqual({
        role: "pm",
        projectType: "new",
        setupComplete: true,
      });
      expect((payload as Record<string, unknown>).repoUrl).toBeUndefined();
    });

    it("supports all three role ids", () => {
      for (const role of ["pm", "developer", "reviewer"] as const) {
        const payload = buildOnboardingSettings(role, "new", "");
        expect(payload.role).toBe(role);
      }
    });
  });

  describe("persistOnboarding", () => {
    it("calls api.updateSettings with the onboarding payload", async () => {
      const updateSettings = vi.fn().mockResolvedValue({});
      const navigate = vi.fn();
      const onFinish = vi.fn();

      const api = { updateSettings } as unknown as Parameters<typeof persistOnboarding>[0]["api"];

      await persistOnboarding(
        { api, navigate, onFinish },
        { role: "developer", setupType: "existing", repoUrl: "https://github.com/foo/bar.git" }
      );

      expect(updateSettings).toHaveBeenCalledTimes(1);
      expect(updateSettings).toHaveBeenCalledWith({
        role: "developer",
        projectType: "existing",
        setupComplete: true,
        repoUrl: "https://github.com/foo/bar.git",
      });
    });

    it("navigates to home and invokes onFinish after a successful update", async () => {
      const updateSettings = vi.fn().mockResolvedValue({});
      const navigate = vi.fn();
      const onFinish = vi.fn();

      const api = { updateSettings } as unknown as Parameters<typeof persistOnboarding>[0]["api"];

      await persistOnboarding(
        { api, navigate, onFinish },
        { role: "pm", setupType: "new", repoUrl: "" }
      );

      expect(navigate).toHaveBeenCalledWith("/", { replace: true });
      expect(onFinish).toHaveBeenCalledTimes(1);
    });

    it("does not navigate or invoke onFinish if updateSettings rejects", async () => {
      const updateSettings = vi.fn().mockRejectedValue(new Error("network"));
      const navigate = vi.fn();
      const onFinish = vi.fn();

      const api = { updateSettings } as unknown as Parameters<typeof persistOnboarding>[0]["api"];

      await expect(
        persistOnboarding(
          { api, navigate, onFinish },
          { role: "reviewer", setupType: "new", repoUrl: "" }
        )
      ).rejects.toThrow("network");

      expect(navigate).not.toHaveBeenCalled();
      expect(onFinish).not.toHaveBeenCalled();
    });
  });
});
