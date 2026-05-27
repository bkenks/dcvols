package compose

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// File is the parsed view of a Docker Compose file, narrowed to the single
// field dcvols needs: the services map. Any other top-level keys present in the
// YAML are silently ignored by the decoder.
type File struct {
	// Services maps each service name to its definition. dcvols only ever reads
	// the volume list of each service.
	Services map[string]Service `yaml:"services"`
}

// Service is the parsed view of one Compose service, narrowed to just its
// volume declarations.
type Service struct {
	// Volumes is the service's volumes list, which may freely mix the short and
	// long forms described on VolumeEntry.
	Volumes []VolumeEntry `yaml:"volumes"`
}

// VolumeEntry represents a single entry in a service's volumes list. Compose
// permits two syntaxes for these entries and VolumeEntry captures both.
//
// The short form is a single string "HOST:CONTAINER[:OPTS]", for example
// "./data:/var/lib/data:ro". The entire string is stored verbatim in raw.
//
// The long form is a YAML mapping with type/source/target keys, for example:
//
//   - type: bind
//     source: ./data
//     target: /var/lib/data
//
// Its fields are decoded into Type/Source/Target.
//
// Exactly one of (raw) or (Type/Source/Target) is populated for a given entry,
// depending on which form the YAML used; UnmarshalYAML decides based on the
// node kind. raw is unexported because it is an internal parsing detail; callers
// obtain the host path through hostPath instead.
type VolumeEntry struct {
	raw    string // populated for short-form (scalar) entries
	Type   string `yaml:"type"`   // long form: "bind", "volume", "tmpfs", ...
	Source string `yaml:"source"` // long form: host path (only for bind mounts)
	Target string `yaml:"target"` // long form: container path
}

// UnmarshalYAML implements yaml.Unmarshaler so a VolumeEntry can be decoded from
// either Compose syntax. A scalar node is the short form and is stored verbatim
// in raw; any other node (a mapping) is the long form and is decoded field by
// field.
//
// The local alias type is what prevents infinite recursion: decoding into
// *alias uses yaml.v3's default struct decoding rather than calling this method
// again, because alias does not inherit VolumeEntry's UnmarshalYAML.
func (v *VolumeEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		v.raw = value.Value
		return nil
	}
	type alias VolumeEntry
	return value.Decode((*alias)(v))
}

// Parse decodes Compose YAML into a File. Only the services/volumes subset is
// populated; all other content is ignored. The returned error wraps the
// underlying YAML decode error with a "parsing yaml" prefix.
func Parse(data []byte) (File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parsing yaml: %w", err)
	}
	return f, nil
}
