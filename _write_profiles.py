from pathlib import Path
import json

profiles = [
  {
    "id": "animation",
    "label": "动画",
    "description": "二维/三维动画剧集。强调角色造型稳定、动作衔接和资产复用，镜头可偏完整动作段落。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 3, "target": 6, "max": 12},
    "planning": {
      "targetShotsPerMinute": 10,
      "pacingIntensity": 0.55,
      "dialogueWeight": 0.55,
      "voiceoverWeight": 0.25,
      "continuityStrength": 0.95,
      "assetReuseBias": 0.95,
    },
    "metadata": {"family": "animation"},
  },
  {
    "id": "live-action",
    "label": "真人实拍",
    "description": "真人短剧/剧集。对白驱动，保持场面调度、表演状态和光线连续。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 2, "target": 5, "max": 10},
    "planning": {
      "targetShotsPerMinute": 12,
      "pacingIntensity": 0.65,
      "dialogueWeight": 0.85,
      "voiceoverWeight": 0.15,
      "continuityStrength": 0.9,
      "assetReuseBias": 0.7,
    },
    "metadata": {"family": "live-action"},
  },
  {
    "id": "comic-drama",
    "label": "漫剧",
    "description": "漫画分格转动态。镜头偏构图明确、信息密度高，适合对白气泡与定格转场。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 2, "target": 4, "max": 8},
    "planning": {
      "targetShotsPerMinute": 14,
      "pacingIntensity": 0.7,
      "dialogueWeight": 0.7,
      "voiceoverWeight": 0.2,
      "continuityStrength": 0.85,
      "assetReuseBias": 0.9,
    },
    "metadata": {"family": "comic"},
  },
  {
    "id": "short-drama",
    "label": "短剧",
    "description": "竖屏/横屏短剧节奏。镜头更密，冲突推进快，对白承担主要信息。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 2, "target": 4, "max": 8},
    "planning": {
      "targetShotsPerMinute": 16,
      "pacingIntensity": 0.85,
      "dialogueWeight": 0.8,
      "voiceoverWeight": 0.1,
      "continuityStrength": 0.8,
      "assetReuseBias": 0.75,
    },
    "metadata": {"family": "short-drama"},
  },
  {
    "id": "cinematic",
    "label": "电影感",
    "description": "长镜头与场面调度优先。节奏更缓，连续性和光影一致性要求更高。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 4, "target": 8, "max": 16},
    "planning": {
      "targetShotsPerMinute": 8,
      "pacingIntensity": 0.35,
      "dialogueWeight": 0.45,
      "voiceoverWeight": 0.2,
      "continuityStrength": 0.95,
      "assetReuseBias": 0.65,
    },
    "metadata": {"family": "cinematic"},
  },
  {
    "id": "explainer",
    "label": "解说片",
    "description": "旁白驱动的说明/纪录风格。画面服务讲解，允许更高旁白权重和资产复用。",
    "version": 1,
    "preferredShotDurationSeconds": {"min": 3, "target": 6, "max": 12},
    "planning": {
      "targetShotsPerMinute": 10,
      "pacingIntensity": 0.4,
      "dialogueWeight": 0.2,
      "voiceoverWeight": 0.9,
      "continuityStrength": 0.7,
      "assetReuseBias": 0.8,
    },
    "metadata": {"family": "explainer"},
  },
]

manifest = {"schemaVersion": 1, "profiles": profiles}
json_path = Path(r"D:\openai\MediaGo-Drama\services\server\internal\service\productionprofile\profiles.v1.json")
json_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

ts_path = Path(r"D:\openai\MediaGo-Drama\apps\workspace\src\domains\episode\lib\production-profile-manifest.ts")
ts = '''import type { ProductionProfileDefinition } from "@/domains/episode/lib/production";

export const productionProfileManifestSchemaVersion = 1 as const;

export interface ProductionProfileManifest {
\tschemaVersion: typeof productionProfileManifestSchemaVersion;
\tprofiles: ProductionProfileDefinition[];
}

export const builtinProductionProfileManifest: ProductionProfileManifest = ''' + json.dumps(manifest, ensure_ascii=False, indent=2) + ''';

export const loadProductionProfileManifest = (
\tmanifest: ProductionProfileManifest,
): ProductionProfileDefinition[] => {
\tif (manifest.schemaVersion !== productionProfileManifestSchemaVersion) {
\t\tthrow new Error(`unsupported production profile manifest schema: ${manifest.schemaVersion}`);
\t}
\treturn manifest.profiles.map((profile) => ({
\t\t...profile,
\t\tid: profile.id.trim(),
\t\tlabel: profile.label.trim(),
\t\tdescription: profile.description.trim(),
\t\tmetadata: profile.metadata ? { ...profile.metadata } : undefined,
\t}));
};
'''
ts_path.write_text(ts.replace("\\t", "\t"), encoding="utf-8")
print("wrote", json_path)
print("wrote", ts_path)
print("count", len(profiles))
