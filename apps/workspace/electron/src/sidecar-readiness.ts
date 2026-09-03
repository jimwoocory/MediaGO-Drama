import type { SidecarConnection } from "./sidecar.js";

const sidecarTokenHeader = "X-MediaGo-Sidecar-Token";
const defaultTimeoutMs = 10_000;
const defaultRetryDelayMs = 200;

export interface SidecarReadinessOptions {
	fetch?: typeof globalThis.fetch;
	now?: () => number;
	retryDelayMs?: number;
	sleep?: (durationMs: number) => Promise<void>;
	timeoutMs?: number;
}

/** Waits until an authenticated sidecar health probe reports success. */
export const waitForSidecarReadiness = async (
	sidecar: SidecarConnection,
	options: SidecarReadinessOptions = {},
): Promise<void> => {
	const fetcher = options.fetch ?? globalThis.fetch;
	const now = options.now ?? Date.now;
	const sleep = options.sleep ?? defaultSleep;
	const timeoutMs = positiveDuration(options.timeoutMs, defaultTimeoutMs);
	const retryDelayMs = positiveDuration(options.retryDelayMs, defaultRetryDelayMs);
	const deadline = now() + timeoutMs;
	let lastError: unknown;

	while (true) {
		const remainingMs = deadline - now();
		if (remainingMs <= 0) {
			throw readinessTimeoutError(sidecar.origin, timeoutMs, lastError);
		}

		const controller = new AbortController();
		const abortTimer = setTimeout(() => controller.abort(), remainingMs);
		try {
			const response = await fetcher(`${sidecar.origin}/api/v1/health`, {
				method: "GET",
				headers: { [sidecarTokenHeader]: sidecar.token },
				signal: controller.signal,
			});
			if (response.ok) return;
			if (response.status === 401 || response.status === 403) {
				throw new Error(`sidecar readiness authentication failed with HTTP ${response.status}`);
			}
			lastError = new Error(`sidecar readiness returned HTTP ${response.status}`);
		} catch (error) {
			if (isAuthenticationFailure(error)) throw error;
			lastError = error;
		} finally {
			clearTimeout(abortTimer);
		}

		const delayMs = Math.min(retryDelayMs, Math.max(0, deadline - now()));
		if (delayMs <= 0) throw readinessTimeoutError(sidecar.origin, timeoutMs, lastError);
		await sleep(delayMs);
	}
};

const positiveDuration = (value: number | undefined, fallback: number): number =>
	typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;

const defaultSleep = (durationMs: number): Promise<void> =>
	new Promise((resolve) => setTimeout(resolve, durationMs));

const isAuthenticationFailure = (error: unknown): error is Error =>
	error instanceof Error && error.message.startsWith("sidecar readiness authentication failed");

const readinessTimeoutError = (origin: string, timeoutMs: number, lastError: unknown): Error => {
	const detail = lastError instanceof Error ? `: ${lastError.message}` : "";
	return new Error(`sidecar readiness timed out after ${timeoutMs}ms for ${origin}${detail}`);
};
