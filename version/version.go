// Package version provides build-time version and buildstamp information.
package version

// Buildstamp version number defined by the compiler.
// Set via ldflags at build time:
//
//	-ldflags "-X github.com/flowline-io/flowbot-registry/version.Buildstamp=`date -u '+%Y-%m-%dT%H:%M:%SZ'`"
var Buildstamp = "undef"

// Buildtags is set to the git tag or commit hash at build time.
// Set via ldflags at build time:
//
//	-ldflags "-X github.com/flowline-io/flowbot-registry/version.Buildtags=`git describe --tags`"
var Buildtags = "v0.92.0"
