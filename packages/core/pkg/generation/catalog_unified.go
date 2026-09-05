package generation

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// ProviderUnified reuses the existing aggregation credential.
const ProviderUnified = "aihubmix"

// UnifiedRoute builds a restart-safe route without mutable global registration.
func UnifiedRoute(model, protocol string) (ModelRoute, bool) {
	model = strings.TrimSpace(model)
	if model == "" || len(model) > 512 || !utf8.ValidString(model) || strings.ContainsAny(model, "\r\n\x00") {
		return ModelRoute{}, false
	}
	var kind Kind
	switch protocol {
	case "images", "chat-image":
		kind = KindImage
	case "speech":
		kind = KindAudio
	case "videos":
		kind = KindVideo
	default:
		return ModelRoute{}, false
	}
	id := "unified." + protocol + "." + base64.RawURLEncoding.EncodeToString([]byte(model))
	return ModelRoute{ID: id, LegacyModelID: id, FamilyID: "unified-" + string(kind), VersionID: id,
		Label: "统一接口", Kind: kind, Provider: ProviderUnified, Model: model, Adapter: "unified." + protocol,
		Status: RouteStatusAvailable, AuthKeys: []string{ProviderUnified}, Async: kind == KindVideo,
		SupportsReferenceURLs: protocol == "chat-image", Params: []ParamSpec{}}, true
}

func findUnifiedRoute(id string) (ModelRoute, bool) {
	parts := strings.SplitN(id, ".", 3)
	if len(parts) != 3 || parts[0] != "unified" || len(parts[2]) > 700 {
		return ModelRoute{}, false
	}
	model, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ModelRoute{}, false
	}
	route, ok := UnifiedRoute(string(model), parts[1])
	return route, ok && route.ID == id
}

func findDiscoveredRoute(id string) (ModelRoute, bool) {
	if id == CodexImageRoute().ID {
		return CodexImageRoute(), true
	}
	return findUnifiedRoute(id)
}

// AppendUnifiedRoute adds every linked record required by workbench selectors.
func AppendUnifiedRoute(catalog *ModelCatalog, route ModelRoute) {
	for _, existing := range catalog.Routes {
		if existing.ID == route.ID {
			return
		}
	}
	found := false
	for _, family := range catalog.Families {
		if family.ID == route.FamilyID {
			found = true
			break
		}
	}
	if !found {
		catalog.Families = append(catalog.Families, ModelFamily{ID: route.FamilyID, Label: route.Label, Kind: route.Kind})
	}
	modelLabel := route.Model
	if route.Provider == ProviderCodexImage {
		modelLabel = route.Label
	}
	catalog.Versions = append(catalog.Versions, ModelVersion{ID: route.VersionID, FamilyID: route.FamilyID,
		Label: modelLabel, CanonicalModel: route.Model, Kind: route.Kind,
		Capabilities: Capabilities{Async: route.Async, SupportsReferenceURLs: route.SupportsReferenceURLs}})
	catalog.Routes = append(catalog.Routes, route)
	catalog.Models = append(catalog.Models, ModelSpec{ID: route.ID, Label: route.Model, Kind: route.Kind,
		Provider: route.Provider, Model: route.Model, Adapter: route.Adapter, Async: route.Async,
		SupportsReferenceURLs: route.SupportsReferenceURLs, Params: route.Params})
}
