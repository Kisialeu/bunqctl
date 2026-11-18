// Package docs embeds the user documentation so that "bunqctl docs" shows the
// guides that match the installed binary, offline.
package docs

import "embed"

// FS holds the plain-text guides of this directory, one file per topic.
//
//go:embed *.txt
var FS embed.FS
