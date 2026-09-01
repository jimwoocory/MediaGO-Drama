from pathlib import Path
p = Path(r"D:\openai\MediaGo-Drama\apps\workspace\src\domains\episode\lib\production-profiles.ts")
t = p.read_text(encoding="utf-8")
t = t.replace(
"const normalizeProductionProfile = (
\tprofile: ProductionProfileDefinition,
): ProductionProfileDefinition => {",
"function normalizeProductionProfile(
\tprofile: ProductionProfileDefinition,
): ProductionProfileDefinition {",
)
t = t.replace(
"const normalizeProfileID = (value: string) => value.trim().toLowerCase();",
"function normalizeProfileID(value: string) { return value.trim().toLowerCase(); }",
)
t = t.replace(
"const finiteNonNegative = (value?: number) =>\n\ttypeof value === \"number\" && Number.isFinite(value) && value >= 0 ? value : undefined;\n",
"function finiteNonNegative(value?: number) {\n\treturn typeof value === \"number\" && Number.isFinite(value) && value >= 0 ? value : undefined;\n}\n",
)
p.write_text(t, encoding="utf-8")
print("helpers hoisted")
