package commands

// Flags holds global flags shared across all commands.
type Flags struct {
	LogLevel   string
	NoColor    bool
	ConfigFile string
	Repo       string
	Remote     string
	Branch     string
	Format     string
}

// Output formats.
const (
	FormatURL  = "url"
	FormatMD   = "md"
	FormatJSON = "json"
)
