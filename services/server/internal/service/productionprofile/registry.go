package productionprofile

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const ManifestSchemaVersion = 1

//go:embed profiles.v1.json
var builtinManifestJSON []byte

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Profiles      []Profile `json:"profiles"`
}

type Profile struct {
	ID                           string                  `json:"id"`
	Label                        string                  `json:"label"`
	Description                  string                  `json:"description"`
	Version                      int                     `json:"version"`
	PreferredShotDurationSeconds *ShotDurationPreference `json:"preferredShotDurationSeconds,omitempty"`
	Planning                     *PlanningPolicy         `json:"planning,omitempty"`
	Metadata                     map[string]any          `json:"metadata,omitempty"`
}

type ShotDurationPreference struct {
	Min    *float64 `json:"min,omitempty"`
	Target *float64 `json:"target,omitempty"`
	Max    *float64 `json:"max,omitempty"`
}

type PlanningPolicy struct {
	TargetShotsPerMinute *float64 `json:"targetShotsPerMinute,omitempty"`
	PacingIntensity      *float64 `json:"pacingIntensity,omitempty"`
	DialogueWeight       *float64 `json:"dialogueWeight,omitempty"`
	VoiceoverWeight      *float64 `json:"voiceoverWeight,omitempty"`
	ContinuityStrength   *float64 `json:"continuityStrength,omitempty"`
	AssetReuseBias       *float64 `json:"assetReuseBias,omitempty"`
}

type ShotCountGuidance struct {
	Min    *int `json:"min,omitempty"`
	Target *int `json:"target,omitempty"`
	Max    *int `json:"max,omitempty"`
}

type Directive struct {
	ProfileID                    string                  `json:"profileId"`
	ProfileLabel                 string                  `json:"profileLabel"`
	ProfileVersion               int                     `json:"profileVersion"`
	TargetDurationSeconds        *float64                `json:"targetDurationSeconds,omitempty"`
	PreferredShotDurationSeconds *ShotDurationPreference `json:"preferredShotDurationSeconds,omitempty"`
	ShotCount                    *ShotCountGuidance      `json:"shotCount,omitempty"`
	Policy                       PlanningPolicy          `json:"policy"`
	Prompt                       string                  `json:"prompt"`
}

type Registry struct {
	profiles []Profile
	byID     map[string]Profile
}

func NewBuiltinRegistry() (*Registry, error) {
	return NewRegistryFromJSON(builtinManifestJSON)
}

func MustBuiltinRegistry() *Registry {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		panic(err)
	}
	return registry
}

func NewRegistryFromJSON(data []byte) (*Registry, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decoding production profile manifest: %w", err)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return nil, fmt.Errorf("unsupported production profile manifest schema: %d", manifest.SchemaVersion)
	}
	registry := &Registry{byID: map[string]Profile{}}
	for _, raw := range manifest.Profiles {
		profile := normalizeProfile(raw)
		if err := validateProfile(profile); err != nil {
			return nil, err
		}
		if _, exists := registry.byID[profile.ID]; exists {
			return nil, fmt.Errorf("duplicate production profile id: %s", profile.ID)
		}
		registry.byID[profile.ID] = profile
		registry.profiles = append(registry.profiles, profile)
	}
	sort.SliceStable(registry.profiles, func(i, j int) bool { return registry.profiles[i].ID < registry.profiles[j].ID })
	return registry, nil
}

func (registry *Registry) List() []Profile {
	if registry == nil || len(registry.profiles) == 0 {
		return []Profile{}
	}
	result := make([]Profile, len(registry.profiles))
	copy(result, registry.profiles)
	return result
}

func (registry *Registry) Get(id string) (Profile, bool) {
	if registry == nil {
		return Profile{}, false
	}
	profile, ok := registry.byID[normalizeID(id)]
	return profile, ok
}

func (registry *Registry) Resolve(id string, targetDurationSeconds float64) (Directive, bool) {
	profile, ok := registry.Get(id)
	if !ok {
		return Directive{}, false
	}
	return ResolveDirective(profile, targetDurationSeconds), true
}

func ResolveDirective(profile Profile, targetDurationSeconds float64) Directive {
	target := positivePointer(targetDurationSeconds)
	preferred := compactPreference(profile.PreferredShotDurationSeconds)
	policy := compactPolicy(profile.Planning)
	var shotCount *ShotCountGuidance
	if target != nil {
		shotCount = deriveShotCountGuidance(*target, preferred, policy)
	}
	directive := Directive{
		ProfileID:                    profile.ID,
		ProfileLabel:                 profile.Label,
		ProfileVersion:               profile.Version,
		TargetDurationSeconds:        target,
		PreferredShotDurationSeconds: preferred,
		ShotCount:                    shotCount,
		Policy:                       policy,
	}
	directive.Prompt = BuildPrompt(directive)
	return directive
}

