package internal

// TODO is this internal or nothing or another folder called malaina

import "embed"

//go:embed template.html
var TemplateFS embed.FS
var RelatedTypes = []string{
	"SEQUEL", "PREQUEL",
	"PARENT", "SIDE_STORY",
	"ALTERNATIVE", "SPIN_OFF",
	"SUMMARY",
}
