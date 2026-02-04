package internal

import "embed"

//go:embed template.html
var TemplateFS embed.FS
var RelatedTypes = []string{
	"SEQUEL", "PREQUEL",
	"PARENT", "SIDE_STORY",
	"ALTERNATIVE", "SPIN_OFF",
	"SUMMARY",
}
