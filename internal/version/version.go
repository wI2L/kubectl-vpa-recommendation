package version

import (
	"fmt"
	"runtime"
)

var (
	gitVersion      string // semantic version, derived by build scripts
	gitVersionMajor string // major version, always numeric
	gitVersionMinor string // minor version, always numeric
	gitCommit       string // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState    string // state of git tree, either "clean" or "dirty"
	buildDate       string // build date in rfc3339 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
)

// Info represents the version information.
type Info struct {
	Major        string `json:"major,omitempty"`
	Minor        string `json:"minor,omitempty"`
	GitVersion   string `json:"gitVersion,omitempty"`
	GitCommit    string `json:"gitCommit,omitempty"`
	GitTreeState string `json:"gitTreeState,omitempty"`
	BuildDate    string `json:"buildDate,omitempty"`
	GoVersion    string `json:"goVersion,omitempty"`
	Compiler     string `json:"compiler,omitempty"`
	Platform     string `json:"platform,omitempty"`
}

// Get returns metadata and information about the version.
func Get() Info {
	return Info{
		Major:        gitVersionMajor,
		Minor:        gitVersionMinor,
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String implements the fmt.Stringer interface.
// It returns the version as a human-friendly string.
func (info Info) String() string {
	return info.GitVersion
}

// Raw returns the raw representation of the version information.
// Used for debugging purpose or as the output of the version command.
func (info Info) Raw() string {
	return fmt.Sprintf("%#v", info)
}
