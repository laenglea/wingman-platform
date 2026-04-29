package claude

type Config struct {
	command string
	model   string
}

type Option func(*Config)

func WithCommand(command string) Option {
	return func(c *Config) {
		c.command = command
	}
}
