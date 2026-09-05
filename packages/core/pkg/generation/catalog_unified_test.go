package generation

import "testing"

func TestUnifiedRoutesRoundTripAndCatalogLinks(t *testing.T) {
	for _, protocol := range []string{"images", "chat-image", "speech", "videos"} {
		t.Run(protocol, func(t *testing.T) {
			route, ok := UnifiedRoute("Vendor/Exact-Model:最新", protocol)
			if !ok {
				t.Fatal("route rejected")
			}
			resolved, ok := FindRoute(route.ID)
			if !ok || resolved.Model != route.Model || resolved.Provider != ProviderUnified {
				t.Fatalf("lost identity: %#v", resolved)
			}
			if _, ok := FindModel(route.ID); !ok {
				t.Fatal("legacy model missing")
			}
			if _, ok := FindRouteByLegacyModelID(route.ID); !ok {
				t.Fatal("legacy route missing")
			}
			catalog := ModelCatalog{}
			AppendUnifiedRoute(&catalog, route)
			AppendUnifiedRoute(&catalog, route)
			if len(catalog.Families) != 1 || len(catalog.Versions) != 1 || len(catalog.Routes) != 1 || len(catalog.Models) != 1 {
				t.Fatalf("incomplete or duplicate catalog: %#v", catalog)
			}
			if catalog.Versions[0].FamilyID != catalog.Families[0].ID || route.VersionID != catalog.Versions[0].ID {
				t.Fatal("broken selector links")
			}
		})
	}
	for _, id := range []string{"unified.images.!!!", "unified.invalid.dGVzdA", "unified.images."} {
		if _, ok := FindRoute(id); ok {
			t.Errorf("accepted invalid route %q", id)
		}
	}
}

func TestCodexImageRouteUsesNoAPIKey(t *testing.T) {
	route := CodexImageRoute()
	resolved, ok := FindRoute(route.ID)
	if !ok || resolved.Provider != ProviderCodexImage || len(resolved.AuthKeys) != 0 {
		t.Fatalf("route = %#v", resolved)
	}
	if _, ok := FindRouteByLegacyModelID(route.ID); !ok {
		t.Fatal("legacy route missing")
	}
	if _, ok := FindModel(route.ID); !ok {
		t.Fatal("legacy model missing")
	}
	catalog := ModelCatalog{}
	AppendUnifiedRoute(&catalog, route)
	if catalog.Versions[0].Label != route.Label {
		t.Fatal("subscription label missing")
	}
}