func BuildPrompt(directive Directive) string {
	lines := []string{
		"# Production Planning Context",
		fmt.Sprintf("制作模式：%s (%s) · v%d", directive.ProfileLabel, directive.ProfileID, directive.ProfileVersion),
		"本上下文只用于规划镜头，不得把 Provider 单次生成时长限制写回 ProductionShot。",
	}
	if directive.TargetDurationSeconds != nil {
		lines = append(lines, fmt.Sprintf("项目目标时长：%s 秒；这是编辑目标，不是硬上限。", formatNumber(*directive.TargetDurationSeconds)))
	}
	if preferred := directive.PreferredShotDurationSeconds; preferred != nil {
		parts := []string{}
		if preferred.Min != nil {
			parts = append(parts, fmt.Sprintf("最短偏好 %s 秒", formatNumber(*preferred.Min)))
		}
		if preferred.Target != nil {
			parts = append(parts, fmt.Sprintf("目标 %s 秒", formatNumber(*preferred.Target)))
		}
		if preferred.Max != nil {
			parts = append(parts, fmt.Sprintf("最长偏好 %s 秒", formatNumber(*preferred.Max)))
		}
		if len(parts) > 0 {
			lines = append(lines, "镜头自然时长偏好："+strings.Join(parts, "，")+"；这是偏好，不是镜头合法性限制。")
		}
	}
	if count := directive.ShotCount; count != nil {
		parts := []string{}
		if count.Min != nil {
			parts = append(parts, fmt.Sprintf("至少约 %d", *count.Min))
		}
		if count.Target != nil {
			parts = append(parts, fmt.Sprintf("建议约 %d", *count.Target))
		}
		if count.Max != nil {
			parts = append(parts, fmt.Sprintf("至多约 %d", *count.Max))
		}
		if len(parts) > 0 {
			lines = append(lines, "镜头数量指导："+strings.Join(parts, "，")+" 个；根据剧情完整性允许偏离。")
		}
	}
	appendNumberLine(&lines, "镜头密度", directive.Policy.TargetShotsPerMinute, " 镜头/分钟")
	appendWeightLine(&lines, "节奏强度", directive.Policy.PacingIntensity, "0 偏舒缓，1 偏快速")
	appendWeightLine(&lines, "对白权重", directive.Policy.DialogueWeight, "越高越优先用角色对白承载信息")
	appendWeightLine(&lines, "旁白权重", directive.Policy.VoiceoverWeight, "越高越允许旁白承载信息")
	appendWeightLine(&lines, "连续性强度", directive.Policy.ContinuityStrength, "越高越严格保持角色、场景、动作和光线状态连续")
	appendWeightLine(&lines, "资产复用倾向", directive.Policy.AssetReuseBias, "越高越优先复用已建立角色/场景/道具资产")
	return strings.Join(lines, "\n")
}

func normalizeProfile(profile Profile) Profile {
	profile.ID = normalizeID(profile.ID)
	profile.Label = strings.TrimSpace(profile.Label)
	profile.Description = strings.TrimSpace(profile.Description)
	return profile
}

