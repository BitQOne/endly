package endly

import (
	"github.com/viant/toolbox"
	"math"
	"strings"
)

//OperatingSystem represents an OperatingSystem
type OperatingSystem struct {
	System       string
	Name         string
	Hardware     string
	Architecture string
	Version      string
	Path         *SystemPath
}

func normalizeVersion(version string, count int) int {
	var result = 0
	var fragments = strings.Split(version, ".")
	for i, fragment := range fragments {
		factor := math.Pow(10.0, (2.0 * float64(count-i)))
		result += toolbox.AsInt(fragment) * int(factor)
	}
	return result
}

//Matches returns true if operating system matches provided target
func (s *OperatingSystem) Matches(target *OperatingSystemTarget) bool {
	if target == nil {
		return true
	}
	if target.Name != "" && target.Name != s.Name {
		return false
	}
	if target.System != "" && target.System != s.System {
		return false
	}

	if target.MinRequiredVersion == "" && target.MaxAllowedVersion == "" {
		return true
	}
	var versionFragmentCount = strings.Count(s.Version, ".")
	var actualVersion = normalizeVersion(s.Version, versionFragmentCount)

	if target.MinRequiredVersion != "" {
		var minRequiredVersion = normalizeVersion(target.MinRequiredVersion, versionFragmentCount)
		if actualVersion < minRequiredVersion {
			return false
		}
	}
	var maxAllowedVersion = normalizeVersion(target.MaxAllowedVersion, versionFragmentCount)
	return actualVersion >= maxAllowedVersion
}

//SystemPath represents a system path
type SystemPath struct {
	index      map[string]bool
	SystemPath []string
	Path       []string
}

//Push appends path to the system paths
func (p *SystemPath) Push(paths ...string) {
	for _, path := range paths {
		if strings.Contains(path, "\n") {
			continue
		}
		if _, has := p.index[path]; has {
			return
		}
		p.Path = append(p.Path, path)
		p.index[path] = true
	}
}

//EnvValue returns evn values
func (p *SystemPath) EnvValue() string {
	var directories = append(p.Path, p.SystemPath...)
	return strings.Join(directories, ":")
}

//OperatingSystemTarget represents operating system target
type OperatingSystemTarget struct {
	System             string
	Name               string
	MinRequiredVersion string
	MaxAllowedVersion  string
}
