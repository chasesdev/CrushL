package reasoning

import (
	"regexp"
	"strings"
)

// EngineeringAbbreviations maps common engineering shorthand to full terms
var EngineeringAbbreviations = map[string]string{
	// Common command abbreviations
	"impl":   "implement",
	"rm":     "remove",
	"mv":     "move",
	"cp":     "copy",
	"mkdir":  "make directory",
	"chmod":  "change mode",
	"chown":  "change owner",

	// Authentication & Security
	"authn":  "authentication",
	"authz":  "authorization",
	"oauth":  "oauth",
	"jwt":    "json web token",
	"saml":   "saml",
	"sso":    "single sign on",
	"mfa":    "multi factor authentication",
	"2fa":    "two factor authentication",

	// Data & Database
	"db":     "database",
	"rdb":    "relational database",
	"nosql":  "nosql database",
	"sql":    "sql",
	"orm":    "object relational mapper",
	"crud":   "create read update delete",

	// API & Web
	"api":    "api",
	"rest":   "rest api",
	"restful": "rest",
	"graphql": "graphql",
	"grpc":   "grpc",
	"rpc":    "remote procedure call",
	"http":   "http",
	"https":  "https",
	"url":    "url",
	"uri":    "uri",
	"json":   "json",
	"xml":    "xml",
	"yaml":   "yaml",

	// Infrastructure & DevOps
	"k8s":    "kubernetes",
	"cicd":   "continuous integration and deployment",
	"ci":     "continuous integration",
	"cd":     "continuous deployment",
	"cdn":    "content delivery network",
	"dns":    "dns",
	"ssl":    "ssl",
	"tls":    "tls",
	"vpc":    "virtual private cloud",
	"vm":     "virtual machine",
	"docker": "docker",
	"infra":  "infrastructure",

	// Development & Project Management
	"pr":     "pull request",
	"mr":     "merge request",
	"wip":    "work in progress",
	"tbd":    "to be determined",
	"tba":    "to be announced",
	"poc":    "proof of concept",
	"mvp":    "minimum viable product",
	"tech debt": "technical debt",
	"refactor": "refactor",

	// Frontend
	"ui":     "user interface",
	"ux":     "user experience",
	"css":    "css",
	"html":   "html",
	"js":     "javascript",
	"ts":     "typescript",
	"spa":    "single page application",
	"ssr":    "server side rendering",

	// Backend
	"bg":     "background",
	"bg job": "background job",
	"cron":   "cron job",
	"queue":  "queue",
	"cache":  "cache",
	"redis":  "redis",
	"memcached": "memcached",

	// Testing
	"qa":     "quality assurance",
	"e2e":    "end to end",
	"tdd":    "test driven development",
	"bdd":    "behavior driven development",
	"ut":     "unit test",
	"it":     "integration test",

	// Version Control
	"repo":   "repository",
	"repos":  "repositories",
	"vc":     "version control",
	"vcs":    "version control system",
	"scm":    "source control management",

	// Performance & Monitoring
	"perf":   "performance",
	"mon":    "monitoring",
	"telemetry": "telemetry",
	"metrics": "metrics",
	"logs":   "logs",
	"apm":    "application performance monitoring",

	// Miscellaneous
	"cfg":    "configuration",
	"config": "configuration",
	"env":    "environment",
	"deps":   "dependencies",
	"dep":    "dependency",
	"lib":    "library",
	"libs":   "libraries",
	"pkg":    "package",
	"mod":    "module",
	"docs":   "documentation",
}

// EngineeringVerbs are action verbs commonly used in engineering tasks
var EngineeringVerbs = []string{
	// Core actions
	"add", "create", "implement", "build", "make", "generate",
	"remove", "delete", "drop", "purge", "clean",
	"update", "modify", "change", "edit", "alter", "adjust",
	"fix", "resolve", "patch", "repair", "correct",
	"refactor", "restructure", "reorganize", "simplify",

	// Testing & Verification
	"test", "verify", "validate", "check", "ensure", "confirm",
	"debug", "troubleshoot", "diagnose",

	// Documentation
	"document", "explain", "describe", "clarify", "comment",

	// Analysis
	"analyze", "research", "investigate", "explore", "examine", "review",
	"audit", "assess", "evaluate",

	// Data operations
	"fetch", "get", "retrieve", "load", "pull", "grab",
	"set", "put", "store", "save", "persist", "push",
	"sync", "synchronize", "replicate", "backup", "restore",

	// Deployment
	"deploy", "release", "publish", "ship", "rollout",
	"install", "setup", "configure", "provision",
	"migrate", "upgrade", "downgrade", "rollback",

	// Version control
	"commit", "merge", "rebase", "cherry-pick", "squash",
	"branch", "tag", "stash", "clone", "fork",

	// Code organization
	"move", "rename", "copy", "extract", "split", "combine", "merge",
	"import", "export", "include", "exclude",

	// Performance
	"optimize", "improve", "enhance", "boost", "speed up",
	"cache", "memoize", "lazy load", "preload",

	// Security
	"secure", "protect", "encrypt", "decrypt", "hash", "sign", "verify",
	"sanitize", "validate", "escape", "authenticate", "authorize",

	// Monitoring
	"monitor", "track", "log", "trace", "profile", "measure",

	// UI/UX
	"design", "style", "layout", "render", "display", "show", "hide",
	"enable", "disable", "toggle", "switch", "activate", "deactivate",

	// Integration
	"integrate", "connect", "link", "attach", "bind", "hook",
	"disconnect", "unlink", "detach", "unbind", "unhook",

	// Execution
	"run", "execute", "invoke", "call", "trigger", "fire",
	"start", "stop", "pause", "resume", "restart", "reload",
}

// JargonExpander handles expansion of engineering jargon and abbreviations
type JargonExpander struct {
	abbrevMap map[string]string
}

// NewJargonExpander creates a new jargon expander
func NewJargonExpander() *JargonExpander {
	return &JargonExpander{
		abbrevMap: EngineeringAbbreviations,
	}
}

// Expand expands engineering abbreviations in text
func (j *JargonExpander) Expand(text string) string {
	expanded := text

	// Expand abbreviations (case-insensitive)
	for abbrev, full := range j.abbrevMap {
		// Match whole words only (with word boundaries)
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(abbrev) + `\b`)
		expanded = pattern.ReplaceAllStringFunc(expanded, func(match string) string {
			// Preserve original casing style
			if isAllCaps(match) {
				return strings.ToUpper(full)
			} else if isCapitalized(match) {
				return capitalize(full)
			}
			return full
		})
	}

	return expanded
}

// isAllCaps checks if a string is all uppercase
func isAllCaps(s string) bool {
	return s == strings.ToUpper(s) && s != strings.ToLower(s)
}

// isCapitalized checks if a string starts with capital letter
func isCapitalized(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := string(s[0])
	return first == strings.ToUpper(first) && first != strings.ToLower(first)
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(string(s[0])) + s[1:]
}

// IsEngineeringVerb checks if a word is a common engineering action verb
func IsEngineeringVerb(word string) bool {
	lower := strings.ToLower(word)
	for _, verb := range EngineeringVerbs {
		if lower == verb {
			return true
		}
	}
	return false
}
