import { describe, expect, it, vi } from "vitest";
import { waitForSidecarReadiness } from "./sidecar-readiness.js";

const sidecar = { origin: "http://127.0.0.1:53124", token: "sidecar-token" };

describe("sidecar readiness", () => {
	it("accepts an authenticated successful health response", async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response(null, { status: 200 }));

		await waitForSidecarReadiness(sidecar, { fetch: fetcher, timeoutMs: 100 });

		expect(fetcher).toHaveBeenCalledWith(`${sidecar.origin}/api/v1/health`, {
			method: "GET",
			headers: { "X-MediaGo-Sidecar-Token": sidecar.token },
			signal: expect.any(AbortSignal),
		});
	});

	it("times out after bounded retry attempts", async () => {
		let now = 0;
		const fetcher = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));

		await expect(
			waitForSidecarReadiness(sidecar, {
				fetch: fetcher,
				now: () => now,
				retryDelayMs: 40,
				sleep: async (durationMs) => {
					now += durationMs;
				},
				timeoutMs: 100,
			}),
		).rejects.toThrow("sidecar readiness timed out after 100ms");
		expect(fetcher).toHaveBeenCalledTimes(3);
	});

	it("fails immediately when the health request is not authenticated", async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));

		await expect(
			waitForSidecarReadiness(sidecar, { fetch: fetcher, timeoutMs: 100 }),
		).rejects.toThrow("sidecar readiness authentication failed with HTTP 401");
		expect(fetcher).toHaveBeenCalledOnce();
	});
});
