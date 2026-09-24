package llm

type Provider interface {
	Name() string
	Complete(prompt string) (string, error)
}
