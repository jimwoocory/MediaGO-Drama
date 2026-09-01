import httpClient from "@/shared/lib/http";
import type { ProductionProfileDefinition } from "@/domains/episode/lib/production";

export const productionProfilesKey = "/production-profiles";

export interface ProductionProfilesResponse {
	schemaVersion: 1;
	profiles: ProductionProfileDefinition[];
}

export const getProductionProfiles = async (): Promise<ProductionProfilesResponse> => {
	const response = await httpClient.get<ProductionProfilesResponse>(productionProfilesKey);
	return response.data ?? { schemaVersion: 1, profiles: [] };
};
