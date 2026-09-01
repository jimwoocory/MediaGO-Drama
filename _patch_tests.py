from pathlib import Path
p = Path(r"D:\openai\MediaGo-Drama\services\server\internal\service\productionprofile\registry_test.go")
text = p.read_text(encoding="utf-8")
start = text.find("func TestBuiltinProductionProfileManifestStartsEmptyUntilDefinitionsAreVerified")
if start < 0:
    raise SystemExit("start not found")
end = text.find("func TestProductionProfileRegistryResolvesPlanningDirective", start)
if end < 0:
    raise SystemExit("end not found")
new = '''func TestBuiltinProductionProfileManifestIncludesSixModes(t *testing.T) {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry() error = %v", err)
	}
	profiles := registry.List()
	if got := len(profiles); got != 6 {
		t.Fatalf("builtin profile count = %d, want 6", got)
	}
	if _, ok := registry.Get("animation"); !ok {
		t.Fatal("missing animation profile")
	}
}

'''
p.write_text(text[:start] + new + text[end:], encoding="utf-8")
print("patched", p)

p2 = Path(r"D:\openai\MediaGo-Drama\apps\workspace\src\domains\episode\lib\production-profile-manifest.test.ts")
t2 = p2.read_text(encoding="utf-8")
if "restored six built-in" not in t2:
    t2 = t2.replace(
        "from \"@/domains/episode/lib/production-profile-manifest\";",
        "from \"@/domains/episode/lib/production-profile-manifest\";\nimport { builtinProductionProfileManifest } from \"@/domains/episode/lib/production-profile-manifest\";",
    )
    extra = '''
	it("exposes the restored six built-in production modes", () => {
		const profiles = loadProductionProfileManifest(builtinProductionProfileManifest);
		expect(profiles.map((profile) => profile.id)).toEqual([
			"animation",
			"live-action",
			"comic-drama",
			"short-drama",
			"cinematic",
			"explainer",
		]);
		expect(profiles.find((profile) => profile.id === "animation")?.label).toBe("动画");
	});
'''
    t2 = t2.replace("\tdescribe(\"production profile manifest\", () => {", "describe(\"production profile manifest\", () => {")
    t2 = t2.replace("\t});\n", extra + "\t});\n", 1) if False else t2
    # insert before last closing of describe
    idx = t2.rfind("});")
    t2 = t2[:idx] + extra + t2[idx:]
    p2.write_text(t2, encoding="utf-8")
    print("patched", p2)
