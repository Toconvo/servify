package version

// These variables can be set via -ldflags during build.
// Defaults are for local development.
var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)