func normalizeID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateProfile(profile Profile) error {
	if profile.ID == "" {
		return fmt.Errorf("production profile id is required")
	}
	if profile.Label == "" {
		return fmt.Errorf("production profile label is required: %s", profile.ID)
	}
	if profile.Version < 1 {
		return fmt.Errorf("production profile version must be a positive integer: %s", profile.ID)
	}
	if preferred := profile.PreferredShotDurationSeconds; preferred != nil {
		for name, value := range map[string]*float64{"min": preferred.Min, "target": preferred.Target, "max": preferred.Max} {
			if value != nil && !isPositive(*value) {
				return fmt.Errorf("invalid production profile %s: %s must be > 0", profile.ID, name)
			}
		}
		if preferred.Min != nil && preferred.Max != nil && *preferred.Min > *preferred.Max {
			return fmt.Errorf("invalid production profile %s: preferred shot duration min must be <= max", profile.ID)
		}
		if preferred.Target != nil && preferred.Min != nil && *preferred.Target < *preferred.Min {
			return fmt.Errorf("invalid production profile %s: preferred shot duration target must be >= min", profile.ID)
		}
		if preferred.Target != nil && preferred.Max != nil && *preferred.Target > *preferred.Max {
			return fmt.Errorf("invalid production profile %s: preferred shot duration target must be <= max", profile.ID)
		}
	}
	if policy := profile.Planning; policy != nil {
		if policy.TargetShotsPerMinute != nil && !isPositive(*policy.TargetShotsPerMinute) {
			return fmt.Errorf("invalid production profile %s: targetShotsPerMinute must be > 0", profile.ID)
		}
		weights := map[string]*float64{
			"pacingIntensity":    policy.PacingIntensity,
			"dialogueWeight":     policy.DialogueWeight,
			"voiceoverWeight":    policy.VoiceoverWeight,
			"continuityStrength": policy.ContinuityStrength,
			"assetReuseBias":     policy.AssetReuseBias,
		}
		for name, value := range weights {
			if value != nil && !isUnitInterval(*value) {
				return fmt.Errorf("invalid production profile %s: %s must be between 0 and 1", profile.ID, name)
			}
		}
	}
	return nil
}

func deriveShotCountGuidance(targetDurationSeconds float64, preferred *ShotDurationPreference, policy PlanningPolicy) *ShotCountGuidance {
	if !isPositive(targetDurationSeconds) {
		return nil
	}
	var min, target, max *int
	if preferred != nil && preferred.Max != nil {
		value := maxInt(1, int(math.Ceil(targetDurationSeconds / *preferred.Max)))
		min = &value
	}
	if preferred != nil && preferred.Min != nil {
		value := maxInt(1, int(math.Floor(targetDurationSeconds / *preferred.Min)))
		max = &value
	}
	if policy.TargetShotsPerMinute != nil {
		value := maxInt(1, int(math.Round((targetDurationSeconds/60)**policy.TargetShotsPerMinute)))
		target = &value
	} else if preferred != nil && preferred.Target != nil {
		value := maxInt(1, int(math.Round(targetDurationSeconds / *preferred.Target)))
		target = &value
	}
	if target != nil && min != nil && *target < *min {
		*target = *min
	}
	if target != nil && max != nil && *target > *max {
		*target = *max
	}
	if min == nil && target == nil && max == nil {
		return nil
	}
	return &ShotCountGuidance{Min: min, Target: target, Max: max}
}

func compactPreference(value *ShotDurationPreference) *ShotDurationPreference {
	if value == nil {
		return nil
	}
	result := &ShotDurationPreference{
		Min:    positiveClone(value.Min),
		Target: positiveClone(value.Target),
		Max:    positiveClone(value.Max),
	}
	if result.Min == nil && result.Target == nil && result.Max == nil {
		return nil
	}
	return result
}

func compactPolicy(value *PlanningPolicy) PlanningPolicy {
	if value == nil {
		return PlanningPolicy{}
	}
	return PlanningPolicy{
		TargetShotsPerMinute: positiveClone(value.TargetShotsPerMinute),
		PacingIntensity:      unitIntervalClone(value.PacingIntensity),
		DialogueWeight:       unitIntervalClone(value.DialogueWeight),
		VoiceoverWeight:      unitIntervalClone(value.VoiceoverWeight),
		ContinuityStrength:   unitIntervalClone(value.ContinuityStrength),
		AssetReuseBias:       unitIntervalClone(value.AssetReuseBias),
	}
}

func positivePointer(value float64) *float64 {
	if !isPositive(value) {
		return nil
	}
	result := value
	return &result
}

func positiveClone(value *float64) *float64 {
	if value == nil || !isPositive(*value) {
		return nil
	}
	result := *value
	return &result
}

func unitIntervalClone(value *float64) *float64 {
	if value == nil || !isUnitInterval(*value) {
		return nil
	}
	result := *value
	return &result
}

func isPositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func isUnitInterval(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func appendNumberLine(lines *[]string, label string, value *float64, suffix string) {
	if value == nil {
		return
	}
	*lines = append(*lines, fmt.Sprintf("%s：%s%s。", label, formatNumber(*value), suffix))
}

func appendWeightLine(lines *[]string, label string, value *float64, explanation string) {
	if value == nil {
		return
	}
	*lines = append(*lines, fmt.Sprintf("%s：%s（%s）。", label, formatNumber(*value), explanation))
}

func formatNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		return fmt.Sprintf("%.0f", value)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
