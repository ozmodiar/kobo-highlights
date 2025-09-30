package formats

// Domain structs shared by all formats.

type Highlight struct {
	Text string
	Date string // raw date string from DB (kept as-is for now)
}

type Book struct {
	Title      string
	Author     string
	Highlights []Highlight
}

// Format defines a pluggable output format target.
type Format interface {
	Export(books []Book) error
	Name() string
}

// FormatFactory holds metadata + builder for a format implementation.
type FormatFactory struct {
	Name  string
	Flags []FlagProvider // deferred flag providers to keep registry decoupled from cli framework
	Build func(resolver FlagValueResolver) (Format, error)
}

// FlagProvider returns a flag definition (kept intentionally untyped as 'any').
type FlagProvider interface{ CLIFlag() any }

// FlagValueResolver abstracts fetching CLI flag values (allows easier testing).
type FlagValueResolver interface{ String(name string) string }

var formatRegistry = map[string]*FormatFactory{}

// RegisterFormat adds a format factory to the registry (last one wins on name collision).
func RegisterFormat(f *FormatFactory) { formatRegistry[f.Name] = f }

// GetFormatFactory returns a factory by name.
func GetFormatFactory(name string) (*FormatFactory, bool) {
	f, ok := formatRegistry[name]
	return f, ok
}

// ListFormatNames returns registered format names.
func ListFormatNames() []string {
	names := make([]string, 0, len(formatRegistry))
	for n := range formatRegistry {
		names = append(names, n)
	}
	return names
}
