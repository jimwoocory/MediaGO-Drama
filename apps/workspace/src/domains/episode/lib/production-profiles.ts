import type { ProductionPlan, ProductionProfileDefinition } from "@/domains/episode/lib/production";
import {
	builtinProductionProfileManifest,
	loadProductionProfileManifest,
} from "@/domains/episode/lib/production-profile-manifest";
import {
	resolveProductionPlanningDirective,
	type ProductionPlanningDirective,
	validateProductionPlanningProfile,
} from "@/domains/episode/lib/production-planning";

export interface ProductionProfileRegistry {
	list: () => ProductionProfileDefinition[];
	get: (id: string) => ProductionProfileDefinition | undefined;
	has: (id: string) => boolean;
}

/**
 * Built-in production profiles are intentionally registered separately from the Shot Schema.
 * The legacy six-mode definitions can be restored here without migrating ProductionShot data.
 */
export const builtinProductionProfiles: readonly ProductionProfileDefinition[] =
	loadProductionProfileManifest(builtinProductionProfileManifest);

export const createProductionProfileRegistry = (
	profiles: readonly ProductionProfileDefinition[] = builtinProductionProfiles,
): ProductionProfileRegistry => {
	const byID = new Map<string, ProductionProfileDefinition>();
	for (const profile of profiles) {
		const normalized = normalizeProductionProfile(profile);
		const planningErrors = validateProductionPlanningProfile(normalized);
		if (planningErrors.length > 0) {
			throw new Error(`invalid production profile ${normalized.id}: ${planningErrors.join("; ")}`);
		}
		if (byID.has(normalized.id)) {
			throw new Error(`duplicate production profile id: ${normalized.id}`);
		}
		byID.set(normalized.id, normalized);
	}
	return {
		list: () => Array.from(byID.values()),
		get: (id) => byID.get(normalizeProfileID(id)),
		has: (id) => byID.has(normalizeProfileID(id)),
	};
};

export const productionProfileRegistry = createProductionProfileRegistry();

export const resolveProductionPlanPlanningDirective = (
	plan: ProductionPlan,
	registry: ProductionProfileRegistry = productionProfileRegistry,
): ProductionPlanningDirective | undefined => {
	const profileID = normalizeProfileID(plan.profileId ?? "");
	if (!profileID) return undefined;
	const profile = registry.get(profileID);
	if (!profile) return undefined;
	return resolveProductionPlanningDirective(profile, plan.targetDurationSeconds);
};

export const applyProductionProfileSelection = (
	plan: ProductionPlan,
	selection: {
		profileId?: string;
		targetDurationSeconds?: number;
	},
	registry: ProductionProfileRegistry = productionProfileRegistry,
): ProductionPlan => {
	const requestedProfileID = normalizeProfileID(selection.profileId ?? "");
	const profileID =
		requestedProfileID && registry.has(requestedProfileID) ? requestedProfileID : undefined;
	const targetDurationSeconds = finiteNonNegative(selection.targetDurationSeconds);
	return {
		...plan,
		profileId: profileID,
		targetDurationSeconds,
		// Shots remain untouched: a profile is planning metadata, not a schema migration.
		shots: plan.shots,
	};
};

function normalizeProductionProfile(
	profile: ProductionProfileDefinition,
): ProductionProfileDefinition {
	const id = normalizeProfileID(profile.id);
	if (!id) throw new Error("production profile id is required");
	const label = profile.label.trim();
	if (!label) throw new Error(`production profile label is required: ${id}`);
	if (!Number.isInteger(profile.version) || profile.version < 1) {
		throw new Error(`production profile version must be a positive integer: ${id}`);
	}
	return {
		...profile,
		id,
		label,
		description: profile.description.trim(),
	};
}

function normalizeProfileID(value: string) {
	return value.trim().toLowerCase();
}

function finiteNonNegative(value?: number) {
	return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : undefined;
}
