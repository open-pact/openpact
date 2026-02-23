package chat

// AllowlistConfig is the common security config shared by all providers.
type AllowlistConfig struct {
	AllowedUsers []string `yaml:"allowed_users"`
	AllowedChans []string `yaml:"allowed_chans"`
}
