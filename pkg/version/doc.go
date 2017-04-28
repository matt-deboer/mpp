// Package version contains build-time version info
package version // import "github.com/matt-deboer/mpp/pkg/version"

var (
	// Name is set at compile time based on the git repository
	Name string
	// Version is set at compile time with the git version
	Version string
	// Branch is set at compile time with the git version
	Branch string
	// Revision is set at compile time with the git version
	Revision string
)
