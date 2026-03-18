package skills

// SkillScope captures the logical source class for a loaded skill.
type SkillScope string

const (
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeUser   SkillScope = "user"
	SkillScopeSystem SkillScope = "system"
	SkillScopeCustom SkillScope = "custom"
)

// LoadOrigin describes where a skill came from during discovery.
type LoadOrigin struct {
	Path   string
	Scope  SkillScope
	Origin string
}

// SkillLoadOutcome is the structured result of skill discovery.
type SkillLoadOutcome struct {
	Registrations []SkillRegistration
	Errors        []error
	Origins       map[string]LoadOrigin
}

const (
	MetadataKeySkillPath   = "skill.path"
	MetadataKeySkillScope  = "skill.scope"
	MetadataKeySkillOrigin = "skill.origin"
	MetadataKeySkillID     = "skill.id"
)
